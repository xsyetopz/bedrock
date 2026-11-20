package cpp_test

import (
	"maps"
	"os/exec"
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const cmakeProject = `cmake_minimum_required(VERSION 3.20)
project(demo C)
include(CTest)
add_library(demo src/demo.c)
install(TARGETS demo)
add_executable(app src/main.c)
target_link_libraries(app demo)
add_library(fixture STATIC tests/fixture.c)
add_executable(demo_test tests/demo_test.c)
target_link_libraries(demo_test demo fixture)
add_test(NAME demo COMMAND demo_test)
`

const mesonProject = `project('demo', 'c')
demo = library('demo', 'src/demo.c', install: true)
executable('app', 'src/main.c', link_with: demo)
fixture = static_library('fixture', 'tests/fixture.c')
demo_test = executable('demo_test', 'tests/demo_test.c', link_with: [demo, fixture])
test('demo', demo_test)
`

const xmakeProject = `target("demo")
    set_kind("static")
    add_files("src/demo.c")
target("app")
    set_kind("binary")
    add_deps("demo")
    add_files("src/main.c")
target("fixture")
    set_kind("static")
    set_default(false)
    add_files("tests/fixture.c")
target("demo_test")
    set_kind("binary")
    set_default(false)
    add_deps("demo", "fixture")
    add_files("tests/demo_test.c")
    add_tests("default")
`

// e2eSources are a library, an app that links it, and a test that links it
// and a fixture.
func e2eSources(expected string) map[string]string {
	return map[string]string{
		"src/demo.c":      "int demo(void) { return 1; }\n",
		"src/main.c":      "int demo(void);\nint main(void) { return demo() == 1 ? 0 : 1; }\n",
		"tests/fixture.c": "int expected(void) { return " + expected + "; }\n",
		"tests/demo_test.c": "int demo(void);\nint expected(void);\n" +
			"int main(void) { return demo() == expected() ? 0 : 1; }\n",
	}
}

type buildSystem struct {
	name string
	tool string
	file string
	text string
}

func buildSystems() []buildSystem {
	return []buildSystem{
		{"CMake", "cmake", "CMakeLists.txt", cmakeProject},
		{"Meson", "meson", "meson.build", mesonProject},
		{"xmake", "xmake", "xmake.lua", xmakeProject},
	}
}

func TestEndToEndAProjectsTestSourcesAreTestFilesAndItsTestsRun(t *testing.T) {
	t.Parallel()

	for _, system := range buildSystems() {
		t.Run(system.name, func(t *testing.T) {
			t.Parallel()

			bedrocktest.E2E(t, "builds a project with the real toolchain")

			if _, err := exec.LookPath(system.tool); err != nil {
				t.Skipf("`%s` is not installed", system.tool)
			}

			// Arrange
			files := e2eSources("1")
			files[system.file] = system.text
			root := bedrocktest.Repo(t, files)
			ctx := bedrocktest.Context(t, root, bedrocktest.Select(t, "cpp", nil))
			h := &harness{runner: nil, ctx: ctx, stderr: nil, root: root}

			// Act
			compiled, compileErr := subcommand(t, "compile-commands").Run(t.Context(), ctx, nil)
			picked := h.testFiles(t, slices.Sorted(maps.Keys(files)))
			tested, testErr := subcommand(t, "test").Run(t.Context(), ctx, nil)

			// Assert
			if compileErr != nil || compiled != 0 {
				t.Errorf("compile-commands = %d, %v; want 0, nil", compiled, compileErr)
			}

			if want := []string{"tests/demo_test.c", "tests/fixture.c"}; !slices.Equal(picked, want) {
				t.Errorf("test files = %v, want %v", picked, want)
			}

			if testErr != nil || tested != 0 {
				t.Errorf("test = %d, %v; want 0, nil", tested, testErr)
			}
		})
	}
}

func TestEndToEndAFailingTestFailsBedrockCppTest(t *testing.T) {
	t.Parallel()

	for _, system := range buildSystems() {
		t.Run(system.name, func(t *testing.T) {
			t.Parallel()

			bedrocktest.E2E(t, "builds a project with the real toolchain")

			if _, err := exec.LookPath(system.tool); err != nil {
				t.Skipf("`%s` is not installed", system.tool)
			}

			// Arrange
			files := e2eSources("2")
			files[system.file] = system.text
			root := bedrocktest.Repo(t, files)
			ctx := bedrocktest.Context(t, root, bedrocktest.Select(t, "cpp", nil))

			// Act
			tested, err := subcommand(t, "test").Run(t.Context(), ctx, nil)

			// Assert
			if err != nil || tested != 1 {
				t.Errorf("test = %d, %v; want 1, nil", tested, err)
			}
		})
	}
}
