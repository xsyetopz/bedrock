import { expect, test } from "bun:test";

import { $ } from "bun";

import type { Context } from "../../core/language.ts";
import { buildPlan, contextFor } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { tempRepo } from "../../core/testing.ts";
import { haxeTest, haxeTypecheck } from "./builds.ts";
import { haxe } from "./index.ts";
import { haxeFormat, haxeLint } from "./style.ts";

const hasHaxe = Bun.which("haxe") !== null;

/** Whether haxelib resolves the formatter and checkstyle, as `just haxe::setup` installs them. */
const styleTools = hasHaxe
	? await $`haxelib path checkstyle formatter`.nothrow().quiet()
	: undefined;
const hasStyleTools = styleTools?.exitCode === 0;

/** A class, a build that names its target, and a test build that runs on the interpreter. */
const SOURCES = {
	"src/demo/Greeting.hx":
		'package demo;\n\nclass Greeting {\n\tpublic static function of(name:String):String {\n\t\treturn "Hello, " + name;\n\t}\n}\n',
	"src/Main.hx":
		'import demo.Greeting;\n\nclass Main {\n\tstatic function main() {\n\t\ttrace(Greeting.of("world"));\n\t}\n}\n',
	"test/TestMain.hx":
		'import demo.Greeting;\n\nclass TestMain {\n\tstatic function main() {\n\t\tSys.exit(Greeting.of("you") == "Hello, you" ? 0 : 1);\n\t}\n}\n',
	"build.hxml": "-cp src\n--main Main\n--js bin/main.js\n",
	"test.hxml": "-cp src\n-cp test\n--main TestMain\n--interp\n",
};

const contextIn = (root: string): Context =>
	contextFor(root, [{ language: haxe, options: {} }], haxe, {}, false);

test.skipIf(!hasHaxe)(
	"an adopted Haxe project type-checks and runs its tests",
	async () => {
		// Arrange
		const root = await tempRepo(SOURCES);
		const plan = await buildPlan(root, [{ language: haxe, options: {} }], {
			init: false,
		});
		await applySteps(root, plan.steps);

		// Act
		const typechecked = await haxeTypecheck.run(contextIn(root), []);
		const tested = await haxeTest.run(contextIn(root), []);

		// Assert
		expect(typechecked).toBe(0);
		expect(tested).toBe(0);
		expect(await Bun.file(`${root}/bin/main.js`).exists()).toBe(false);
	},
	120_000,
);

test.skipIf(!hasHaxe)(
	"a failing test or type error fails its command",
	async () => {
		// Arrange
		const root = await tempRepo({
			...SOURCES,
			"src/demo/Greeting.hx":
				'package demo;\n\nclass Greeting {\n\tpublic static function of(name:String):String {\n\t\treturn "Hi, " + name;\n\t}\n}\n',
			"src/Main.hx":
				'class Main {\n\tstatic function main() {\n\t\tvar n:Int = "text";\n\t}\n}\n',
		});

		// Act
		const typechecked = await haxeTypecheck.run(contextIn(root), []);
		const tested = await haxeTest.run(contextIn(root), []);

		// Assert
		expect(typechecked).toBe(1);
		expect(tested).toBe(1);
	},
	120_000,
);

test.skipIf(!hasStyleTools)(
	"an adopted Haxe project passes format and lint, and a finding fails lint",
	async () => {
		// Arrange
		const root = await tempRepo(SOURCES);
		const plan = await buildPlan(root, [{ language: haxe, options: {} }], {
			init: false,
		});
		await applySteps(root, plan.steps);

		// Act
		const formatted = await haxeFormat.run(contextIn(root), ["--check"]);
		const linted = await haxeLint.run(contextIn(root), []);
		await Bun.write(
			`${root}/src/Unused.hx`,
			"import demo.Greeting;\n\nclass Unused {}\n",
		);
		const flagged = await haxeLint.run(contextIn(root), []);

		// Assert
		expect(formatted).toBe(0);
		expect(linted).toBe(0);
		expect(flagged).toBe(1);
	},
	120_000,
);
