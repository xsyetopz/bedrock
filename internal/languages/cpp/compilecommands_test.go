package cpp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
)

// swiftTools fakes the commands that describe a SwiftPM package with Clang
// targets of the given sources.
func swiftTools(t *testing.T, standard string, sdk string, sources ...string) map[string]toolFunc {
	t.Helper()

	describe := map[string]any{
		"cxx_language_standard": standard,
		"targets": []map[string]any{
			{"module_type": "ClangTarget", "path": "Sources/Lib", "sources": sources},
			{"module_type": "SwiftTarget", "path": "Sources/App", "sources": []string{"a.swift"}},
		},
	}
	info := map[string]any{"paths": map[string]string{"runtimeResourcePath": "/tc/usr/lib/swift"}}

	return map[string]toolFunc{
		"swift": func(command execx.Command) execx.Result {
			if slices.Contains(command.Args, "describe") {
				return ok(jsonOf(t, describe))
			}

			return ok(jsonOf(t, info))
		},
		"xcrun": func(execx.Command) execx.Result {
			if sdk == "" {
				return failed("no sdk")
			}

			return ok(sdk + "\n")
		},
	}
}

// swiftPackage returns a package manifest and the Clang sources a test lists,
// which compile-commands keeps only when the repository holds them.
func swiftPackage(sources ...string) map[string]string {
	files := map[string]string{"Package.swift": "let package = 1\n"}
	for _, source := range sources {
		files["Sources/Lib/"+source] = "\n"
	}

	return files
}

// compileEntries runs `compile-commands` and returns the entries it wrote.
func compileEntries(t *testing.T, h *harness) []map[string]any {
	t.Helper()

	code, err := subcommand(t, "compile-commands").Run(t.Context(), h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	if code != 0 {
		t.Fatalf("exit %d: %s", code, h.stderr)
	}

	var entries []map[string]any
	if err = json.Unmarshal([]byte(bedrocktest.Read(t, h.root, "compile_commands.json")), &entries); err != nil {
		t.Fatal(err)
	}

	return entries
}

func TestCompileCommandsCompilesEachSwiftPackageSourceByItsLanguage(t *testing.T) {
	t.Parallel()

	cases := []struct{ source, compiler, flag string }{
		{"a.c", "clang", "-std=c17"},
		{"a.m", "clang", "-fobjc-arc"},
		{"a.mm", "clang++", "-std=c++20"},
		{"a.cpp", "clang++", "-std=c++20"},
	}

	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			t.Parallel()

			// Arrange
			h := newHarness(t, swiftPackage(tc.source), swiftTools(t, "c++20", "/sdk", tc.source))

			// Act
			entries := compileEntries(t, h)

			// Assert
			if len(entries) != 1 {
				t.Fatalf("entries = %v", entries)
			}

			arguments := stringsOf(entries[0]["arguments"])
			if arguments[0] != tc.compiler || !slices.Contains(arguments, tc.flag) {
				t.Errorf("arguments = %v, want %s with %s", arguments, tc.compiler, tc.flag)
			}

			if want := h.root + "/Sources/Lib/" + tc.source; entries[0]["file"] != want {
				t.Errorf("file = %v, want %s", entries[0]["file"], want)
			}

			if entries[0]["directory"] != h.root {
				t.Errorf("directory = %v, want %s", entries[0]["directory"], h.root)
			}
		})
	}
}

func stringsOf(value any) []string {
	items, _ := value.([]any)
	out := make([]string, 0, len(items))

	for _, item := range items {
		text, _ := item.(string)
		out = append(out, text)
	}

	return out
}

func TestCompileCommandsPassesTheSdkToolchainIncludeDeploymentTargetAndTargetIncludes(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, swiftPackage("a.cpp"), swiftTools(t, "c++20", "/sdk", "a.cpp"))

	// Act
	entries := compileEntries(t, h)

	// Assert
	want := []string{
		"clang++", "-std=c++20", "-isysroot", "/sdk", "-isystem", filepath.FromSlash("/tc/usr/include"),
		"-mmacosx-version-min=13.0", "-I", h.root + "/Sources/Lib/include", "-c", h.root + "/Sources/Lib/a.cpp",
	}
	if got := stringsOf(entries[0]["arguments"]); !slices.Equal(got, want) {
		t.Errorf("arguments = %v, want %v", got, want)
	}
}

func TestCompileCommandsOmitsTheSysrootWhenNoneIsKnownAndUsesThePackagesStandard(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, swiftPackage("a.cpp"), swiftTools(t, "gnu++17", "", "a.cpp"))

	// Act
	arguments := stringsOf(compileEntries(t, h)[0]["arguments"])

	// Assert
	if slices.Contains(arguments, "-isysroot") {
		t.Errorf("arguments name a sysroot: %v", arguments)
	}

	if !slices.Contains(arguments, "-std=gnu++17") {
		t.Errorf("arguments miss the package's standard: %v", arguments)
	}
}

