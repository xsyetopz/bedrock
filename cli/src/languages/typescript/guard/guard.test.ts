import { describe, expect, test } from "bun:test";
import { join } from "node:path";

import { $ } from "bun";

import { tempRepo } from "../../../core/testing.ts";
import { adoptTypescript, commitAll, contextIn } from "../harness.ts";
import { guardTypescript } from "./index.ts";
import { findUnsortedCatalogs } from "./package-managers.ts";

// Built at runtime so the guard does not flag this file.
const SUPPRESSION = `// ${["eslint", "disable"].join("-")}`;

const brokenRepo = {
	"package.json": JSON.stringify({
		scripts: { build: "npx tsc" },
		workspaces: { catalog: { b: "1", a: "1" } },
	}),
	"src/index.ts": `${SUPPRESSION}\n`,
	"node_modules/dep/index.ts": `${SUPPRESSION}\n`,
	"node_modules/typescript/package.json": JSON.stringify({ version: "0.0.0" }),
	"yarn.lock": "",
};

const problemsIn = (root: string): Promise<string[]> =>
	guardTypescript(contextIn(root));

describe("guardTypescript", () => {
	test("passes a clean adopted repo", async () => {
		const root = await tempRepo({
			"package.json": JSON.stringify({ scripts: { test: "bun test" } }),
			"src/index.ts": "export const one = 1;\n",
		});
		await adoptTypescript(root);
		await commitAll(root);
		expect(await problemsIn(root)).toEqual([]);
	}, 30_000);

	test("reports each problem, skipping node_modules", async () => {
		const root = await tempRepo(brokenRepo);
		const printed =
			await $`bun -e "console.log(process.versions.typescript)"`.text();
		const builtIn = printed.trim();
		expect(await problemsIn(root)).toEqual([
			"src/index.ts:1: suppression comment",
			"yarn.lock: not used in a Bun-only repo",
			'package.json: script "build" calls another package manager',
			`typescript: 0.0.0 is installed but \`bun check\` runs ${builtIn}; set the catalog to ${builtIn}`,
			"package.json: sort workspaces.catalog by package name",
		]);
	});

	test("skips the paths .oxlintrc.json ignores", async () => {
		const root = await tempRepo({
			"package.json": JSON.stringify({ scripts: { test: "bun test" } }),
			".oxlintrc.json": JSON.stringify({ ignorePatterns: ["openspec/**"] }),
			"openspec/old/a.ts": `${SUPPRESSION}\n`,
			"other/a.ts": `${SUPPRESSION}\n`,
		});
		expect(await problemsIn(root)).toEqual([
			"other/a.ts:1: suppression comment",
		]);
	});

	test("skips files git no longer tracks", async () => {
		const root = await tempRepo(brokenRepo);
		await Bun.write(join(root, "src/index.ts"), "export const one = 1;\n");
		expect(await problemsIn(root)).not.toContain(
			"src/index.ts:1: suppression comment",
		);
	});
});

describe("the package API check", () => {
	test("needs JSDoc only on what the entry points expose", async () => {
		const root = await tempRepo({
			"package.json": JSON.stringify({ name: "app" }),
			"src/main.ts": "export const run = (): void => {};\n",
			"lib/package.json": JSON.stringify({
				exports: { ".": "./src/index.ts", "./package.json": "./package.json" },
			}),
			"lib/src/index.ts":
				'/** Documented. */\nexport const one = 1;\nexport * from "./shapes.ts";\nexport { two } from "./two";\n',
			"lib/src/shapes.ts":
				"export interface Shape {\n\treadonly size: number;\n}\n",
			"lib/src/two.ts": "export const two = 2;\nconst hidden = 3;\n",
			"lib/src/internal.ts": "export const internal = 4;\n",
		});
		await adoptTypescript(root);
		await Bun.$`bun install`.cwd(root).quiet();
		const problems = await problemsIn(root);
		expect(problems.filter((line) => line.includes("JSDoc"))).toEqual([
			"lib/src/shapes.ts:1: add a JSDoc comment; the package exports this",
			"lib/src/two.ts:1: add a JSDoc comment; the package exports this",
		]);
	}, 120_000);
});

describe("findUnsortedCatalogs", () => {
	test("passes sorted catalogs and manifests without one", () => {
		expect(findUnsortedCatalogs({})).toEqual([]);
		expect(
			findUnsortedCatalogs({
				workspaces: {
					catalog: { "@types/bun": "1", oxlint: "1" },
					catalogs: { old: { a: "1", b: "1" } },
				},
			}),
		).toEqual([]);
	});

	test("names each unsorted catalog", () => {
		expect(
			findUnsortedCatalogs({
				workspaces: {
					catalog: { oxlint: "1", "@types/bun": "1" },
					catalogs: { old: { b: "1", a: "1" } },
				},
			}),
		).toEqual([
			"package.json: sort workspaces.catalog by package name",
			"package.json: sort workspaces.catalogs.old by package name",
		]);
	});
});
