import { expect, test } from "bun:test";

import type { Context } from "../../core/language.ts";
import { contextFor } from "../../core/plan.ts";
import { tempRepo } from "../../core/testing.ts";
import { compileCommands } from "./compile-commands.ts";
import { cpp } from "./index.ts";
import { cppTestFiles } from "./test-files.ts";
import { cppTest } from "./test.ts";

/** A library, an app that links it, and a test that links it and a fixture. */
const SOURCES = {
	"src/demo.c": "int demo(void) { return 1; }\n",
	"src/main.c":
		"int demo(void);\nint main(void) { return demo() == 1 ? 0 : 1; }\n",
	"tests/fixture.c": "int expected(void) { return 1; }\n",
	"tests/demo_test.c":
		"int demo(void);\nint expected(void);\nint main(void) { return demo() == expected() ? 0 : 1; }\n",
};

const CMAKE = `cmake_minimum_required(VERSION 3.20)
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
`;

const MESON = `project('demo', 'c')
demo = library('demo', 'src/demo.c', install: true)
executable('app', 'src/main.c', link_with: demo)
fixture = static_library('fixture', 'tests/fixture.c')
demo_test = executable('demo_test', 'tests/demo_test.c', link_with: [demo, fixture])
test('demo', demo_test)
`;

const XMAKE = `target("demo")
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
`;

const contextIn = (root: string): Context =>
	contextFor(root, [{ language: cpp, options: {} }], cpp, {}, false);

const BUILDS = [
	{ system: "CMake", tool: "cmake", file: "CMakeLists.txt", text: CMAKE },
	{ system: "Meson", tool: "meson", file: "meson.build", text: MESON },
	{ system: "xmake", tool: "xmake", file: "xmake.lua", text: XMAKE },
];

for (const { system, tool, file, text } of BUILDS) {
	const absent = Bun.which(tool) === null;

	test.skipIf(absent)(
		`a ${system} project's test sources are test files and its tests run`,
		async () => {
			// Arrange
			const root = await tempRepo({ [file]: text, ...SOURCES });
			const context = contextIn(root);

			// Act
			const commands = await compileCommands.run(context, []);
			const picked = await cppTestFiles(root, Object.keys(SOURCES));
			const tested = await cppTest.run(context, []);

			// Assert
			expect(commands).toBe(0);
			expect([...picked].toSorted()).toEqual([
				"tests/demo_test.c",
				"tests/fixture.c",
			]);
			expect(tested).toBe(0);
		},
		300_000,
	);

	test.skipIf(absent)(
		`a failing ${system} test fails bedrock cpp test`,
		async () => {
			// Arrange
			const root = await tempRepo({
				[file]: text,
				...SOURCES,
				"tests/fixture.c": "int expected(void) { return 2; }\n",
			});

			// Act
			const tested = await cppTest.run(contextIn(root), []);

			// Assert
			expect(tested).toBe(1);
		},
		300_000,
	);
}
