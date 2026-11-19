package rust_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/languages/rust"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/step"
)

const (
	appManifest = "[package]\nname = \"app\"\nversion = \"0.1.0\"\nedition = \"2024\"\n"
	libSource   = "pub fn answer() -> u8 {\n    42\n}\n"
)

func selectRust(t *testing.T, options config.Options) plan.Selected {
	t.Helper()

	return bedrocktest.Select(t, "rust", options)
}

func removes(steps []step.Step, path string) bool {
	return slices.ContainsFunc(steps, func(s step.Step) bool {
		remove, ok := s.(step.Remove)

		return ok && remove.Path == path
	})
}

func hasNote(notes []string, part string) bool {
	return slices.ContainsFunc(notes, func(note string) bool { return strings.Contains(note, part) })
}

func TestAdoptPutsMembersOnTheWorkspaceLints(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":   "[workspace]\nmembers = [\"a\"]\n",
		"a/Cargo.toml": appManifest + "\n[lints.rust]\nunsafe_code = \"allow\"\n",
		"a/src/lib.rs": libSource,
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	member := bedrocktest.Written(result.Steps, "a/Cargo.toml")
	if !strings.Contains(member, "[lints]\nworkspace = true") || strings.Contains(member, "unsafe_code") {
		t.Errorf("member keeps its own lints:\n%s", member)
	}

	if !strings.Contains(bedrocktest.Written(result.Steps, "Cargo.toml"), "[workspace.lints") {
		t.Error("root manifest gains no workspace lints")
	}
}

func TestAdoptRemovesTheXtaskCrateRustBedrockWrote(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":          "[workspace]\nmembers = [\"xtask\", \"crates/*\"]\n",
		"xtask/Cargo.toml":    "[package]\nname = \"xtask\"\nedition = \"2024\"\n\n[dependencies]\nrust-bedrock = { git = \"https://github.com/xsyetopz/rust-bedrock\" }\n",
		"xtask/src/main.rs":   "fn main() {}\n",
		".cargo/config.toml":  "[alias]\nbedrock = \"run --package xtask --\"\n",
		"deny.toml":           "[sources]\nallow-git = [\"https://github.com/xsyetopz/bedrock\"]\n",
		"crates/a/Cargo.toml": appManifest,
		"crates/a/src/lib.rs": libSource,
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if !removes(result.Steps, "xtask") || !removes(result.Steps, ".cargo/config.toml") {
		t.Errorf("plan keeps the xtask crate or its alias: %v", bedrocktest.Paths(result.Steps))
	}

	if manifest := bedrocktest.Written(result.Steps, "Cargo.toml"); strings.Contains(manifest, "xtask") {
		t.Errorf("root manifest still lists xtask:\n%s", manifest)
	}

	if deny := bedrocktest.Written(result.Steps, "deny.toml"); strings.Contains(deny, "xsyetopz/bedrock") {
		t.Errorf("`deny.toml` still allows the git dependency:\n%s", deny)
	}
}

func TestAdoptKeepsAnXtaskCrateThatDoesNotDependOnRustBedrock(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":         "[workspace]\nmembers = [\"xtask\"]\n",
		"xtask/Cargo.toml":   appManifest,
		"xtask/src/main.rs":  "fn main() {}\n",
		".cargo/config.toml": "[alias]\nbedrock = \"run --package xtask --\"\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if removes(result.Steps, "xtask") || removes(result.Steps, ".cargo/config.toml") {
		t.Errorf("plan removes what is not bedrock's: %v", bedrocktest.Paths(result.Steps))
	}

	if !hasNote(result.Notes, "does not depend on `rust-bedrock`") {
		t.Errorf("no note about the xtask crate: %v", result.Notes)
	}
}

func TestAdoptRenamesModRsBesideItsDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":         appManifest,
		"src/lib.rs":         "mod foo;\n",
		"src/foo/mod.rs":     "pub fn f() {}\n",
		"src/bar/mod.rs":     "#[path = \"x.rs\"]\nmod y;\n",
		"src/bar/x.rs":       "pub fn g() {}\n",
		"src/odd/mod.rs":     "include!(concat!(env!(\"OUT_DIR\"), \"/odd.rs\"));\n",
		"src/lib_mods/.keep": "",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if !slices.ContainsFunc(result.Steps, func(s step.Step) bool {
		move, ok := s.(step.Move)

		return ok && move.From == "src/foo/mod.rs" && move.Path == "src/foo.rs"
	}) {
		t.Errorf("`src/foo/mod.rs` does not move: %v", bedrocktest.Paths(result.Steps))
	}

	if rewritten := bedrocktest.Written(
		result.Steps,
		"src/bar.rs",
	); !strings.Contains(
		rewritten,
		`#[path = "bar/x.rs"]`,
	) {
		t.Errorf("`#[path]` keeps its old meaning:\n%s", rewritten)
	}

	if !removes(result.Steps, "src/bar/mod.rs") {
		t.Error("`src/bar/mod.rs` stays beside its replacement")
	}

	if !hasNote(result.Notes, "`src/odd/mod.rs`: not renamed") {
		t.Errorf("no note about the odd `mod.rs`: %v", result.Notes)
	}
}

func TestAdoptRemovesFilesTheManagedOnesReplace(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":     appManifest,
		"src/lib.rs":     libSource,
		".rustfmt.toml":  "max_width = 80\n",
		"rust-toolchain": "stable\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if !removes(result.Steps, ".rustfmt.toml") || !removes(result.Steps, "rust-toolchain") {
		t.Errorf("replaced files stay: %v", bedrocktest.Paths(result.Steps))
	}

	if !hasNote(result.Notes, "Removed `.rustfmt.toml`, `rust-toolchain`") {
		t.Errorf("no note about the removed files: %v", result.Notes)
	}
}

