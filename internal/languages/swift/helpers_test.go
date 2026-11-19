package swift_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/swift"
	"github.com/xsyetopz/bedrock/internal/plan"
)

const xctestSource = "import XCTest\nfinal class A: XCTestCase {\n  func testB() {}\n}\n"

const strictPackage = `{"toolsVersion":{"_version":"6.4.0"},"swiftLanguageVersions":["6"],"targets":[]}`

// dumper stands in for SwiftPM: it answers `dump-package` with a canned
// result and sends every other command to the real runner, so file listing
// still works.
type dumper struct {
	execx.Runner

	stdout  string
	stderr  string
	code    int
	present bool
}

func (d dumper) LookPath(string) bool { return d.present }

func (d dumper) Run(ctx context.Context, command execx.Command) (execx.Result, error) {
	if slices.Contains(command.Args, "dump-package") {
		return execx.Result{Stdout: []byte(d.stdout), Stderr: []byte(d.stderr), Code: d.code}, nil
	}

	return d.Runner.Run(ctx, command)
}

// guardWith runs the Swift guard in a repository whose `swift` prints dump.
func guardWith(t *testing.T, files map[string]string, fake dumper, fix bool) ([]language.Finding, string) {
	t.Helper()

	root := bedrocktest.Repo(t, files)
	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", nil))
	fake.Runner = c.Runner
	c.Runner = fake

	found, err := swift.New().Guard(t.Context(), c, fix)
	if err != nil {
		t.Fatal(err)
	}

	return found, root
}

// namedRepo returns an empty git repository in a directory called name,
// because the starter package takes its name from the directory.
func namedRepo(t *testing.T, name string) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(root, 0o750); err != nil {
		t.Fatal(err)
	}

	runner := execx.System{Stdout: io.Discard, Stderr: io.Discard}

	_, err := execx.Output(
		t.Context(),
		runner,
		execx.Command{Dir: root, Name: "git", Args: []string{"init", "--quiet"}},
	)
	if err != nil {
		t.Fatal(err)
	}

	return root
}

// strictDump is a `swift` that reports a strict package.
func strictDump() dumper { return dumper{stdout: strictPackage, present: true} }

// planSwift plans an adopt of Swift into a repository holding files.
func planSwift(t *testing.T, files map[string]string) (string, plan.Plan) {
	t.Helper()

	root := bedrocktest.Repo(t, files)

	return root, bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "swift", nil))
}

// subcommandResult is what a subcommand returned and printed.
type subcommandResult struct {
	status int
	stdout string
	stderr string
}

// runSubcommand runs `convert-tests` and returns its status and output.
func runSubcommand(t *testing.T, root string, args ...string) subcommandResult {
	t.Helper()

	c := bedrocktest.Context(t, root, bedrocktest.Select(t, "swift", nil))

	var out, errs bytes.Buffer

	c.Stdout, c.Stderr = &out, &errs

	status, err := swift.New().Subcommands()[0].Run(t.Context(), c, args)
	if err != nil {
		t.Fatal(err)
	}

	return subcommandResult{status: status, stdout: out.String(), stderr: errs.String()}
}
