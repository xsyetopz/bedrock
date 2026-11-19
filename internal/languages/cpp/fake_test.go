package cpp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

// toolFunc stands in for a build tool's program.
type toolFunc func(command execx.Command) execx.Result

// fakeRunner runs git for real and the named build tools as fakes, so a test
// drives the language through the repository and the tools' printed data
// without a toolchain installed.
type fakeRunner struct {
	system execx.System
	tools  map[string]toolFunc
	calls  []string
	mu     sync.Mutex
}

func (f *fakeRunner) Run(ctx context.Context, command execx.Command) (execx.Result, error) {
	tool, ok := f.tools[command.Name]
	if !ok {
		return f.system.Run(ctx, command)
	}

	f.mu.Lock()
	f.calls = append(f.calls, command.Name+" "+strings.Join(command.Args, " "))
	f.mu.Unlock()

	return tool(command), nil
}

func (f *fakeRunner) LookPath(name string) bool {
	if _, ok := f.tools[name]; ok {
		return true
	}

	return f.system.LookPath(name)
}

// called returns the recorded fake calls that start with prefix.
func (f *fakeRunner) called(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var found []string

	for _, call := range f.calls {
		if strings.HasPrefix(call, prefix) {
			found = append(found, call)
		}
	}

	return found
}

// harness is a cpp context over a repository with fake tools and captured
// messages.
type harness struct {
	runner *fakeRunner
	ctx    *language.Context
	stderr *bytes.Buffer
	root   string
}

func newHarness(t *testing.T, files map[string]string, tools map[string]toolFunc) *harness {
	t.Helper()

	root := bedrocktest.Repo(t, files)
	runner := &fakeRunner{
		system: execx.System{Stdout: io.Discard, Stderr: io.Discard},
		tools:  tools,
		calls:  nil,
		mu:     sync.Mutex{},
	}
	stderr := &bytes.Buffer{}

	ctx := bedrocktest.Context(t, root, bedrocktest.Select(t, "cpp", nil))
	ctx.Runner = runner
	ctx.Stderr = stderr

	return &harness{runner: runner, ctx: ctx, stderr: stderr, root: root}
}

// subcommand returns cpp's subcommand name.
func subcommand(t *testing.T, name string) language.Subcommand {
	t.Helper()

	provider, ok := bedrocktest.Language(t, "cpp").(language.SubcommandProvider)
	if !ok {
		t.Fatal("cpp has no subcommands")
	}

	for _, sub := range provider.Subcommands() {
		if sub.Name == name {
			return sub
		}
	}

	t.Fatalf("cpp has no subcommand `%s`", name)

	return language.Subcommand{}
}

// testFiles asks cpp which of files only tests compile.
func (h *harness) testFiles(t *testing.T, files []string) []string {
	t.Helper()

	classifier, ok := bedrocktest.Language(t, "cpp").(language.TestFileClassifier)
	if !ok {
		t.Fatal("cpp does not classify test files")
	}

	picked, err := classifier.TestFiles(t.Context(), h.ctx, files)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for name := range picked {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// ok is a program that printed stdout and exited 0.
func ok(stdout string) execx.Result {
	return execx.Result{Stdout: []byte(stdout), Stderr: nil, Code: 0}
}

// failed is a program that printed stderr and exited 1.
func failed(stderr string) execx.Result {
	return execx.Result{Stdout: nil, Stderr: []byte(stderr), Code: 1}
}

// jsonOf encodes value for a fake tool's output.
func jsonOf(t *testing.T, value any) string {
	t.Helper()

	text, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return string(text)
}

// flagValue returns the argument after flag.
func flagValue(command execx.Command, flag string) string {
	index := slices.Index(command.Args, flag)
	if index < 0 || index+1 >= len(command.Args) {
		return ""
	}

	return command.Args[index+1]
}

// cmakeThatWritesCommands is a cmake that configures by writing one entry
// named for its source directory into the build directory.
func cmakeThatWritesCommands(t *testing.T) toolFunc {
	t.Helper()

	return func(command execx.Command) execx.Result {
		build := flagValue(command, "-B")
		entry := []map[string]string{{"file": filepath.Base(flagValue(command, "-S")), "directory": build}}

		if err := os.MkdirAll(build, 0o750); err != nil {
			return failed(err.Error())
		}

		if err := os.WriteFile(
			filepath.Join(build, "compile_commands.json"),
			[]byte(jsonOf(t, entry)),
			0o600,
		); err != nil {
			return failed(err.Error())
		}

		return ok("")
	}
}