func TestAdoptNotesEditionsOlderThan2024(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":      "[workspace]\nmembers = [\"old\", \"bare\"]\n",
		"old/Cargo.toml":  "[package]\nname = \"old\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
		"bare/Cargo.toml": "[package]\nname = \"bare\"\nversion = \"0.1.0\"\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if !hasNote(result.Notes, "`old/Cargo.toml`: edition 2021; after applying, `cargo fix --edition` migrates it") {
		t.Errorf("no note for the 2021 crate: %v", result.Notes)
	}

	if !hasNote(result.Notes, "`bare/Cargo.toml`: declares no edition") {
		t.Errorf("no note for the crate without an edition: %v", result.Notes)
	}

	if !strings.Contains(bedrocktest.Written(result.Steps, "bare/Cargo.toml"), `edition = "2015"`) {
		t.Error("the crate without an edition is not named 2015")
	}
}

func TestAdoptLeavesANestedWorkspaceAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":            appManifest,
		"src/lib.rs":            libSource,
		"vendor/Cargo.toml":     "[workspace]\n\n[package]\nname = \"vendored\"\nedition = \"2018\"\n",
		"vendor/src/foo/mod.rs": "pub fn f() {}\n",
		"vendor/src/lib.rs":     "mod foo;\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	for _, path := range bedrocktest.Paths(result.Steps) {
		if strings.HasPrefix(path, "vendor/") {
			t.Errorf("plan touches the nested workspace: %s", path)
		}
	}
}

func TestAdoptLeavesAManifestThatIsNotTOML(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"Cargo.toml": "[package\nname = \n"})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if bedrocktest.Written(result.Steps, "Cargo.toml") != "" {
		t.Error("plan rewrites a manifest it cannot read")
	}

	if !hasNote(result.Notes, "`Cargo.toml`: not valid TOML; left as it is.") {
		t.Errorf("no note about the manifest: %v", result.Notes)
	}
}