func TestCompileCommandsWritesAnEmptyListForAPackageWithNoClangTargets(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{"Package.swift": "let package = 1\n"}, swiftTools(t, "c++20", ""))

	// Act
	entries := compileEntries(t, h)

	// Assert
	if len(entries) != 0 {
		t.Errorf("entries = %v", entries)
	}

	if text := bedrocktest.Read(t, h.root, "compile_commands.json"); text != "[]\n" {
		t.Errorf("file = %q", text)
	}
}

func TestCompileCommandsConfiguresOnlyTheTopCMakeProject(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{
		"port/CMakeLists.txt":     "project(p)\n",
		"port/main.c":             "\n",
		"port/lib/CMakeLists.txt": "add_library(l l.c)\n",
	}, nil)
	h.runner.tools = map[string]toolFunc{"cmake": cmakeThatWritesCommands(t)}

	// Act
	entries := compileEntries(t, h)

	// Assert
	if calls := h.runner.called(
		"cmake",
	); len(calls) != 1 ||
		!strings.Contains(calls[0], "-S "+filepath.Join(h.root, "port")+" ") {
		t.Errorf("cmake calls = %v", calls)
	}

	if len(entries) != 1 || entries[0]["file"] != filepath.Join(h.root, "port", "main.c") {
		t.Errorf("entries = %v", entries)
	}
}

func TestCompileCommandsLeavesOutADependencyFetchedIntoABuildDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{
		"port/CMakeLists.txt": "project(p)\n",
		".cache/bedrock/cmake/port/_deps/x-src/CMakeLists.txt": "project(x)\n",
	}, nil)
	h.runner.tools = map[string]toolFunc{"cmake": cmakeThatWritesCommands(t)}

	// Act
	compileEntries(t, h)

	// Assert
	if calls := h.runner.called("cmake"); len(calls) != 1 {
		t.Errorf("cmake calls = %v", calls)
	}
}

func TestCompileCommandsKeepsOnlyTheSourcesARepositoryListsAndDoesNotMarkGenerated(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{
		"CMakeLists.txt": "project(app)\n",
		".gitattributes": "gen.c linguist-generated\n",
		".gitignore":     "build/\n",
		"main.c":         "\n",
		"gen.c":          "\n",
		"build/dep.c":    "\n",
	}, nil)
	h.runner.tools = map[string]toolFunc{"cmake": func(command execx.Command) execx.Result {
		build := flagValue(command, "-B")
		entries := []map[string]string{
			{"file": "main.c", "directory": h.root},
			{"file": h.root + "/gen.c", "directory": build},
			{"file": h.root + "/build/dep.c", "directory": build},
			{"file": "/usr/include/stdio.h", "directory": build},
		}
		if err := os.MkdirAll(build, 0o750); err != nil {
			return failed(err.Error())
		}

		if err := os.WriteFile(
			filepath.Join(build, "compile_commands.json"),
			[]byte(jsonOf(t, entries)),
			0o600,
		); err != nil {
			return failed(err.Error())
		}

		return ok("")
	}}

	// Act
	entries := compileEntries(t, h)

	// Assert
	if len(entries) != 1 || entries[0]["file"] != "main.c" {
		t.Errorf("entries = %v, want only main.c", entries)
	}
}

func TestCompileCommandsSkipsASubprojectThatFailsToConfigure(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{
		"port/CMakeLists.txt":         "project(p)\n",
		"port/main.c":                 "\n",
		"sdk/template/CMakeLists.txt": "project(t)\n",
	}, nil)
	good := cmakeThatWritesCommands(t)
	h.runner.tools = map[string]toolFunc{"cmake": func(command execx.Command) execx.Result {
		if strings.HasSuffix(flagValue(command, "-S"), "template") {
			return failed("\nno SDK\n")
		}

		return good(command)
	}}

	// Act
	entries := compileEntries(t, h)

	// Assert
	if len(entries) != 1 || entries[0]["file"] != filepath.Join(h.root, "port", "main.c") {
		t.Errorf("entries = %v", entries)
	}

	if want := "skipped sdk/template: cmake configure of sdk/template failed: no SDK"; !strings.Contains(
		h.stderr.String(),
		want,
	) {
		t.Errorf("stderr = %q, want it to hold %q", h.stderr, want)
	}
}

func TestCompileCommandsFailsWhenTheRootProjectFailsToConfigure(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{"CMakeLists.txt": "project(app)\n"}, nil)
	h.runner.tools = map[string]toolFunc{"cmake": func(execx.Command) execx.Result { return failed("bad project") }}

	// Act
	code, err := subcommand(t, "compile-commands").Run(t.Context(), h.ctx, nil)

	// Assert
	if err != nil || code != 1 {
		t.Errorf("code, err = %d, %v; want 1, nil", code, err)
	}

	if want := "cmake configure of . failed: bad project"; !strings.Contains(h.stderr.String(), want) {
		t.Errorf("stderr = %q, want it to hold %q", h.stderr, want)
	}
}
