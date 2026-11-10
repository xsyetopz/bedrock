import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { $ } from "bun";

const BASE = join(import.meta.dir, "../../../../base/justfile");
const roots: string[] = [];

const scratch = async (project: string): Promise<string> => {
	const root = await mkdtemp(join(tmpdir(), "justfile-"));
	roots.push(root);
	await Bun.write(join(root, "justfile"), Bun.file(BASE));
	await Bun.write(
		join(root, ".just/typescript.just"),
		'test:\n\t@echo "language test"\n',
	);
	if (project !== "") {
		await Bun.write(join(root, "project.just"), project);
	}
	return root;
};

const run = async (root: string, recipe: string): Promise<string> => {
	const output = await $`just ${recipe}`.cwd(root).quiet().text();
	return output.trim();
};

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

describe("each", () => {
	test("runs a project-<task> recipe from project.just after the languages", async () => {
		const root = await scratch('project-test:\n\t@echo "project test"\n');
		expect(await run(root, "test")).toBe("language test\nproject test");
	});

	test("runs only the languages when project.just has no such recipe", async () => {
		const root = await scratch('project-lint:\n\t@echo "project lint"\n');
		expect(await run(root, "test")).toBe("language test");
	});

	test("works without a project.just", async () => {
		const root = await scratch("");
		expect(await run(root, "test")).toBe("language test");
	});
});
