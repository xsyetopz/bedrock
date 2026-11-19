import { expect, test } from "bun:test";

import { buildPlan } from "../../core/plan.ts";
import { tempRepo } from "../../core/testing.ts";
import { contextIn, written } from "../swift/harness.ts";
import { swift } from "../swift/index.ts";
import { cpp } from "./index.ts";

const selected = [
	{ language: swift, options: {} },
	{ language: cpp, options: {} },
];

test("adopting cpp writes the C++ files", async () => {
	const root = await tempRepo({ "Package.swift": "let package = 1\n" });

	const plan = await buildPlan(root, selected, { init: false });

	const paths = plan.steps.map((step) => step.path);
	expect(paths).toContain(".clang-format");
	expect(paths).toContain(".clang-tidy");
	expect(paths).toContain(".just/cpp.just");
});

test("adopting cpp alone writes its files and workflow without Swift's", async () => {
	// Arrange
	const root = await tempRepo({ "CMakeLists.txt": "project(app C)\n" });

	// Act
	const plan = await buildPlan(root, [{ language: cpp, options: {} }], {
		init: false,
	});

	// Assert
	const paths = plan.steps.map((step) => step.path);
	expect(paths).toContain(".just/cpp.just");
	expect(paths).toContain(".github/workflows/cpp.yml");
	expect(paths).not.toContain("Package.swift");
	expect(paths).not.toContain(".github/workflows/swift.yml");
});

test.each(["_clang-format", "_clang-tidy"])(
	"removes the old %s",
	async (path) => {
		const root = await tempRepo({
			"Package.swift": "let package = 1\n",
			[path]: "x\n",
		});

		const plan = await buildPlan(root, selected, { init: false });

		expect(plan.steps).toContainEqual({ kind: "remove", path });
	},
);

test("the guard recipe runs bedrock guard cpp", async () => {
	const root = await tempRepo({ "Package.swift": "let package = 1\n" });

	const plan = await buildPlan(root, selected, { init: false });

	expect(written(plan.steps, ".just/cpp.just")).toContain(
		"{{ just_executable() }} bedrock guard cpp",
	);
});

test("the guard has no findings of its own", async () => {
	const root = await tempRepo();

	expect(await cpp.guard(contextIn(root), false)).toEqual([]);
});
