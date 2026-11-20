package rust_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/rust"
)

const inlineTests = "pub fn answer() -> u8 {\n    42\n}\n\n#[cfg(test)]\nmod tests {\n    #[test]\n    fn t() {}\n}\n"

// adopted returns a committed repository the adopt left matching, with files
// written over it.
func adopted(t *testing.T, files map[string]string) string {
	t.Helper()

	root := bedrocktest.Repo(t, map[string]string{"Cargo.toml": appManifest, "src/lib.rs": libSource})
	bedrocktest.Adopt(t, root, false, selectRust(t, nil))

	for path, content := range files {
		bedrocktest.Write(t, root, path, content)
	}

	bedrocktest.CommitAll(t, root)

	return root
}

func guard(t *testing.T, root string, fix bool) []string {
	t.Helper()

	findings, err := rust.New().Guard(t.Context(), bedrocktest.Context(t, root, selectRust(t, nil)), fix)
	if err != nil {
		t.Fatal(err)
	}

	return findings
}

func TestGuardFindsNothingInAnAdoptedRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, nil)

	// Act
	findings := guard(t, root, false)

	// Assert
	if len(findings) != 0 {
		t.Errorf("findings: %v", findings)
	}
}

func TestGuardReportsTestsKeptInTheFileTheyTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{"src/lib.rs": inlineTests})

	// Act
	findings := guard(t, root, false)

	// Assert
	want := "src/lib.rs:6: mod `tests` keeps its tests in this file; move them to `src/tests.rs`"
	if len(findings) != 1 || !strings.HasPrefix(findings[0], want) {
		t.Errorf("findings: %v", findings)
	}
}

func TestGuardReportsATestModuleSetByPathAttribute(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{
		"src/lib.rs":       "#[cfg(test)]\n#[path = \"elsewhere.rs\"]\nmod tests;\n",
		"src/elsewhere.rs": "#[test]\nfn t() {}\n",
	})

	// Act
	findings := guard(t, root, false)

	// Assert
	if len(findings) != 1 || !strings.Contains(findings[0], "mod `tests` sets its file with `#[path]`") {
		t.Errorf("findings: %v", findings)
	}
}

func TestGuardFixMovesTestsToTheirOwnFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{"src/lib.rs": inlineTests})

	// Act
	findings := guard(t, root, true)

	// Assert
	if len(findings) != 0 {
		t.Errorf("findings after the fix: %v", findings)
	}

	if !strings.Contains(bedrocktest.Read(t, root, "src/lib.rs"), "mod tests;") {
		t.Errorf("`lib.rs` does not declare the module:\n%s", bedrocktest.Read(t, root, "src/lib.rs"))
	}

	if !strings.Contains(bedrocktest.Read(t, root, "src/tests.rs"), "fn t()") {
		t.Error("`src/tests.rs` does not hold the tests")
	}
}

func TestGuardReportsLintSuppressionsWithoutAReason(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{
		"src/lib.rs": "#[allow(dead_code)]\nfn a() {}\n\n#[expect(dead_code)]\nfn b() {}\n",
	})

	// Act
	findings := guard(t, root, false)

	// Assert
	if len(findings) != 2 ||
		!strings.HasPrefix(findings[0], "src/lib.rs:1: `#[allow]` hides a lint") ||
		!strings.HasPrefix(findings[1], "src/lib.rs:4: `#[expect]` without a reason") {
		t.Errorf("findings: %v", findings)
	}
}

func TestGuardReportsAMemberThatSkipsTheWorkspaceLints(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{"crates/a/Cargo.toml": appManifest, "crates/a/src/lib.rs": libSource})

	// Act
	findings := guard(t, root, false)

	// Assert
	want := "`crates/a/Cargo.toml`: differs from what `bedrock` writes; run `bedrock update`"
	if len(findings) != 1 || findings[0] != want {
		t.Errorf("findings: %v", findings)
	}
}

func TestGuardReportsAnIntegrationTestWithoutCfgTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{"tests/it.rs": "#[test]\nfn t() {}\n"})

	// Act
	findings := guard(t, root, false)

	// Assert
	if len(findings) != 1 || !strings.Contains(findings[0], "`tests/it.rs`: differs") {
		t.Errorf("findings: %v", findings)
	}
}

func TestGuardFixGatesAnIntegrationTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{"tests/it.rs": "#[test]\nfn t() {}\n"})

	// Act
	findings := guard(t, root, true)

	// Assert
	if len(findings) != 0 {
		t.Errorf("findings after the fix: %v", findings)
	}

	if !strings.Contains(bedrocktest.Read(t, root, "tests/it.rs"), "#![cfg(test)]") {
		t.Error("the integration test is not gated")
	}
}

