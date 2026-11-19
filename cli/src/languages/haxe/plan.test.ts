import { describe, expect, test } from "bun:test";

import { buildPlan } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { tempRepo } from "../../core/testing.ts";
import { typescript } from "../typescript/index.ts";
import { haxe } from "./index.ts";

const selected = [{ language: haxe, options: {} }];

describe("adopt", () => {
	test("writes the recipes, workflow, and starter checkstyle config", async () => {
		// Arrange
		const root = await tempRepo({ "build.hxml": "-cp src\n-js out.js\n" });

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).toContain(".just/haxe.just");
		expect(paths).toContain(".github/workflows/haxe.yml");
		expect(paths).toContain("checkstyle.json");
	});

	test("a second adopt plans no steps", async () => {
		// Arrange
		const root = await tempRepo({ "build.hxml": "-cp src\n-js out.js\n" });
		const first = await buildPlan(root, selected, { init: false });
		await applySteps(root, first.steps);

		// Act
		const again = await buildPlan(root, selected, { init: false });

		// Assert
		expect(again.steps).toEqual([]);
	});

	test("notes a hand-written workflow it replaces, and not its own", async () => {
		// Arrange
		const root = await tempRepo({
			"build.hxml": "-cp src\n-js out.js\n",
			".github/workflows/haxe.yml": "name: Haxe\n",
		});

		// Act
		const first = await buildPlan(root, selected, { init: false });
		await applySteps(root, first.steps);
		const again = await buildPlan(root, selected, { init: false });

		// Assert
		expect(first.notes.join("\n")).toContain(
			".github/workflows/haxe.yml: replaced with bedrock's",
		);
		expect(again.notes).toEqual([]);
	});

	test("notes a test hxml file that names no way to run its tests", async () => {
		// Arrange
		const root = await tempRepo({
			"test.hxml": "-cp test\n--main TestMain\n-lib utest\n",
			"test-interp.hxml": "-cp test\n--main TestMain\n--interp\n",
		});

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		expect(plan.notes).toEqual([
			"test.hxml: names no --interp, --run, or --cmd, so just haxe::test does not run it; a project-test recipe in project.just runs it on its target.",
		]);
	});

	test("adopts Haxe beside TypeScript in one repo", async () => {
		// Arrange
		const root = await tempRepo({
			"package.json": '{ "name": "app" }\n',
			"build.hxml": "-cp src\n-js out.js\n",
		});
		const both = [
			{ language: typescript, options: {} },
			{ language: haxe, options: {} },
		];

		// Act
		const plan = await buildPlan(root, both, { init: false });

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).toContain(".just/haxe.just");
		expect(paths).toContain(".just/typescript.just");
	});
});
