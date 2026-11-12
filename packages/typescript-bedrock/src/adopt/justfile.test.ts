import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { $ } from "bun";

const BASE = join(import.meta.dir, "../../../../base/justfile");
const MODULE = join(import.meta.dir, "../../files/.just/typescript.just");
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

const typecheck = async (project: string): Promise<string> => {
	const root = await scratch(project);
	await Bun.write(join(root, ".just/typescript.just"), Bun.file(MODULE));
	await Bun.write(
		join(root, "package.json"),
		'{"scripts":{"check":"echo bun check"}}',
	);
	return run(root, "typescript::typecheck");
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

describe("typescript::typecheck", () => {
	test("runs project-pre-typecheck before bun check", async () => {
		const output = await typecheck(
			'project-pre-typecheck:\n\t@echo "pre typecheck"\n',
		);
		expect(output).toContain("pre typecheck\n");
		expect(output.indexOf("pre typecheck")).toBeLessThan(
			output.indexOf("bun check"),
		);
	});

	test("runs only bun check without such a recipe", async () => {
		const output = await typecheck('project-lint:\n\t@echo "project lint"\n');
		expect(output).not.toContain("pre typecheck");
		expect(output).toContain("bun check");
	});
});
