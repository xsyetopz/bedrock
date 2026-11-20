package typescript_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/language"
)

// compiled lists what `bun run build` leaves in dist/ for the three platforms
// of an executable; the package name replaces @@.
func compiled() []string {
	return []string{
		"@@-linux-x64", "@@-linux-arm64", "@@-darwin-x64", "@@-darwin-arm64",
		"@@-windows-x64.exe", "@@-windows-arm64.exe",
	}
}

// bun runs bun in root and fails the test on a non-zero exit.
func bun(t *testing.T, root string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "bun", args...)
	cmd.Dir = root

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bun %v: %v\n%s", args, err, out)
	}
}

// buildLeaves builds the project and checks each artifact exists in dist/.
func buildLeaves(t *testing.T, root string, artifacts []string) {
	t.Helper()

	bun(t, root, "run", "build")

	for _, artifact := range artifacts {
		artifact = strings.Replace(artifact, "@@", filepath.Base(root), 1)
		if _, err := os.Stat(filepath.Join(root, "dist", artifact)); err != nil {
			t.Errorf("build left no %s: %v", artifact, err)
		}
	}
}

func TestInitKindInstallsTypeChecksLintsTestsAndPassesTheGuard(t *testing.T) {
	t.Parallel()

	bedrocktest.E2E(t, "installs packages")

	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun is not installed")
	}

	for _, tc := range []struct {
		kind, platform string
		artifacts      []string
	}{
		{"library", "", nil},
		{"cli", "", nil},
		{"plugin", "", nil},
		{"server", "", nil},
		{"react", "", []string{"index.html"}},
		{"cli", "linux,macos,windows", compiled()},
		{"server", "linux,macos,windows", compiled()},
	} {
		t.Run(tc.kind+"@"+tc.platform, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			chosen := bedrocktest.Select(t, "typescript", map[string]string{"kind": tc.kind, "platform": tc.platform})
			bedrocktest.Adopt(t, root, true, chosen)

			// Act
			bun(t, root, "install")
			bedrocktest.CommitAll(t, root)

			// Assert
			for _, args := range [][]string{{"check"}, {"run", "oxlint"}, {"test"}, {"fmt", "--check"}} {
				bun(t, root, args...)
			}

			if len(tc.artifacts) > 0 {
				buildLeaves(t, root, tc.artifacts)
			}

			guarder, ok := bedrocktest.Language(t, "typescript").(language.Guarder)
			if !ok {
				t.Fatal("the language has no guard")
			}

			findings, err := guarder.Guard(t.Context(), bedrocktest.Context(t, root, chosen), false)
			if err != nil {
				t.Fatal(err)
			}

			if len(findings) != 0 {
				t.Errorf("guard findings: %q", findings)
			}
		})
	}
}
