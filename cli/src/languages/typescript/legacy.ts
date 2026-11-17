import { join } from "node:path";

import { Glob } from "bun";

import { existing, readText } from "../../core/fs.ts";
import type { Step } from "../../core/steps.ts";
import { listSources } from "./ignores.ts";
import { asRecord, isRecord, type Json, readJsonc, toJson } from "./json.ts";
import { LEGACY_PACKAGE } from "./manifest.ts";

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
/** Configs of tools Bun and bedrock replace. Core removes the hook managers and Makefile. */
const REPLACED_CONFIGS = [
	".eslintignore",
	".eslintrc",
	".eslintrc.cjs",
	".eslintrc.js",
	".eslintrc.json",
	".eslintrc.yaml",
	".eslintrc.yml",
	".lintstagedrc",
	".lintstagedrc.json",
	".lintstagedrc.js",
	".pnp.cjs",
	".pnp.loader.mjs",
	".pnpmfile.cjs",
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
	"lint-staged.config.js",
	"lint-staged.config.mjs",
	"pnpm-workspace.yaml",
	"vitest.config.js",
	"vitest.config.mjs",
	"vitest.config.mts",
	"vitest.config.ts",
];
/** Other package managers' lockfiles; `bun install` migrates them into `bun.lock` first. */
export const LOCKFILES = [
	"bun.lockb",
	"npm-shrinkwrap.json",
	"package-lock.json",
	"pnpm-lock.yaml",
	"yarn.lock",
];
/** The CI file of the old template, or one that runs another package manager. */
const OLD_CI = ".github/workflows/ci.yml";
const OLD_CI_COMMAND = /\b(?:npm|npx|pnpm|yarn)\b|bun run verify/u;
const WORKFLOWS = new Glob(".github/workflows/*.{yml,yaml}");
/** Written by bedrock, so a mention there is bedrock's own to replace. */
const MANAGED = new Set([".github/workflows/typescript.yml"]);
const MANIFESTS = new Glob("**/package.json");

/** Replaced tool configs and an old-template CI file that adopt removes. */
export const removalSteps = async (root: string): Promise<Step[]> => {
	const ci = await readText(join(root, OLD_CI));
	const paths = [
		...(await existing(root, [...FORMATTER_CONFIGS, ...REPLACED_CONFIGS])),
		...(ci !== undefined && OLD_CI_COMMAND.test(ci) ? [OLD_CI] : []),
	];
	return paths.map((path): Step => ({ kind: "remove", path }));
};

/** Which of the other package managers' lockfiles sit in `root`. */
export const lockfilesIn = (root: string): Promise<string[]> =>
	existing(root, LOCKFILES);

/** A package.json without the `typescript-bedrock` dependency, or undefined when it had none. */
const withoutLegacy = (manifest: Json): Json | undefined => {
	const holders = ["dependencies", "devDependencies"].filter(
		(key) => LEGACY_PACKAGE in asRecord(manifest[key]),
	);
	if (holders.length === 0) {
		return undefined;
	}
	const next: Json = { ...manifest };
	for (const key of holders) {
		next[key] = Object.fromEntries(
			Object.entries(asRecord(manifest[key])).filter(
				([name]) => name !== LEGACY_PACKAGE,
			),
		);
	}
	return next;
};

/** Drops `typescript-bedrock` from the workspace packages' manifests; the root one has its own migration. */
export const workspaceManifestSteps = async (root: string): Promise<Step[]> => {
	const listed = await listSources(root, MANIFESTS);
	const paths = listed.filter((path) => path !== "package.json");
	const manifests = await Promise.all(
		paths.map((path) => readJsonc(join(root, path))),
	);
	return paths.flatMap((path, index): Step[] => {
		const next = withoutLegacy(asRecord(manifests[index]));
		return next === undefined
			? []
			: [{ kind: "write", path, content: toJson(next) }];
	});
};

/** The files bedrock does not write that still name `typescript-bedrock`: a person moves what they ran. */
export const legacyMentions = async (root: string): Promise<string[]> => {
	const paths = [
		...(await listSources(root, WORKFLOWS)),
		...(await existing(root, ["project.just"])),
	].filter((path) => !MANAGED.has(path));
	const texts = await Promise.all(
		paths.map((path) => readText(join(root, path))),
	);
	return paths
		.filter((_, index) => texts[index]?.includes(LEGACY_PACKAGE) === true)
		.map(
			(path) =>
				`${path} still mentions ${LEGACY_PACKAGE}, which bedrock no longer installs. Run \`just typescript::guard\` and \`just lint\` instead.`,
		);
};

/** Whether a tool config still names the old `typescript-bedrock` package, such as in `extends`. */
export const extendsLegacy = (config: unknown): boolean =>
	isRecord(config) && JSON.stringify(config).includes(LEGACY_PACKAGE);
