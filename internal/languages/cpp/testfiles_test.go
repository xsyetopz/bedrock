package cpp_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
)

// demoSources are a library, an app that links it, and a test that links it
// and a fixture, which every build system's test describes the same way.
func demoSources() map[string]string {
	return map[string]string{
		"src/demo.c":        "int demo(void) { return 1; }\n",
		"src/main.c":        "int main(void) { return 0; }\n",
		"tests/fixture.c":   "int expected(void) { return 1; }\n",
		"tests/demo_test.c": "int main(void) { return 0; }\n",
	}
}

func sourceNames(sources map[string]string) []string {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

func TestTestFilesPicksTheFilesOfASwiftPackagesTestsDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"Package.swift":             "// swift-tools-version: 6.4\n",
		"Sources/KitCxx/Kit.cpp":    "int kit() { return 0; }\n",
		"Tests/KitTests/Helper.cpp": "int helper() { return 0; }\n",
		"vendor/Tests/Other.cpp":    "int other() { return 0; }\n",
	}
	h := newHarness(t, files, nil)

	// Act
	picked := h.testFiles(t, []string{"Sources/KitCxx/Kit.cpp", "Tests/KitTests/Helper.cpp", "vendor/Tests/Other.cpp"})

	// Assert
	if want := []string{"Tests/KitTests/Helper.cpp"}; !slices.Equal(picked, want) {
		t.Errorf("picked = %v, want %v", picked, want)
	}
}

func TestTestFilesFollowsACMakeBuildsTests(t *testing.T) {
	t.Parallel()

	// Arrange
	files := demoSources()
	files["CMakeLists.txt"] = "project(demo C)\n"
	h := newHarness(t, files, nil)
	writeCMakeReply(t, h.root)

	// Act
	picked := h.testFiles(t, sourceNames(demoSources()))

	// Assert
	if want := []string{"tests/demo_test.c", "tests/fixture.c"}; !slices.Equal(picked, want) {
		t.Errorf("picked = %v, want %v", picked, want)
	}
}

// writeCMakeReply writes what configuring a CMake demo project leaves: the
// File API reply for its targets and the CTest file naming the test program.
func writeCMakeReply(t *testing.T, root string) {
	t.Helper()

	// Tools print the resolved root, as /private/var for /var.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	build := ".cache/bedrock/cmake/+/"
	reply := build + ".cmake/api/v1/reply/"
	type object = map[string]any

	targets := map[string]object{
		"demo": {
			"id": "demo", "sources": []object{{"path": "src/demo.c"}}, "install": object{},
		},
		"app": {
			"id": "app", "sources": []object{{"path": "src/main.c"}},
			"dependencies": []object{{"id": "demo"}}, "artifacts": []object{{"path": "app"}},
		},
		"fixture": {"id": "fixture", "sources": []object{{"path": "tests/fixture.c"}}},
		"demo_test": {
			"id": "demo_test", "sources": []object{{"path": "tests/demo_test.c"}},
			"dependencies": []object{{"id": "demo"}, {"id": "fixture"}}, "artifacts": []object{{"path": "demo_test"}},
		},
	}

	var entries []object

	for name, target := range targets {
		bedrocktest.Write(t, root, reply+"target-"+name+".json", jsonOf(t, target))
		entries = append(entries, object{"jsonFile": "target-" + name + ".json"})
	}

	bedrocktest.Write(t, root, reply+"index-1.json",
		jsonOf(t, object{"reply": object{"codemodel-v2": object{"jsonFile": "codemodel.json"}}}))
	bedrocktest.Write(t, root, reply+"codemodel.json", jsonOf(t, object{
		"paths":          object{"source": resolved},
		"configurations": []object{{"targets": entries}},
	}))
	bedrocktest.Write(t, root, build+"CTestTestfile.cmake",
		"add_test([=[demo]=] \""+filepath.Join(resolved, build, "demo_test")+"\")\n")
}

