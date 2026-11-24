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

// writingClaude is a claude that, like Claude Code, writes the types of the
// plugin named by --plugin-dir; idleClaude writes nothing.
const (
	writingClaude = `#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
    if [[ "$1" == --plugin-dir ]]; then dir="$2"; fi
    shift
done
[[ "$CLAUDE_CODE_PLUGIN_DIR_WATCH" == 1 && "$ANTHROPIC_BASE_URL" == http://127.0.0.1:9 ]] || exit 1
mkdir -p "$dir/.claude-plugin/types/claude-code"
echo "// Claude Code types" >"$dir/.claude-plugin/types/claude-code/index.d.ts"
`
	idleClaude = "#!/usr/bin/env bash\nexit 1\n"
)

// oxlintRecipe adopts typescript into a repo with files, then runs the shared
// oxlint recipe with a bun that prints its arguments and a claude with the
// given script, both first on PATH. It returns bun's arguments and the
// recipe's stderr.
func oxlintRecipe(t *testing.T, files map[string]string, claude string, args ...string) recipeRun {
	t.Helper()

	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}

	root := bedrocktest.Repo(t, files)
	bedrocktest.Adopt(t, root, false, selected(t))

	bin := t.TempDir()
	bedrocktest.Write(t, bin, "bun", "#!/usr/bin/env bash\necho \"$*\"\n")

	bedrocktest.Write(t, bin, "claude", claude)

	for _, tool := range []string{"bun", "claude"} {
		if chmodErr := os.Chmod(filepath.Join(bin, tool), 0o700); chmodErr != nil {
			t.Fatal(chmodErr)
		}
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

func TestOxlintRecipeSkipsAClaudePluginWhoseTypesCannotBeWritten(t *testing.T) {
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
	run := oxlintRecipe(t, files, idleClaude)

	// Assert
	if !strings.Contains(run.args, "--ignore-pattern=plugins/untyped/**") {
		t.Errorf("oxlint args %q lack the untyped plugin's ignore pattern", run.args)
	}

	if strings.Count(run.args, "--ignore-pattern") != 1 {
		t.Errorf("oxlint args %q skip a plugin that is typed or does not list the types", run.args)
	}

	for _, want := range []string{"plugins/untyped", "wrote no plugins/untyped/" + pluginTypes} {
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
	run := oxlintRecipe(t, files, idleClaude, "--fix", "./plugins/untyped/src")

	// Assert
	if strings.Contains(run.args, "--ignore-pattern") {
		t.Errorf("oxlint args %q skip a plugin the caller named", run.args)
	}

	if !strings.Contains(run.args, "--fix ./plugins/untyped/src") {
		t.Errorf("oxlint args %q lack the caller's arguments", run.args)
	}
}

func TestOxlintRecipeWritesAClaudePluginsMissingTypesThenLintsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"plugins/untyped/" + pluginManifest: "{}",
		"plugins/untyped/tsconfig.json":     typedTsconfig,
	}

	// Act
	run := oxlintRecipe(t, files, writingClaude)

	// Assert
	if strings.Contains(run.args, "--ignore-pattern") {
		t.Errorf("oxlint args %q skip a plugin whose types were written", run.args)
	}

	if strings.Contains(run.notes, "Skipping") {
		t.Errorf("stderr %q reports a skip", run.notes)
	}
}
