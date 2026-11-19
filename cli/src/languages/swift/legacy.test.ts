import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { legacyPlan } from "./legacy.ts";

const OLD_TOOLS =
	'let package = Package(name: "BuildTools", dependencies: [.package(url: "https://github.com/xsyetopz/bedrock", revision: "swift-bedrock-v0.1.0")])\n';

describe("legacyPlan", () => {
	test("removes BuildTools that depends on swift-bedrock", async () => {
		const root = await tempRepo({ "BuildTools/Package.swift": OLD_TOOLS });

		const plan = await legacyPlan(root);

		expect(plan.steps).toContainEqual({ kind: "remove", path: "BuildTools" });
	});

	test("keeps a BuildTools package that is the project's own", async () => {
		const root = await tempRepo({
			"BuildTools/Package.swift":
				'let package = Package(name: "BuildTools", dependencies: [.package(url: "https://example.com/other", from: "1.0.0")])\n'
		});

		const plan = await legacyPlan(root);

		expect(plan.steps).toEqual([]);
		expect(plan.notes).toEqual([]);
	});

	test.each([".swiftformat", ".swiftlint.yaml", "Mintfile"])(
		"removes %s",
		async (path) => {
			const root = await tempRepo({ [path]: "x\n" });

			const plan = await legacyPlan(root);

			expect(plan.steps).toEqual([{ kind: "remove", path }]);
		}
	);

	test.each([
		[
			"project.just",
			"lint:\n\tswift run --package-path BuildTools swiftlint\n"
		],
		[".gitignore", "BuildTools/.build\n"],
		[".just/ci.just", "x:\n\tcd BuildTools\n"],
		[".github/workflows/ci.yml", "run: swift run --package-path BuildTools x\n"]
	])("notes the BuildTools reference in %s", async (path, text) => {
		const root = await tempRepo({
			"BuildTools/Package.swift": OLD_TOOLS,
			[path]: text
		});

		const plan = await legacyPlan(root);

		expect(plan.notes).toHaveLength(1);
		expect(plan.notes[0]?.startsWith(`${path}: `)).toBe(true);
	});

	test("skips the managed swift files and blocks bedrock rewrites", async () => {
		const root = await tempRepo({
			"BuildTools/Package.swift": OLD_TOOLS,
			".just/swift.just": "BuildTools\n",
			".github/workflows/swift.yml": "BuildTools\n",
			".gitignore":
				"# >>> swift-bedrock\nBuildTools/.build\n# <<< swift-bedrock\n"
		});

		const plan = await legacyPlan(root);

		expect(plan.notes).toEqual([]);
	});

	test("does nothing in a clean repository", async () => {
		const root = await tempRepo({ "Package.swift": "" });

		expect(await legacyPlan(root)).toEqual({
			steps: [],
			notes: [],
			projectFiles: []
		});
	});
});
