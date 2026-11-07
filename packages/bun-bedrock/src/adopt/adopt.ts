import { realpath } from "node:fs/promises";
import { basename, join } from "node:path";

import { asRecord, exists, type Json, readJsonc, toJson } from "./json.ts";
import { migrateManifest } from "./manifest.ts";
import { sharedSteps } from "./shared.ts";
import { pending, type Step } from "./steps.ts";
import { templateManifest } from "./template.ts";
import { testSteps } from "./tests.ts";
import { migrateTsconfig } from "./tsconfig.ts";

export interface AdoptOptions {
	/** Where the repo gets bun-bedrock from, such as `file:../bun-bedrock`. */
	readonly packageSpec?: string | undefined;
}

export interface Plan {
	readonly steps: readonly Step[];
	/** Removed after `bun install` has migrated them into `bun.lock`. */
	readonly lockfiles: readonly string[];
	readonly notes: readonly string[];
}

/** Other formatters' configs: every adopted repo takes the bedrock style. */
const FORMATTER_CONFIGS = [
	".oxfmtrc.json",
	".oxfmtrc.jsonc",
	".prettierrc",
	".prettierrc.cjs",
	".prettierrc.js",
	".prettierrc.json",
	".prettierrc.json5",
	".prettierrc.mjs",
	".prettierrc.toml",
	".prettierrc.ts",
	".prettierrc.yaml",
	".prettierrc.yml",
	"prettier.config.cjs",
	"prettier.config.js",
	"prettier.config.mjs",
	"prettier.config.ts",
];
const REPLACED_CONFIGS = [
	".eslintignore",
	".eslintrc",
	".eslintrc.cjs",
	".eslintrc.js",
	".eslintrc.json",
	".eslintrc.yaml",
	".eslintrc.yml",
	".husky",
	".lefthook.yml",
	".lintstagedrc",
	".lintstagedrc.json",
	".lintstagedrc.js",
	".pnp.cjs",
	".pnp.loader.mjs",
	".pnpmfile.cjs",
	".pre-commit-config.yaml",
	".simple-git-hooks.json",
	".yarn",
	".yarnrc",
	".yarnrc.yml",
	"biome.json",
	"biome.jsonc",
	"eslint.config.cjs",
	"eslint.config.js",
	"eslint.config.mjs",
	"eslint.config.mts",
	"eslint.config.ts",
	"jest.config.cjs",
	"jest.config.js",
	"jest.config.json",
	"jest.config.mjs",
	"jest.config.ts",
	"lefthook.yaml",
	"lefthook.yml",
	"lint-staged.config.js",
	"lint-staged.config.mjs",
	"Makefile",
	"pnpm-workspace.yaml",
	"vitest.config.js",
	"vitest.config.mjs",
	"vitest.config.mts",
	"vitest.config.ts",
];
const LOCKFILES = [
	"bun.lockb",
	"npm-shrinkwrap.json",
	"package-lock.json",
	"pnpm-lock.yaml",
	"yarn.lock",
];
/** Shared by every bedrock; a repository test keeps these equal to `base/`. */
export const BASE_FILES = [
	".github/workflows/base.yml",
	".githooks/commit-msg",
	".githooks/pre-commit",
	".markdownlint.jsonc",
	"justfile",
];
/** Thin files that point at bun-bedrock. A changed copy is backed up, then replaced. */
const TOOL_FILES = [
	".bun-version",
	".github/workflows/bun.yml",
	".just/bun.just",
	".markdownlint-cli2.jsonc",
	".oxlintrc.json",
];
/** Project files a repo may have tuned. They are written only if missing. */
const STARTER_FILES = [".vscode/extensions.json", ".vscode/settings.json"];
/** The CI file of the old template, or one that runs another package manager. */
const OLD_CI = ".github/workflows/ci.yml";
const OLD_CI_COMMAND = /\b(?:npm|npx|pnpm|yarn)\b|bun run verify/u;

const existing = async (
	root: string,
	paths: readonly string[],
): Promise<string[]> => {
	const found = await Promise.all(
		paths.map((path) => exists(join(root, path))),
	);
	return paths.filter((_, index) => found[index] === true);
};

