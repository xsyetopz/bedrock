package haxe_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/languages/haxe"
)

// scriptedRunner runs git for real, so file listing is the real one, and
// answers the Haxe toolchain's programs from a script while recording them.
type scriptedRunner struct {
	git execx.System
	// replies maps a command line to its result; others pass with no output.
	replies map[string]execx.Result
	calls   []string
	mu      sync.Mutex
}

func (r *scriptedRunner) Run(ctx context.Context, command execx.Command) (execx.Result, error) {
	if command.Name == "git" {
		return r.git.Run(ctx, command)
	}

	line := command.Name + " " + strings.Join(command.Args, " ")

	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, line)

	return r.replies[line], nil
}

func (*scriptedRunner) LookPath(string) bool { return true }

type outcome struct {
	stdout string
	stderr string
	calls  []string
	code   int
}

// runCommand runs `bedrock haxe <name>` over a repository of files.
func runCommand(
	t *testing.T,
	files map[string]string,
	replies map[string]execx.Result,
	name string,
	args ...string,
) outcome {
	t.Helper()

	root := bedrocktest.Repo(t, files)
	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "haxe", nil))
	runner := &scriptedRunner{
		git:     execx.System{Stdout: nil, Stderr: nil},
		replies: replies,
		calls:   nil,
		mu:      sync.Mutex{},
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	c.Runner, c.Stdout, c.Stderr = runner, stdout, stderr

	for _, sub := range haxe.New().Subcommands() {
		if sub.Name != name {
			continue
		}

		code, err := sub.Run(t.Context(), c, args)
		if err != nil {
			t.Fatal(err)
		}

		return outcome{code: code, stdout: stdout.String(), stderr: stderr.String(), calls: runner.calls}
	}

	t.Fatalf("no subcommand %s", name)

	return outcome{}
}

func TestSpecNamesTheHaxeBlockAndItsAliases(t *testing.T) {
	t.Parallel()

	// Act
	spec := haxe.New().Spec()

	// Assert
	if spec.ID != "haxe" || spec.Block != "haxe-bedrock" || !slices.Contains(spec.Aliases, "hx") {
		t.Errorf("spec %+v", spec)
	}
}

func TestTypecheckCompilesOnlyHxmlFilesThatNameATarget(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"build.hxml":                    "-cp src\n--main Main\n--js bin/main.js\n",
		"libs.hxml":                     "-lib utest\n",
		"services/api/build.hxml":       "# -js ignored\n-cp src\n--interp\n",
		"app.hxml":                      "common.hxml\n--main App\n",
		"common.hxml":                   "-cp src\n-x Entry\n",
		".haxelib/utest/1/haxelib.hxml": "-js out.js\n",
	}

	// Act
	got := runCommand(t, files, nil, "typecheck")

	// Assert
	want := []string{
		"haxe --no-output --each app.hxml",
		"haxe --no-output --each build.hxml",
		"haxe --no-output --each common.hxml",
		"haxe --no-output --each services/api/build.hxml",
	}
	if got.code != 0 || !slices.Equal(got.calls, want) {
		t.Errorf("code %d, calls %q, want %q", got.code, got.calls, want)
	}
}

func TestTypecheckRunsEveryBuildAndFailsWhenOneFails(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"a.hxml": "-js a.js\n", "b.hxml": "-js b.js\n"}
	replies := map[string]execx.Result{"haxe --no-output --each a.hxml": {Stdout: nil, Stderr: nil, Code: 1}}

	// Act
	got := runCommand(t, files, replies, "typecheck")

	// Assert
	if got.code != 1 || len(got.calls) != 2 {
		t.Errorf("code %d, calls %q", got.code, got.calls)
	}
}

func TestTestRunsTestHxmlFilesThatRunAndNamesOnesThatDoNot(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"test.hxml":        "-cp test\n--main TestMain\n-lib utest\n",
		"test-interp.hxml": "-cp test\n--main TestMain\n--interp\n",
		"test-cmd.hxml":    "-js out.js\n--cmd node out.js\n",
		"build.hxml":       "-js out.js\n--cmd node out.js\n",
		"testing.hxml":     "--interp\n",
	}

	// Act
	got := runCommand(t, files, nil, "test")

	// Assert
	want := []string{"haxe test-cmd.hxml", "haxe test-interp.hxml"}
	if got.code != 0 || !slices.Equal(got.calls, want) {
		t.Errorf("code %d, calls %q, want %q", got.code, got.calls, want)
	}

	if !strings.Contains(got.stderr, "not run `test.hxml`: it names no `--interp`, `--run`, or `--cmd`") {
		t.Errorf("stderr:\n%s", got.stderr)
	}
}

func TestTestReadsFlagsOfIncludedHxmlFilesAndStopsAtCycles(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"test.hxml":   "common.hxml\n--main TestMain\n",
		"common.hxml": "-cp src\n--interp\n",
		"tests.hxml":  "a.hxml\n",
		"a.hxml":      "tests.hxml\nmissing.hxml\n-cp src\n",
	}

	// Act
	got := runCommand(t, files, nil, "test")

	// Assert
	if !slices.Equal(got.calls, []string{"haxe test.hxml"}) {
		t.Errorf("calls %q", got.calls)
	}

	if !strings.Contains(got.stderr, "not run `tests.hxml`") {
		t.Errorf("stderr:\n%s", got.stderr)
	}
}

func TestLintPassesOnNoErrorsAndFailsOnAnErrorOrNoSummary(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"src/app/Main.hx":  "",
		"test/TestMain.hx": "",
		"Build.hx":         "",
		".haxelib/utest/1,13,2/src/utest/Test.hx": "",
		"README.md": "",
	}
	invocation := "haxelib run checkstyle -s Build.hx -s src -s test -c checkstyle.json --nostyle"

	for name, want := range map[string]struct {
		report string
		code   int
	}{
		"warnings only": {"Total Issues: 2 (Errors: 0, Warnings: 2, Infos: 0)\n", 0},
		"no issues":     {"No issues found.\n", 0},
		"256 errors":    {"Total Issues: 300 (Errors: 256, Warnings: 44, Infos: 0)\n", 1},
		"no summary":    {"Error: Unknown command\n", 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			replies := map[string]execx.Result{invocation: {Stdout: []byte(want.report), Stderr: nil, Code: 0}}

			// Act
			got := runCommand(t, files, replies, "lint")

			// Assert
			if got.code != want.code || !slices.Equal(got.calls, []string{invocation}) {
				t.Errorf("code %d, calls %q", got.code, got.calls)
			}

			if got.stdout != want.report {
				t.Errorf("stdout %q", got.stdout)
			}
		})
	}
}

func TestLintAndFormatWithoutHaxeSourcesRunNothing(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"lint", "format"} {
		// Act
		got := runCommand(t, map[string]string{"build.hxml": buildHxml}, nil, name)

		// Assert
		if got.code != 0 || len(got.calls) != 0 {
			t.Errorf("%s: code %d, calls %q", name, got.code, got.calls)
		}
	}
}

func TestFormatChecksOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"src/Main.hx": ""}

	// Act
	plain := runCommand(t, files, nil, "format")
	checked := runCommand(t, files, nil, "format", "--check")

	// Assert
	if !slices.Equal(plain.calls, []string{"haxelib run formatter -s src"}) {
		t.Errorf("plain calls %q", plain.calls)
	}

	if !slices.Equal(checked.calls, []string{"haxelib run formatter -s src --check"}) || checked.code != 0 {
		t.Errorf("check: code %d, calls %q", checked.code, checked.calls)
	}
}
