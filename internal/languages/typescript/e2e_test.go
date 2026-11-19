package typescript_test

import (
	"os/exec"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/language"
)

func TestInitKindInstallsTypeChecksLintsTestsAndPassesTheGuard(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("installs packages")
	}

	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun is not installed")
	}

	for _, kind := range []string{"library", "cli", "plugin"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			chosen := bedrocktest.Select(t, "typescript", map[string]string{"kind": kind})
			bedrocktest.Adopt(t, root, true, chosen)

			run := func(args ...string) {
				t.Helper()

				cmd := exec.CommandContext(t.Context(), "bun", args...)
				cmd.Dir = root

				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("bun %v: %v\n%s", args, err, out)
				}
			}

			// Act
			run("install")
			bedrocktest.CommitAll(t, root)

			// Assert
			for _, args := range [][]string{{"check"}, {"run", "oxlint"}, {"test"}, {"fmt", "--check"}} {
				run(args...)
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
