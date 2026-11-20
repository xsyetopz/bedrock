package golang_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
)

// run runs name with args in root and returns whether it exited 0, and its output.
func run(t *testing.T, root, name string, args ...string) (bool, string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), name, args...)
	command.Dir = root

	output, err := command.CombinedOutput()

	return err == nil, string(output)
}

func requireTools(t *testing.T, tools ...string) {
	t.Helper()

	bedrocktest.E2E(t, "runs the Go toolchain")

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("`%s` is not on `PATH`", tool)
		}
	}
}

// initRepo inits kind in a new repository and commits it.
func initRepo(t *testing.T, kind string, platforms ...string) string {
	t.Helper()

	options := config.Options{"kind": kind}
	if len(platforms) != 0 {
		options["platform"] = strings.Join(platforms, ",")
	}

	root := bedrocktest.Repo(t, nil)
	bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "go", options))
	bedrocktest.CommitAll(t, root)

	return root
}

func TestInitBuildsTestsAndKeepsGoModTidy(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"command", "library", "service"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			requireTools(t, "go")

			// Arrange
			root := initRepo(t, kind)
			guarder := bedrocktest.Language(t, "go").(language.Guarder)

			// Act
			var failed []string

			for _, args := range [][]string{{"build", "./..."}, {"test", "./..."}, {"mod", "tidy", "-diff"}} {
				if ok, output := run(t, root, "go", args...); !ok {
					failed = append(failed, "go "+strings.Join(args, " ")+":\n"+output)
				}
			}

			context := bedrocktest.Context(t, root, bedrocktest.Select(t, "go", nil))
			findings, err := guarder.Guard(t.Context(), context, false)

			// Assert
			if err != nil {
				t.Fatal(err)
			}

			if len(failed) != 0 {
				t.Errorf("failed checks:\n%s", strings.Join(failed, "\n"))
			}

			if len(findings) != 0 {
				t.Errorf("guard findings: %v", findings)
			}
		})
	}
}

func TestInitPassesTheFormatAndLintRecipesCommands(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"command", "library", "service"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			requireTools(t, "go", "golangci-lint")

			// Arrange
			root := initRepo(t, kind)

			// Act
			// golangci-lint takes one lock per machine; serial runners wait for it instead of failing.
			_, format := run(t, root, "golangci-lint", "fmt", "--diff")
			linted, lint := run(t, root, "golangci-lint", "run", "--allow-serial-runners")

			// Assert
			if format != "" {
				t.Errorf("`golangci-lint fmt --diff` reports:\n%s", format)
			}

			if !linted || !strings.Contains(lint, "0 issues") {
				t.Errorf("`golangci-lint run` reports:\n%s", lint)
			}
		})
	}
}
