import { realpath } from "node:fs/promises";
import { basename, join } from "node:path";

import { copies, existing, planBunfig } from "./copies.ts";
import { asRecord, type Json, readJsonc, toJson } from "./json.ts";
import { migrateManifest } from "./manifest.ts";
import { sharedSteps } from "./shared.ts";
import { pending, type Step } from "./steps.ts";
import { templateManifest } from "./template.ts";
import { testSteps } from "./tests.ts";
import { migrateTsconfig } from "./tsconfig.ts";

export interface AdoptOptions {
	/** Where the repo gets typescript-bedrock from, such as `file:../typescript-bedrock`. */
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
	".githooks/pre-push",
	".markdownlint.jsonc",
	"justfile",
];
/** Thin files that point at typescript-bedrock. A changed copy is backed up, then replaced,
 * except the entries a repo adds to the ignore list of `.oxlintrc.json` or `.markdownlint-cli2.jsonc`, which stay. */
const TOOL_FILES = [
	".bun-version",
	".github/workflows/typescript.yml",
	".just/typescript.just",
	".markdownlint-cli2.jsonc",
	".oxlintrc.json",
];
/** Project files a repo may have tuned. They are written only if missing. */
const STARTER_FILES = [".vscode/extensions.json", ".vscode/settings.json"];
/** The CI file of the old template, or one that runs another package manager. */
const OLD_CI = ".github/workflows/ci.yml";
const OLD_CI_COMMAND = /\b(?:npm|npx|pnpm|yarn)\b|bun run verify/u;

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
			"The target is typescript-bedrock itself. Pass the path of the repo to adopt.",
		);
	}
	const filesRoot = join(packageRoot, "files");
	const migrated = await planManifest(packageRoot, target, options);
	const ts = await planTsconfig(packageRoot, target);
	const bunfig = await planBunfig(filesRoot, target);
	const ci = Bun.file(join(target, OLD_CI));
	const oldCi = (await ci.exists()) && OLD_CI_COMMAND.test(await ci.text());
	const replaced = [
		...(await existing(target, [...FORMATTER_CONFIGS, ...REPLACED_CONFIGS])),
		...(oldCi ? [OLD_CI] : []),
	];
	const steps: Step[] = [
		...(await jsonWrite(target, "package.json", migrated.manifest)),
		...(await jsonWrite(target, "tsconfig.json", ts.tsconfig)),
		...bunfig.steps,
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
		notes: [...migrated.notes, ...ts.notes, ...bunfig.notes],
	};
};