/** npm packs drop `bunfig.toml`, and linters read nested configs, so these keep a suffix. */
const STORED_WITH_SUFFIX = new Set([
	".markdownlint-cli2.jsonc",
	".oxlintrc.json",
	"bunfig.toml",
]);
const storedPath = (path: string): string =>
	STORED_WITH_SUFFIX.has(path) ? `${path}.template` : path;

const copyFile = async (dir: string, path: string): Promise<Step> => ({
	kind: "write",
	path,
	content: await Bun.file(join(dir, storedPath(path))).text(),
	executable: path.startsWith(".githooks/"),
});

const copies = async (
	dir: string,
	target: string,
	paths: readonly string[],
	overwrite: boolean,
): Promise<Step[]> => {
	const present = new Set(overwrite ? [] : await existing(target, paths));
	return Promise.all(
		paths
			.filter((path) => !present.has(path))
			.map((path) => copyFile(dir, path)),
	);
};

const planManifest = async (
	packageRoot: string,
	target: string,
	{ packageSpec }: AdoptOptions,
): Promise<{ manifest: Json; notes: string[] }> => {
	const manifest = (await readJsonc(join(target, "package.json"))) ?? {
		name: basename(target),
		version: "0.0.0",
		private: true,
		type: "module",
	};
	const pnpmFile = Bun.file(join(target, "pnpm-workspace.yaml"));
	const pnpm = (await pnpmFile.exists())
		? asRecord(Bun.YAML.parse(await pnpmFile.text()))
		: undefined;
	return migrateManifest(
		manifest,
		await templateManifest(packageRoot, manifest, packageSpec),
		pnpm,
	);
};

const planTsconfig = async (
	packageRoot: string,
	target: string,
): Promise<{ tsconfig: Json; notes: string[] }> => {
	const tsconfig =
		(await readJsonc(join(target, "tsconfig.json"))) ??
		(await readJsonc(join(packageRoot, "files/tsconfig.json"))) ??
		{};
	const base = (await readJsonc(join(packageRoot, "tsconfig.base.json"))) ?? {};
	return migrateTsconfig(tsconfig, base);
};

/** Rewrites a JSON file only when its data changes, so its layout and comments stay. */
const jsonWrite = async (
	target: string,
	path: string,
	next: Json,
): Promise<Step[]> =>
	Bun.deepEquals(await readJsonc(join(target, path)), next)
		? []
		: [{ kind: "write", path, content: toJson(next) }];

/** The files that stay the project's own: the guard does not call them drift. */
export const PROJECT_FILES = new Set([
	"bunfig.toml",
	"package.json",
	"tsconfig.json",
	...STARTER_FILES,
]);

export const planAdoption = async (
	packageRoot: string,
	target: string,
	options: AdoptOptions,
): Promise<Plan> => {
	if ((await realpath(packageRoot)) === (await realpath(target))) {
		throw new Error(
			"The target is bun-bedrock itself. Pass the path of the repo to adopt.",
		);
	}
	const filesRoot = join(packageRoot, "files");
	const migrated = await planManifest(packageRoot, target, options);
	const ts = await planTsconfig(packageRoot, target);
	const ci = Bun.file(join(target, OLD_CI));
	const oldCi = (await ci.exists()) && OLD_CI_COMMAND.test(await ci.text());
	const replaced = [
		...(await existing(target, [...FORMATTER_CONFIGS, ...REPLACED_CONFIGS])),
		...(oldCi ? [OLD_CI] : []),
	];
	const steps: Step[] = [
		...(await jsonWrite(target, "package.json", migrated.manifest)),
		...(await jsonWrite(target, "tsconfig.json", ts.tsconfig)),
		...(await copies(filesRoot, target, ["bunfig.toml"], true)),
		...(await copies(join(filesRoot, "base"), target, BASE_FILES, true)),
		...(await copies(filesRoot, target, TOOL_FILES, true)),
		...(await copies(filesRoot, target, STARTER_FILES, false)),
		...replaced.map((path): Step => ({ kind: "remove", path })),
		...(await sharedSteps(filesRoot, target)),
		...(await testSteps(target)),
	];
	return {
		steps: await pending(target, steps),
		lockfiles: await existing(target, LOCKFILES),
		notes: [...migrated.notes, ...ts.notes],
	};
};
