import { describe, expect, test } from "bun:test";

import { go } from "../languages/go/index.ts";
import { rust } from "../languages/rust/index.ts";
import { typescript } from "../languages/typescript/index.ts";
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

	test("keeps the repo's own .editorconfig sections, then plans nothing", async () => {
		// Arrange
		const own = "\n[gen/**]\nbedrock_file_code_lines_warning = unset\n";
		const root = await tempRepo({ ".editorconfig": `root = true\n${own}` });

		// Act
		const first = await buildPlan(root, selected, { init: false });
		await applySteps(root, first.steps);
		const second = await buildPlan(root, selected, { init: false });

		// Assert
		const text = await Bun.file(`${root}/.editorconfig`).text();
		expect(text).toContain(own);
		expect(text).toContain("bedrock_languages = rust\n");
		expect(second.steps).toEqual([]);
	});

	test("keeps the project's own lines in a shared file", async () => {
		const root = await tempRepo({ ".gitignore": "secrets/\n" });
		const plan = await buildPlan(root, selected, { init: false });
		await applySteps(root, plan.steps);
		const gitignore = await Bun.file(`${root}/.gitignore`).text();
		expect(gitignore).toStartWith("secrets/\n");
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

	test("keeps out of the removal note a dropped language's file the base still writes", async () => {
		// Arrange
		const root = await tempRepo({ ".markdownlint-cli2.jsonc": "{}\n" });

		// Act
		const plan = await buildPlan(root, selected, {
			init: false,
			dropped: [typescript],
		});

		// Assert
		expect(plan.steps).not.toContainEqual({
			kind: "remove",
			path: ".markdownlint-cli2.jsonc",
		});
		expect(plan.notes.join("\n")).not.toContain(".markdownlint-cli2.jsonc");
	});
});
