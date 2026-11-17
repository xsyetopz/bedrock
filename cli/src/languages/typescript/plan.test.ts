import { describe, expect, test } from "bun:test";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import { tempRepo } from "../../core/testing.ts";
import { adoptTypescript, contextIn, textOf } from "./harness.ts";
import { installDependencies } from "./install.ts";
import { asRecord, readJsonc } from "./json.ts";

const LEGACY = {
	"package.json": JSON.stringify({
		name: "old",
		scripts: {
			guard: "typescript-bedrock guard",
			lint: "oxlint",
			test: "jest",
		},
		devDependencies: { "typescript-bedrock": "catalog:", eslint: "^9.0.0" },
		workspaces: { catalog: { "typescript-bedrock": "1.2.3" } },
	}),
	"tsconfig.json": JSON.stringify({
		extends: "typescript-bedrock/tsconfig.base.json",
		compilerOptions: { rootDir: "." },
	}),
	".oxlintrc.json": JSON.stringify({
		extends: ["./node_modules/typescript-bedrock/oxlintrc.json"],
		ignorePatterns: ["vendor/**"],
		rules: { "no-console": "off" },
	}),
	".markdownlint-cli2.jsonc": JSON.stringify({
		ignores: ["notes/**"],
		config: { extends: "./node_modules/typescript-bedrock/markdownlint.jsonc" },
	}),
	"eslint.config.js": "export default [];\n",
	"yarn.lock": "",
	".github/workflows/ci.yml": "steps:\n  - run: npm test\n",
	"project.just": "guard:\n    bunx typescript-bedrock guard\n",
	"src/a.test.ts": 'test("a", () => {});\n',
};

describe("planTypescript on a repo that typescript-bedrock adopted", () => {
	test("drops the package, its extends, and the tools it replaced", async () => {
		const root = await tempRepo(LEGACY);
		const plan = await adoptTypescript(root);
		const manifest = asRecord(await readJsonc(join(root, "package.json")));
		const text = [
			await textOf(root, "package.json"),
			await textOf(root, "tsconfig.json"),
			await textOf(root, ".oxlintrc.json"),
			await textOf(root, ".markdownlint-cli2.jsonc"),
		].join("\n");
		expect(text).not.toContain("typescript-bedrock");
		expect(asRecord(manifest["scripts"])).toMatchObject({
			guard: "just typescript::guard",
			lint: "just lint",
		});
		expect(await exists(join(root, "eslint.config.js"))).toBe(false);
		expect(await exists(join(root, ".github/workflows/ci.yml"))).toBe(false);
		expect(plan.notes.join("\n")).toContain(
			"project.just still mentions typescript-bedrock",
		);
	});

	test("keeps the repo's own ignores and says what the rewrite dropped", async () => {
		const root = await tempRepo(LEGACY);
		const plan = await adoptTypescript(root);
		expect(await textOf(root, ".oxlintrc.json")).toContain('"vendor/**"');
		expect(await textOf(root, ".markdownlint-cli2.jsonc")).toContain(
			'"notes/**"',
		);
		expect(plan.notes.join("\n")).toContain('"rules" were not carried over');
	});

	test("moves test files onto bun:test", async () => {
		const root = await tempRepo(LEGACY);
		await adoptTypescript(root);
		expect(await textOf(root, "src/a.test.ts")).toStartWith(
			'import { test } from "bun:test";',
		);
	});

	test("is stable: a second adopt plans nothing", async () => {
		const root = await tempRepo(LEGACY);
		await adoptTypescript(root);
		const second = await adoptTypescript(root);
		expect(second.steps).toEqual([]);
	});
});

describe("planTypescript on a fresh repo", () => {
	test("writes full configs that extend nothing", async () => {
		const root = await tempRepo();
		await adoptTypescript(root);
		const configs = [
			await textOf(root, "tsconfig.json"),
			await textOf(root, ".oxlintrc.json"),
			await textOf(root, ".markdownlint-cli2.jsonc"),
		];
		for (const config of configs) {
			expect(config).not.toContain('"extends"');
		}
		expect(await textOf(root, ".oxlintrc.json")).toContain(".claude/**");
		expect(await textOf(root, "bunfig.toml")).toContain("[install]");
	});

	test("lists package.json, tsconfig.json, and bunfig.toml as the project's", async () => {
		const root = await tempRepo();
		const plan = await adoptTypescript(root);
		expect(plan.projectFiles.has("package.json")).toBe(true);
		expect(plan.projectFiles.has("tsconfig.json")).toBe(true);
		expect(plan.projectFiles.has("bunfig.toml")).toBe(true);
	});
});

describe("installDependencies", () => {
	test("removes the other lockfiles once bun.lock exists", async () => {
		const root = await tempRepo({
			"package.json": JSON.stringify({
				name: "x",
				version: "1.0.0",
				dependencies: { dep: "file:./dep" },
			}),
			"dep/package.json": '{"name":"dep","version":"1.0.0"}',
			"yarn.lock": "# yarn lockfile v1\n",
		});
		await installDependencies(contextIn(root));
		expect(await exists(join(root, "bun.lock"))).toBe(true);
		expect(await exists(join(root, "yarn.lock"))).toBe(false);
	}, 60_000);
});
