package rust_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
)

// cargo runs `cargo args` in root and returns its exit success and output.
func cargo(t *testing.T, root string, args ...string) (bool, string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), "cargo", args...)
	command.Dir = root

	output, err := command.CombinedOutput()

	return err == nil, string(output)
}

// cargoSharing runs `cargo args` in root with a target directory shared across
// repos, so registry dependencies compile once for many generated workspaces.
func cargoSharing(t *testing.T, root, targetDir string, args ...string) (bool, string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), "cargo", args...)
	command.Dir = root
	command.Env = append(command.Environ(), "CARGO_TARGET_DIR="+targetDir)

	output, err := command.CombinedOutput()

	return err == nil, string(output)
}

// rustupLock serializes toolchain installs: rustup installs the toolchain and
// targets a `rust-toolchain.toml` pins on first use, and concurrent installs
// into one `~/.rustup` race and fail on each other's partial downloads.
var rustupLock sync.Mutex //nolint:gochecknoglobals // one lock for every test that shares ~/.rustup

// installToolchain installs the toolchain and targets root's
// `rust-toolchain.toml` pins, one install at a time. Without rustup, the cargo
// on `PATH` builds as it is.
func installToolchain(t *testing.T, root string) {
	t.Helper()

	if _, err := exec.LookPath("rustup"); err != nil {
		return
	}

	rustupLock.Lock()
	defer rustupLock.Unlock()

	command := exec.CommandContext(t.Context(), "rustup", "toolchain", "install")
	command.Dir = root

	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("rustup toolchain install: %v\n%s", err, output)
	}
}

func requireCargo(t *testing.T) {
	t.Helper()

	bedrocktest.E2E(t, "runs the Rust toolchain")

	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("`cargo` is not on `PATH`")
	}
}

func TestInitPassesFormatClippyTestsAndTheGuard(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			requireCargo(t)

			if _, err := exec.LookPath("bun"); kind == "bun-ffi" && err != nil {
				t.Skip("`bun` is not on `PATH`")
			}

			// Arrange
			root := bedrocktest.Repo(t, nil)
			bedrocktest.Adopt(t, root, true, selectRust(t, config.Options{"kind": kind}))
			bedrocktest.CommitAll(t, root)
			installToolchain(t, root)

			// Act
			// Sequential: the checks share one target directory.
			var failed []string

			for _, args := range [][]string{
				{"fmt", "--all", "--check"},
				{"clippy", "--workspace", "--all-targets", "--", "-D", "warnings"},
				{"test", "--workspace"},
			} {
				if ok, output := cargo(t, root, args...); !ok {
					failed = append(failed, "cargo "+strings.Join(args, " ")+":\n"+output)
				}
			}

			findings := guard(t, root, false)

			// Assert
			if len(failed) != 0 {
				t.Errorf("failed checks:\n%s", strings.Join(failed, "\n"))
			}

			if len(findings) != 0 {
				t.Errorf("guard findings: %v", findings)
			}
		})
	}
}

func TestADependencyTheCrateNeverUsesFailsCargoCheck(t *testing.T) {
	t.Parallel()
	requireCargo(t)

	// Arrange
	root := bedrocktest.Repo(t, nil)
	bedrocktest.Adopt(t, root, true, selectRust(t, config.Options{"kind": "lib"}))
	installToolchain(t, root)

	manifests, err := filepath.Glob(filepath.Join(root, "crates", "*", "Cargo.toml"))
	if err != nil || len(manifests) != 1 {
		t.Fatalf("manifests: %v, %v", manifests, err)
	}

	bedrocktest.Write(t, root, "crates/unused/Cargo.toml",
		"[package]\nname = \"unused\"\nversion = \"0.1.0\"\nedition = \"2024\"\npublish = false\n")
	bedrocktest.Write(t, root, "crates/unused/src/lib.rs", "")

	relative, err := filepath.Rel(root, manifests[0])
	if err != nil {
		t.Fatal(err)
	}

	bedrocktest.Write(t, root, filepath.ToSlash(relative),
		bedrocktest.Read(t, root, filepath.ToSlash(relative))+"\n[dependencies]\nunused = { path = \"../unused\" }\n")

	// Act
	ok, output := cargo(t, root, "check", "--workspace")

	// Assert
	if ok || !strings.Contains(output, "cargo::unused_dependencies") {
		t.Errorf("cargo check passed or missed the lint (ok=%t):\n%s", ok, output)
	}
}
