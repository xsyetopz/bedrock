package cpp_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
)

func TestTestBuildsAndTestsEachProjectWithItsOwnTool(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{
		"CMakeLists.txt": "project(a)\n",
		"b/meson.build":  "project('b')\n",
		"c/xmake.lua":    "target('c')\n",
	}, nil)
	pass := func(execx.Command) execx.Result { return ok("") }
	h.runner.tools = map[string]toolFunc{
		"cmake": func(command execx.Command) execx.Result {
			if slices.Contains(command.Args, "-S") {
				return cmakeThatWritesCommands(t)(command)
			}

			return ok("")
		},
		"ctest": pass, "meson": pass, "xmake": pass,
	}

	// Act
	code, err := subcommand(t, "test").Run(t.Context(), h.ctx, nil)

	// Assert
	if err != nil || code != 0 {
		t.Fatalf("code, err = %d, %v; stderr: %s", code, err, h.stderr)
	}

	for _, prefix := range []string{"cmake --build", "ctest --test-dir", "meson test -C", "xmake test -y -P"} {
		if len(h.runner.called(prefix)) != 1 {
			t.Errorf("calls = %v, want one `%s`", h.runner.calls, prefix)
		}
	}
}

func TestTestRunsEveryProjectEvenAfterOneFailsAndExitsOne(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{"CMakeLists.txt": "project(a)\n", "b/meson.build": "project('b')\n"}, nil)
	h.runner.tools = map[string]toolFunc{
		"cmake": func(command execx.Command) execx.Result {
			if slices.Contains(command.Args, "-S") {
				return cmakeThatWritesCommands(t)(command)
			}

			return ok("")
		},
		"ctest": func(execx.Command) execx.Result { return failed("1 test failed") },
		"meson": func(execx.Command) execx.Result { return ok("") },
	}

	// Act
	code, err := subcommand(t, "test").Run(t.Context(), h.ctx, nil)

	// Assert
	if err != nil || code != 1 {
		t.Errorf("code, err = %d, %v; want 1, nil", code, err)
	}

	if len(h.runner.called("meson test")) != 1 {
		t.Errorf("meson did not run after ctest failed: %v", h.runner.calls)
	}
}

func TestTestStopsAProjectAtItsFirstFailingCommand(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{"CMakeLists.txt": "project(a)\n"}, nil)
	good := cmakeThatWritesCommands(t)
	h.runner.tools = map[string]toolFunc{
		"cmake": func(command execx.Command) execx.Result {
			if slices.Contains(command.Args, "--build") {
				return failed("compile error")
			}

			return good(command)
		},
		"ctest": func(execx.Command) execx.Result { return ok("") },
	}

	// Act
	code, _ := subcommand(t, "test").Run(t.Context(), h.ctx, nil)

	// Assert
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}

	if len(h.runner.called("ctest")) != 0 {
		t.Errorf("ctest ran after the build failed: %v", h.runner.calls)
	}
}

func TestTestReportsAConfigureFailureOfTheRootProject(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{"meson.build": "project('a')\n"}, nil)
	h.runner.tools = map[string]toolFunc{"meson": func(execx.Command) execx.Result { return failed("\nbad option\n") }}

	// Act
	code, err := subcommand(t, "test").Run(t.Context(), h.ctx, nil)

	// Assert
	if err != nil || code != 1 {
		t.Errorf("code, err = %d, %v; want 1, nil", code, err)
	}

	if want := "meson setup of . failed: bad option"; !strings.Contains(h.stderr.String(), want) {
		t.Errorf("stderr = %q, want it to hold %q", h.stderr, want)
	}
}

func TestTestFindsTheProjectsWhenAHookPointsGitAtAnotherRepository(t *testing.T) {
	// Arrange
	h := newHarness(t, map[string]string{"CMakeLists.txt": "project(a)\n"}, nil)
	h.runner.tools = map[string]toolFunc{
		"cmake": func(command execx.Command) execx.Result {
			if slices.Contains(command.Args, "-S") {
				return cmakeThatWritesCommands(t)(command)
			}

			return ok("")
		},
		"ctest": func(execx.Command) execx.Result { return ok("") },
	}
	// A hook in a worktree exports the worktree's git location to every program it starts.
	other := bedrocktest.Repo(t, nil)
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_WORK_TREE", other)

	// Act
	code, err := subcommand(t, "test").Run(t.Context(), h.ctx, nil)

	// Assert
	if err != nil || code != 0 {
		t.Fatalf("code, err = %d, %v; stderr: %s", code, err, h.stderr)
	}

	if len(h.runner.called("ctest --test-dir")) != 1 {
		t.Errorf("calls = %v, want the root project's `ctest`", h.runner.calls)
	}
}
