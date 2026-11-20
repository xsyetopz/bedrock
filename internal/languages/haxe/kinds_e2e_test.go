package haxe_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests here install each kind's pinned libraries from haxelib into the
// repo's `.haxelib` with `just haxe::setup`, and skip when a tool is absent.

// runtimeTarget is a test hxml and the program its `--cmd` or the compiler
// itself runs.
type runtimeTarget struct{ hxml, runtime string }

func libraryTargets() []runtimeTarget {
	return []runtimeTarget{
		{"test.hxml", "haxe"}, {"test-jvm.hxml", "java"}, {"test-python.hxml", "python3"}, {"test-js.hxml", "node"},
	}
}

// requireSubs fails for each named subcommand that exits non-zero in root.
func requireSubs(t *testing.T, root string, names ...string) {
	t.Helper()

	for _, name := range names {
		args := []string{}
		if name == "format" {
			args = append(args, "--check")
		}

		if code := runSub(t, root, name, args...); code != 0 {
			t.Errorf("`bedrock haxe %s` exits %d", name, code)
		}
	}
}

// eachTarget runs `haxe <hxml>` for every target whose runtime is present and
// checks it exits 0 when pass is true, and non-zero when it is not.
func eachTarget(t *testing.T, root string, pass bool) {
	t.Helper()

	for _, target := range libraryTargets() {
		t.Run(target.hxml, func(t *testing.T) {
			t.Parallel()
			requireTools(t, target.runtime)

			if ok, output := run(t, root, "haxe", target.hxml); ok != pass {
				t.Errorf("`haxe %s` passes: %v, want %v:\n%s", target.hxml, ok, pass, output)
			}
		})
	}
}

func TestE2ELibraryKindChecksAndTestsOnEachTarget(t *testing.T) {
	t.Parallel()
	requireTools(t, "just")

	// Arrange
	root := initKind(t, "library", "")
	setup(t, root)

	// Act and assert
	requireSubs(t, root, "format", "lint", "typecheck")
	t.Run("targets", func(t *testing.T) {
		t.Parallel()
		eachTarget(t, root, true)
	})
}

func TestE2ELibraryKindFailingTestFailsOnEachTarget(t *testing.T) {
	t.Parallel()
	requireTools(t, "just")

	// Arrange
	root := initKind(t, "library", "")
	setup(t, root)

	greeting, _ := filepath.Glob(filepath.Join(root, "src", "*", "Greeting.hx"))
	if len(greeting) != 1 {
		t.Fatalf("Greeting.hx: %v", greeting)
	}

	data, err := os.ReadFile(greeting[0])
	if err != nil {
		t.Fatal(err)
	}

	if err = os.WriteFile(
		greeting[0],
		[]byte(strings.Replace(string(data), "Hello, ", "Bye, ", 1)),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	// Act and assert
	t.Run("targets", func(t *testing.T) {
		t.Parallel()
		eachTarget(t, root, false)
	})
}

func TestE2ECommandKindChecksTestsAndBuildsNatively(t *testing.T) {
	t.Parallel()
	requireTools(t, "just", "c++")

	// Arrange
	root := initKind(t, "command", "linux,macos,windows")
	setup(t, root)

	// Act and assert
	requireSubs(t, root, "format", "lint", "typecheck", "test")
	t.Run("jvm", func(t *testing.T) {
		t.Parallel()
		checkJar(t, root)
	})
	t.Run("native", func(t *testing.T) {
		t.Parallel()
		checkNative(t, root)
	})
}

// checkJar builds `build-jvm.hxml` and runs the jar it writes.
func checkJar(t *testing.T, root string) {
	t.Helper()
	requireTools(t, "java")

	if ok, output := run(t, root, "haxe", "build-jvm.hxml"); !ok {
		t.Fatalf("`haxe build-jvm.hxml`:\n%s", output)
	}

	jars, _ := filepath.Glob(filepath.Join(root, "bin", "*.jar"))
	if len(jars) != 1 {
		t.Fatalf("bin jars: %v", jars)
	}

	if ok, output := run(
		t,
		root,
		"java",
		"-jar",
		jars[0],
	); !ok ||
		strings.ReplaceAll(output, "\r\n", "\n") != "Hello, world!\n" {
		t.Errorf("jar prints %q", output)
	}
}

// checkNative builds for this machine's platform, runs the binary, and checks
// that a platform of another OS is refused.
func checkNative(t *testing.T, root string) {
	t.Helper()

	host := hostPlatform()

	if ok, output := run(t, root, "just", "haxe::build", host); !ok {
		t.Fatalf("`just haxe::build %s`:\n%s", host, output)
	}

	binaries, _ := filepath.Glob(filepath.Join(root, "dist", host, "*"))
	if len(binaries) != 1 {
		t.Fatalf("dist/%s: %v", host, binaries)
	}

	if ok, output := run(t, root, binaries[0]); !ok || strings.ReplaceAll(output, "\r\n", "\n") != "Hello, world!\n" {
		t.Errorf("binary prints %q", output)
	}

	other := "linux"
	if host == other {
		other = "windows"
	}

	if ok, output := run(t, root, "just", "haxe::build", other); ok {
		t.Errorf("`just haxe::build %s` passes on %s:\n%s", other, host, output)
	}
}
