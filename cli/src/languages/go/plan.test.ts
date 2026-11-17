import { describe, expect, test } from "bun:test";

import { buildPlan } from "../../core/plan.ts";
import { tempRepo } from "../../core/testing.ts";
import { written } from "../swift/harness.ts";
import { adoptGo } from "./harness.ts";
import { go } from "./index.ts";

const selected = [{ language: go, options: {} }];

describe("adopt", () => {
	test("writes the recipes, workflow, and starter config for the module", async () => {
		// Arrange
		const root = await tempRepo({
			"go.mod": "module github.com/acme/app\n\ngo 1.27\n",
		});

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).toContain(".just/go.just");
		expect(paths).toContain(".github/workflows/go.yml");
		expect(paths).not.toContain("go.mod");
		expect(written(plan.steps, ".golangci.yml")).toContain(
			"- github.com/acme/app",
		);
		expect(written(plan.steps, ".gitignore")).toContain("# >>> go-bedrock");
		expect(written(plan.steps, "justfile")).toContain(
			'mod? go ".just/go.just"',
		);
	});

	test("bounds memory and parallelism and pins golangci-lint", async () => {
		// Arrange
		const root = await tempRepo({ "go.mod": "module m\n\ngo 1.27\n" });

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		const recipes = written(plan.steps, ".just/go.just");
		expect(recipes).toContain(
			'export GOMEMLIMIT := env("BEDROCK_GO_MEMLIMIT", "2GiB")',
		);
		expect(recipes).toContain('" -p=" + jobs');
		expect(recipes).toContain("golangci-lint@v2.15.0");
		expect(written(plan.steps, ".golangci.yml")).toContain("concurrency: 4");
	});

	test("a second adopt plans no steps", async () => {
		// Arrange
		const root = await tempRepo({ "go.mod": "module m\n\ngo 1.27\n" });
		await adoptGo(root);

		// Act
		const again = await buildPlan(root, selected, { init: false });

		// Assert
		expect(again.steps).toEqual([]);
	});

	test("keeps a .golangci.yml the project changed", async () => {
		// Arrange
		const root = await tempRepo({ "go.mod": "module m\n\ngo 1.27\n" });
		await adoptGo(root);
		await Bun.write(`${root}/.golangci.yml`, 'version: "2"\n');

		// Act
		const again = await buildPlan(root, selected, { init: false });

		// Assert
		expect(again.steps).toEqual([]);
	});
});

describe("init", () => {
	test.each([
		["command", "internal/greeting/greeting.go"],
		["library", ".golangci.yml"],
	])("%s starts a module with its starter files", async (kind, starter) => {
		// Arrange
		const root = await tempRepo();

		// Act
		const plan = await buildPlan(root, [{ language: go, options: { kind } }], {
			init: true,
		});

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).toContain("go.mod");
		expect(paths).toContain(starter);
		expect(plan.notes.join("\n")).toContain("placeholder module example.com/");
	});

	test("takes the module path from .bedrock.json options", async () => {
		// Arrange
		const root = await tempRepo();

		// Act
		const plan = await buildPlan(
			root,
			[
				{
					language: go,
					options: { kind: "library", module: "github.com/acme/lib" },
				},
			],
			{ init: true },
		);

		// Assert
		expect(written(plan.steps, "go.mod")).toContain(
			"module github.com/acme/lib",
		);
		expect(plan.notes).toEqual([]);
	});
});
