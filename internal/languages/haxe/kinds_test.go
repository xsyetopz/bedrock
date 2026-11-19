package haxe_test

import (
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
)

// kindFiles are the files each kind's init writes at the root of a repo.
func kindFiles() map[string][]string {
	return map[string][]string{
		"command": {
			"src/Main.hx",
			"test/TestMain.hx",
			"test.hxml",
			"build-cpp.hxml",
			"build-jvm.hxml",
			"libraries.hxml",
		},
		"library": {
			"haxelib.json", "test/TestMain.hx", "test.hxml", "test-jvm.hxml", "test-python.hxml", "test-js.hxml",
			"libraries.hxml",
		},
	}
}

func kindOptions(kind, platform string) config.Options {
	options := config.Options{"kind": kind}
	if platform != "" {
		options["platform"] = platform
	}

	return options
}

// initKind inits kind in a new repository, commits it, and returns the root.
func initKind(t *testing.T, kind, platform string) string {
	t.Helper()

	root := bedrocktest.Repo(t, nil)
	bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "haxe", kindOptions(kind, platform)))
	bedrocktest.CommitAll(t, root)

	return root
}

func TestInitKindPlansItsFiles(t *testing.T) {
	t.Parallel()

	for kind, want := range kindFiles() {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)

			// Act
			planned := bedrocktest.Plan(t, root, true, bedrocktest.Select(t, "haxe", kindOptions(kind, "")))

			// Assert
			paths := bedrocktest.Paths(planned.Steps)
			for _, path := range want {
				if !slices.Contains(paths, path) {
					t.Errorf("plan misses %s: %v", path, paths)
				}
			}
		})
	}
}

func TestInitCommandRecordsItsPlatforms(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	bedrocktest.Adopt(t, root, true, bedrocktest.Select(t, "haxe", kindOptions("command", "linux,macos,windows")))

	// Assert
	editorconfig := bedrocktest.Read(t, root, ".editorconfig")
	if !strings.Contains(editorconfig, "bedrock_haxe_platform = linux,macos,windows") {
		t.Errorf(".editorconfig does not record the platforms:\n%s", editorconfig)
	}
}

func TestSecondInitPlansNothing(t *testing.T) {
	t.Parallel()

	for kind, platform := range map[string]string{"command": "linux,macos,windows", "library": ""} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)
			selected := bedrocktest.Select(t, "haxe", kindOptions(kind, platform))
			bedrocktest.Adopt(t, root, true, selected)

			// Act
			again := bedrocktest.Plan(t, root, true, selected)

			// Assert
			if len(again.Steps) != 0 {
				t.Errorf("second init plans %v", bedrocktest.Paths(again.Steps))
			}
		})
	}
}

// hostPlatform names the platform this machine builds for.
func hostPlatform() string {
	if runtime.GOOS == "darwin" {
		return "macos"
	}

	return runtime.GOOS
}

// run runs name with args in root, and returns whether it exited 0 with its output.
func run(t *testing.T, root, name string, args ...string) (bool, string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), name, args...)
	command.Dir = root

	output, err := command.CombinedOutput()

	return err == nil, string(output)
}

// requireTools skips unless every tool is on PATH, and in short mode.
func requireTools(t *testing.T, tools ...string) {
	t.Helper()
	requireHaxe(t)

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("`%s` is not on `PATH`", tool)
		}
	}
}

// setup runs `just haxe::setup` in root, which installs the pinned libraries.
func setup(t *testing.T, root string) {
	t.Helper()
	requireTools(t, "just", "haxelib")

	if ok, output := run(t, root, "just", "haxe::setup"); !ok {
		t.Fatalf("`just haxe::setup`:\n%s", output)
	}
}
