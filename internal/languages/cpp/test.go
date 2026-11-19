package cpp

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

// testName names the `test` subcommand, and the `meson test` command that
// runs a Meson project's tests.
const testName = "test"

// command is one program run with its arguments.
type command []string

// testRuns returns, for each project, the commands that build and test it,
// in the order they run; a project's commands stop at the first that fails.
func testRuns(ctx context.Context, c *language.Context) ([][]command, error) {
	cmake, err := eachProject(c, cmakeProjects(ctx, c), func(project string) ([][]command, error) {
		build, configureErr := configureCmake(ctx, c, project)
		if configureErr != nil {
			return nil, configureErr
		}

		return [][]command{{
			{"cmake", "--build", build},
			{"ctest", "--test-dir", build, "--output-on-failure", "--no-tests=ignore"},
		}}, nil
	})
	if err != nil {
		return nil, err
	}

	meson, err := eachProject(c, mesonProjects(ctx, c), func(project string) ([][]command, error) {
		build, configureErr := configureMeson(ctx, c, project)
		if configureErr != nil {
			return nil, configureErr
		}

		return [][]command{{{"meson", testName, "-C", build, "--print-errorlogs"}}}, nil
	})
	if err != nil {
		return nil, err
	}

	runs := slices.Concat(cmake, meson)

	for _, project := range xmakeProjects(ctx, c) {
		runs = append(runs, []command{{"xmake", "test", "-y", "-P", filepath.Join(c.Root, project)}})
	}

	return runs, nil
}

// passes runs one command with its output on the terminal, and reports
// whether it exited 0.
func passes(ctx context.Context, c *language.Context, cmd command) bool {
	_, _ = fmt.Fprintf(c.Stderr, "$ %s\n", strings.Join(cmd, " "))

	result, err := c.Runner.Run(ctx, execx.Command{Dir: c.Root, Name: cmd[0], Args: cmd[1:], Stream: true})
	if err != nil {
		_, _ = fmt.Fprintln(c.Stderr, err)

		return false
	}

	return result.OK()
}

// allPass runs the commands one after another, since a project's tests need
// its build; the first that fails ends the run.
func allPass(ctx context.Context, c *language.Context, commands []command) bool {
	for _, cmd := range commands {
		if !passes(ctx, c, cmd) {
			return false
		}
	}

	return true
}

// runTests builds each CMake, Meson, and xmake project in its bedrock build
// directory and runs its tests: CTest's, `meson test`'s, and `xmake test`'s.
// A subproject that cannot configure is skipped with a warning, as `bedrock
// cpp compile-commands` skips it, and every project runs even after one
// fails, so a run reports them all. `swift test` covers SwiftPM's C++ targets.
// The projects run one at a time, so each one's output reads as a block.
func runTests(ctx context.Context, c *language.Context, _ []string) (int, error) {
	runs, err := testRuns(ctx, c)
	if err != nil {
		_, _ = fmt.Fprintln(c.Stderr, err)

		return 1, nil
	}

	failed := false

	for _, commands := range runs {
		if !allPass(ctx, c, commands) {
			failed = true
		}
	}

	if failed {
		return 1, nil
	}

	return 0, nil
}
