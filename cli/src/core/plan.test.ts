import { describe, expect, test } from "bun:test";

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
});
