package typescript_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const (
	pluginManifest = ".claude-plugin/plugin.json"
	pluginTypes    = ".claude-plugin/types/claude-code/index.d.ts"
	typedTsconfig  = `{"files": [".claude-plugin/types/claude-code/index.d.ts"]}`
)

// recipeRun is what one run of the oxlint recipe printed: the arguments bun
// received and the recipe's own notes on stderr.
type recipeRun struct{ args, notes string }

// oxlintRecipe adopts typescript into a repo with files, then runs the shared
// oxlint recipe with a bun that prints its arguments, and returns bun's
// arguments and the recipe's stderr.
func oxlintRecipe(t *testing.T, files map[string]string, args ...string) recipeRun {
	t.Helper()

	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}

	root := bedrocktest.Repo(t, files)
	bedrocktest.Adopt(t, root, false, selected(t))

	bin := t.TempDir()
	bedrocktest.Write(t, bin, "bun", "#!/usr/bin/env bash\necho \"$*\"\n")

	if chmodErr := os.Chmod(filepath.Join(bin, "bun"), 0o700); chmodErr != nil {
		t.Fatal(chmodErr)
	}

	cmd := exec.CommandContext(t.Context(), just, append([]string{"typescript::oxlint"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BEDROCK_LINT_LOCK="+filepath.Join(t.TempDir(), "lock"))

	var errs strings.Builder

	cmd.Stderr = &errs

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("just typescript::oxlint: %v\n%s", err, errs.String())
	}

	return recipeRun{args: string(out), notes: errs.String()}
}

func TestOxlintRecipeSkipsAClaudePluginWithoutItsStampedTypes(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"plugins/untyped/" + pluginManifest: "{}",
		"plugins/untyped/tsconfig.json":     typedTsconfig,
		"plugins/typed/" + pluginManifest:   "{}",
		"plugins/typed/tsconfig.json":       typedTsconfig,
		"plugins/typed/" + pluginTypes:      "",
		"plugins/plain/" + pluginManifest:   "{}",
		"plugins/plain/tsconfig.json":       "{}",
	}

	// Act
	run := oxlintRecipe(t, files)

	// Assert
	if !strings.Contains(run.args, "--ignore-pattern=plugins/untyped/**") {
		t.Errorf("oxlint args %q lack the untyped plugin's ignore pattern", run.args)
	}

	if strings.Count(run.args, "--ignore-pattern") != 1 {
		t.Errorf("oxlint args %q skip a plugin that is typed or does not list the types", run.args)
	}

	for _, want := range []string{"plugins/untyped", "claude --plugin-dir plugins/untyped"} {
		if !strings.Contains(run.notes, want) {
			t.Errorf("stderr %q lacks %q", run.notes, want)
		}
	}
}

func TestOxlintRecipeLintsAnUntypedPluginNamedByPath(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"plugins/untyped/" + pluginManifest: "{}",
		"plugins/untyped/tsconfig.json":     typedTsconfig,
	}

	// Act
	run := oxlintRecipe(t, files, "--fix", "./plugins/untyped/src")

	// Assert
	if strings.Contains(run.args, "--ignore-pattern") {
		t.Errorf("oxlint args %q skip a plugin the caller named", run.args)
	}

	if !strings.Contains(run.args, "--fix ./plugins/untyped/src") {
		t.Errorf("oxlint args %q lack the caller's arguments", run.args)
	}
}