func TestTestFilesFollowsAMesonBuildsTests(t *testing.T) {
	t.Parallel()

	// Arrange
	files := demoSources()
	files["meson.build"] = "project('demo', 'c')\n"
	h := newHarness(t, files, nil)
	build := filepath.Join(h.root, ".cache/bedrock/meson/+")
	bedrocktest.Write(t, h.root, ".cache/bedrock/meson/+/meson-private/coredata.dat", "")

	file := func(name string) string { return filepath.Join(h.root, name) }
	linked := func(parameters ...string) []map[string]any {
		return []map[string]any{{"linker": []string{"cc"}, "parameters": parameters}}
	}
	targets := []map[string]any{
		{
			"id": "demo", "filename": []string{filepath.Join(build, "libdemo.a")}, "installed": true,
			"target_sources": []map[string]any{{"sources": []string{file("src/demo.c")}}},
		},
		{
			"id": "app", "filename": []string{filepath.Join(build, "app")}, "installed": false,
			"target_sources": append([]map[string]any{{"sources": []string{file("src/main.c")}}},
				linked(filepath.Join(build, "libdemo.a"))...),
		},
		{
			"id": "fixture", "filename": []string{filepath.Join(build, "libfixture.a")}, "installed": false,
			"target_sources": []map[string]any{{"sources": []string{file("tests/fixture.c")}}},
		},
		{
			"id": "demo_test", "filename": []string{filepath.Join(build, "demo_test")}, "installed": false,
			"target_sources": append([]map[string]any{{"sources": []string{file("tests/demo_test.c")}}},
				linked(filepath.Join(build, "libdemo.a"), filepath.Join(build, "libfixture.a"))...),
		},
	}
	tests := []map[string]any{{"cmd": []string{filepath.Join(build, "demo_test")}, "depends": []string{}}}

	h.runner.tools = map[string]toolFunc{"meson": func(command execx.Command) execx.Result {
		if slices.Contains(command.Args, "--tests") {
			return ok(jsonOf(t, tests))
		}

		return ok(jsonOf(t, targets))
	}}

	// Act
	picked := h.testFiles(t, sourceNames(demoSources()))

	// Assert
	if want := []string{"tests/demo_test.c", "tests/fixture.c"}; !slices.Equal(picked, want) {
		t.Errorf("picked = %v, want %v", picked, want)
	}
}

func TestTestFilesFollowsAnXmakeProjectsTests(t *testing.T) {
	t.Parallel()

	// Arrange
	files := demoSources()
	files["xmake.lua"] = "target(\"demo\")\n"
	h := newHarness(t, files, nil)
	absolute := func(name string) string { return filepath.Join(h.root, name) }
	targets := []map[string]any{
		{
			"id":        "demo",
			"sources":   []string{absolute("src/demo.c")},
			"deps":      map[string]any{},
			"test":      false,
			"installed": true,
		},
		{
			"id": "app", "sources": []string{absolute("src/main.c")}, "deps": []string{"demo"},
			"test": false, "installed": true,
		},
		{
			"id": "fixture", "sources": []string{absolute("tests/fixture.c")}, "deps": []string{},
			"test": false, "installed": false,
		},
		{
			"id": "demo_test", "sources": []string{absolute("tests/demo_test.c")}, "deps": []string{"demo", "fixture"},
			"test": true, "installed": false,
		},
	}
	// xmake prints warnings, such as deprecations, around the JSON line.
	h.runner.tools = map[string]toolFunc{"xmake": func(execx.Command) execx.Result {
		return ok("warning: deprecated\n" + jsonOf(t, targets) + "\nnote: done\n")
	}}

	// Act
	picked := h.testFiles(t, sourceNames(demoSources()))

	// Assert
	if want := []string{"tests/demo_test.c", "tests/fixture.c"}; !slices.Equal(picked, want) {
		t.Errorf("picked = %v, want %v", picked, want)
	}
}

func TestTestFilesKeepsASourceAProductTargetAlsoCompiles(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"xmake.lua":        "target(\"app\")\n",
		"src/main.c":       "",
		"src/shared.c":     "",
		"tests/app_test.c": "",
	}
	h := newHarness(t, files, nil)
	absolute := func(name string) string { return filepath.Join(h.root, name) }
	targets := []map[string]any{
		{
			"id": "app", "sources": []string{absolute("src/main.c"), absolute("src/shared.c")},
			"deps": []string{}, "test": false, "installed": true,
		},
		{
			"id": "app_test", "sources": []string{absolute("tests/app_test.c"), absolute("src/shared.c")},
			"deps": []string{}, "test": true, "installed": false,
		},
	}
	h.runner.tools = map[string]toolFunc{"xmake": func(execx.Command) execx.Result { return ok(jsonOf(t, targets)) }}

	// Act
	picked := h.testFiles(t, []string{"src/main.c", "src/shared.c", "tests/app_test.c"})

	// Assert
	if want := []string{"tests/app_test.c"}; !slices.Equal(picked, want) {
		t.Errorf("picked = %v, want %v", picked, want)
	}
}

func TestTestFilesTreatsAnUnloadableXmakeProjectAsHavingNoTargets(t *testing.T) {
	t.Parallel()

	// Arrange
	h := newHarness(t, map[string]string{"xmake.lua": "", "tests/a_test.c": ""}, nil)
	h.runner.tools = map[string]toolFunc{"xmake": func(execx.Command) execx.Result { return failed("cannot load") }}

	// Act
	picked := h.testFiles(t, []string{"tests/a_test.c"})

	// Assert
	if len(picked) != 0 {
		t.Errorf("picked = %v, want none", picked)
	}
}