func TestAdoptGatesAnIntegrationTestWithCfgTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":  appManifest,
		"src/lib.rs":  libSource,
		"tests/it.rs": "//! Only docs\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if got := bedrocktest.Written(result.Steps, "tests/it.rs"); got != "//! Only docs\n#![cfg(test)]\n\n" {
		t.Errorf("gated test is %q", got)
	}
}

func TestSecondAdoptPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml":     appManifest,
		"src/lib.rs":     "mod foo;\n",
		"src/foo/mod.rs": "pub fn f() {}\n",
		"tests/it.rs":    "//! Only docs\n",
		".rustfmt.toml":  "max_width = 80\n",
	})
	bedrocktest.Adopt(t, root, false, selectRust(t, nil))
	bedrocktest.CommitAll(t, root)

	// Act
	again := bedrocktest.Plan(t, root, false, selectRust(t, nil))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestPackageNameFollowsTheDirectoryName(t *testing.T) {
	t.Parallel()

	for directory, want := range map[string]string{
		"my app":    "my-app",
		"2fast":     "app-2fast",
		"--":        "app",
		"ok_name-1": "ok_name-1",
		"MyApp":     "myapp",
	} {
		t.Run(directory, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := filepath.Join(t.TempDir(), directory)
			if err := os.Mkdir(root, 0o750); err != nil {
				t.Fatal(err)
			}

			// Act
			vars, err := rust.New().Variables(t.Context(), bedrocktest.Context(t, root, selectRust(t, nil)))
			if err != nil {
				t.Fatal(err)
			}

			// Assert
			if vars["package"] != want {
				t.Errorf("package is %q, want %q", vars["package"], want)
			}
		})
	}
}

func TestInitWritesTheCratesOfEachKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"lib", "bin", "no-std", "ffi", "wasm", "bun-ffi"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)

			// Act
			result := bedrocktest.Plan(t, root, true, selectRust(t, config.Options{"kind": kind}))

			// Assert
			paths := bedrocktest.Paths(result.Steps)
			for _, want := range []string{"Cargo.toml", "rust-toolchain.toml", "README.md"} {
				if !slices.Contains(paths, want) {
					t.Errorf("init plans no %s: %v", want, paths)
				}
			}

			if !slices.ContainsFunc(paths, func(path string) bool { return strings.HasPrefix(path, "crates/") }) {
				t.Errorf("init plans no crate: %v", paths)
			}

			wasm := strings.Contains(bedrocktest.Written(result.Steps, "rust-toolchain.toml"), "wasm32-unknown-unknown")
			if wasm != (kind == "wasm") {
				t.Errorf("toolchain names the wasm target is %t for %s", wasm, kind)
			}
		})
	}
}

func TestInitNamesEachCrateByKindWhenThereAreSeveral(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, true, selectRust(t, config.Options{"kind": "lib,bin"}))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	for _, suffix := range []string{"-lib/Cargo.toml", "-bin/Cargo.toml"} {
		if !slices.ContainsFunc(paths, func(path string) bool {
			return strings.HasPrefix(path, "crates/") && strings.HasSuffix(path, suffix)
		}) {
			t.Errorf("init plans no crate ending %s: %v", suffix, paths)
		}
	}
}

func TestInitRejectsAnEmptyKindEntry(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)
	engine := bedrocktest.Engine(t)

	// Act
	_, err := engine.Build(t.Context(), root, []plan.Selected{selectRust(t, config.Options{"kind": "lib,"})},
		plan.Options{Version: bedrocktest.Version, Dropped: nil, Init: true})

	// Assert
	if err == nil {
		t.Error("a trailing comma in `kind` is accepted")
	}
}

func TestInitKeepsTheProjectsReadme(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"README.md": "mine\n"})

	// Act
	result := bedrocktest.Plan(t, root, true, selectRust(t, nil))

	// Assert
	if slices.Contains(bedrocktest.Paths(result.Steps), "README.md") {
		t.Error("init overwrites the README")
	}
}