func TestGuardReportsModRsAndNamesItsReplacement(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{"src/lib.rs": "mod foo;\n", "src/foo/mod.rs": "pub fn f() {}\n"})

	// Act
	findings := guard(t, root, false)

	// Assert
	want := "`src/foo/mod.rs`: name it `src/foo.rs`, beside its directory (`bedrock update` does it)"
	if len(findings) != 1 || findings[0] != want {
		t.Errorf("findings: %v", findings)
	}
}

func TestGuardReportsAModRsThatRewritesPathsOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{
		"src/lib.rs":     "mod foo;\n",
		"src/foo/mod.rs": "#[path = \"x.rs\"]\nmod y;\n",
		"src/foo/x.rs":   "pub fn g() {}\n",
	})

	// Act
	findings := guard(t, root, false)

	// Assert
	want := "`src/foo/mod.rs`: name it `src/foo.rs`, beside its directory (`bedrock update` does it)"
	if len(findings) != 1 || findings[0] != want {
		t.Errorf("findings: %v", findings)
	}
}

func TestTestFilesClassifiesWhatRustcBuildsOnlyUnderTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := adopted(t, map[string]string{
		"src/lib.rs":      "#[cfg(test)]\nmod tests;\n",
		"src/tests.rs":    "#[test]\nfn t() {}\n",
		"src/gated.rs":    "#![cfg(test)]\nfn g() {}\n",
		"src/plain.rs":    "pub fn p() {}\n",
		"tests/it.rs":     "#![cfg(test)]\n",
		"src/tests/in.rs": "pub fn q() {}\n",
	})
	files := []string{"src/lib.rs", "src/tests.rs", "src/gated.rs", "src/plain.rs", "tests/it.rs", "src/tests/in.rs"}
	context := bedrocktest.Context(t, root, selectRust(t, nil))

	// Act
	var classifier language.TestFileClassifier = rust.New()

	got, err := classifier.TestFiles(t.Context(), context, files)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	for file, want := range map[string]bool{
		"src/lib.rs": false, "src/tests.rs": true, "src/gated.rs": true,
		"src/plain.rs": false, "tests/it.rs": true, "src/tests/in.rs": true,
	} {
		if got[file] != want {
			t.Errorf("%s is test-only: %t, want %t", file, got[file], want)
		}
	}
}

func TestAfterApplyMigratesAnOldEditionWhenCargoIsPresent(t *testing.T) {
	t.Parallel()

	bedrocktest.E2E(t, "runs `cargo fix`")

	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml": "[package]\nname = \"old\"\nversion = \"0.1.0\"\nedition = \"2018\"\n",
		"src/lib.rs": "extern \"C\" {\n    pub fn abs(x: i32) -> i32;\n}\n",
	})
	context := bedrocktest.Context(t, root, selectRust(t, nil))

	if !context.Runner.LookPath("cargo") {
		t.Skip("`cargo` is not on `PATH`")
	}

	// Act
	err := rust.New().AfterApply(t.Context(), context)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if !strings.Contains(bedrocktest.Read(t, root, "Cargo.toml"), `edition = "2024"`) {
		t.Errorf("manifest keeps its edition:\n%s", bedrocktest.Read(t, root, "Cargo.toml"))
	}

	if !strings.Contains(bedrocktest.Read(t, root, "src/lib.rs"), `unsafe extern "C"`) {
		t.Error("the code is not migrated")
	}

	pending, err := rust.New().RunPending(t.Context(), context)
	if err != nil || pending {
		t.Errorf("migration still pending: %t, %v", pending, err)
	}
}

func TestAfterApplyFailsWhenTheMigrationCannotCompile(t *testing.T) {
	t.Parallel()

	bedrocktest.E2E(t, "runs `cargo fix`")

	root := bedrocktest.Repo(t, map[string]string{
		"Cargo.toml": "[package]\nname = \"broken\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
		"src/lib.rs": "pub fn f() -> u8 { missing }\n",
	})
	context := bedrocktest.Context(t, root, selectRust(t, nil))

	if !context.Runner.LookPath("cargo") {
		t.Skip("`cargo` is not on `PATH`")
	}

	// Act
	err := rust.New().AfterApply(t.Context(), context)

	// Assert
	if err == nil {
		t.Fatal("a failed migration reports no error")
	}

	manifest, readErr := os.ReadFile(filepath.Join(root, "Cargo.toml"))
	if readErr != nil || !strings.Contains(string(manifest), `edition = "2021"`) {
		t.Errorf("manifest changed: %s, %v", manifest, readErr)
	}

	if pending, _ := rust.New().RunPending(t.Context(), context); !pending {
		t.Error("the failed migration is not pending")
	}
}
