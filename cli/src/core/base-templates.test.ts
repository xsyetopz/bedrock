import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { $, Glob } from "bun";

import { templatesRoot } from "./templates.ts";

const BASE = join(templatesRoot(), "base/files");
const roots: string[] = [];

const scratch = async (project: string): Promise<string> => {
	const root = await mkdtemp(join(tmpdir(), "base-templates-"));
	roots.push(root);
	await Bun.write(
		join(root, "justfile"),
		Bun.file(join(BASE, "justfile.tmpl")),
	);
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

const workflows = async (): Promise<Record<string, string>> => {
	const entries: Record<string, string> = {};
	for await (const path of new Glob(
		"*/files/.github/workflows/*.yml.tmpl",
	).scan(templatesRoot())) {
		entries[path] = await Bun.file(join(templatesRoot(), path)).text();
	}
	return entries;
};

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

describe("the base justfile", () => {
	test("runs a project-<task> recipe from project.just after the languages", async () => {
		// Arrange
		const root = await scratch('project-test:\n\t@echo "project test"\n');

		// Act
		const output = await run(root, "test");

		// Assert
		expect(output).toBe("language test\nproject test");
	});

	test("runs only the languages when project.just has no such recipe", async () => {
		// Arrange
		const root = await scratch('project-lint:\n\t@echo "project lint"\n');

		// Act
		const output = await run(root, "test");

		// Assert
		expect(output).toBe("language test");
	});

	test("works without a project.just", async () => {
		// Arrange
		const root = await scratch("");

		// Act
		const output = await run(root, "test");

		// Assert
		expect(output).toBe("language test");
	});
});

describe("the base lefthook.yml", () => {
	test("runs only recipes the base justfile defines", async () => {
		// Arrange
		const root = await scratch("");
		const config = await Bun.file(join(BASE, "lefthook.yml.tmpl")).text();
		const recipeList = await run(root, "--summary");
		const summary = recipeList.split(" ");

		// Act
		const recipes = [...config.matchAll(/run: just (\S+)/gu)].map(
			(match) => match[1],
		);
		const missing = recipes.filter(
			(recipe) => recipe === undefined || !summary.includes(recipe),
		);

		// Assert
		expect(recipes).toEqual(["pre-commit", "bedrock", "pre-push"]);
		expect(missing).toEqual([]);
	});
});

describe("the workflow templates", () => {
	test("run the push checks on main and master", async () => {
		// Arrange
		const files = await workflows();

		// Act
		const missing = Object.entries(files)
			.filter(([, text]) => !text.includes("branches: [main, master]"))
			.map(([path]) => path);

		// Assert
		expect(Object.keys(files).length).toBeGreaterThanOrEqual(5);
		expect(missing).toEqual([]);
	});

	test("pin taiki-e/install-action at one release", async () => {
		// Arrange
		const files = await workflows();

		// Act
		const pins = new Set(
			Object.values(files).flatMap((text) =>
				[...text.matchAll(/taiki-e\/install-action@\S+ # \S+/gu)].map(
					(match) => match[0],
				),
			),
		);

		// Assert
		expect([...pins]).toEqual([
			"taiki-e/install-action@cfdb7ed4ccc7c7dce37759eae89dc5674b469b39 # v2.90.3",
		]);
	});
});
