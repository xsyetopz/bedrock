import { describe, expect, test } from "bun:test";

import { go } from "../languages/go/index.ts";
import { rust } from "../languages/rust/index.ts";
import { buildPlan } from "./plan.ts";
import { applySteps } from "./steps.ts";
import { tempRepo } from "./testing.ts";

const selected = [{ language: rust, options: {} }];

describe("buildPlan", () => {
	test("removes replaced tools and records the languages", async () => {
		const root = await tempRepo({ Makefile: "all:\n" });
		const plan = await buildPlan(root, selected, { init: false });
		expect(plan.steps).toContainEqual({ kind: "remove", path: "Makefile" });
		expect(plan.config.languages).toEqual({ rust: {} });
	});

	test("leaves nothing pending once applied", async () => {
		const root = await tempRepo({ Makefile: "all:\n" });
		const first = await buildPlan(root, selected, { init: false });
		await applySteps(root, first.steps);
		const second = await buildPlan(root, selected, { init: false });
		expect(second.steps).toEqual([]);
	});

	test("keeps the project's own lines in a shared file", async () => {
		const root = await tempRepo({ ".gitignore": "secrets/\n" });
		const plan = await buildPlan(root, selected, { init: false });
		await applySteps(root, plan.steps);
		const gitignore = await Bun.file(`${root}/.gitignore`).text();
		expect(gitignore).toStartWith("secrets/\n");
		expect(gitignore).toContain("# >>> bedrock");
	});

	test("removes the shared-file blocks and files of a dropped language", async () => {
		// Arrange
		const root = await tempRepo({});
		const first = await buildPlan(
			root,
			[...selected, { language: go, options: {} }],
			{ init: false },
		);
		await applySteps(root, first.steps);

		// Act
		const plan = await buildPlan(root, selected, {
			init: false,
			dropped: [go],
		});
		await applySteps(root, plan.steps);

		// Assert
		const gitignore = await Bun.file(`${root}/.gitignore`).text();
		expect(gitignore).not.toContain("go-bedrock");
		expect(gitignore).toContain("# >>> rust-bedrock");
		expect(await Bun.file(`${root}/.just/go.just`).exists()).toBe(false);
		expect(await Bun.file(`${root}/.just/rust.just`).exists()).toBe(true);
		expect(plan.notes).toContain(
			"Removed the blocks of go from the shared files: neither .bedrock.json nor a marker file at the root names the language.",
		);
		expect(plan.notes).toContain(
			"Removed .github/workflows/go.yml, .just/go.just: neither .bedrock.json nor a marker file at the root names the language. git still has them.",
		);
	});

	test("notes no removal when a dropped language left nothing", async () => {
		// Arrange
		const root = await tempRepo({});

		// Act
		const plan = await buildPlan(root, selected, {
			init: false,
			dropped: [go],
		});

		// Assert
		expect(plan.notes.join("\n")).not.toContain("Removed");
	});
});
