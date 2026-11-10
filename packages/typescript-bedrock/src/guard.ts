import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { $, Glob } from "bun";

import { planAdoption, PROJECT_FILES } from "./adopt/adopt.ts";
import { listFiles } from "./files.ts";
import { readOxlintIgnores } from "./ignores.ts";

// Written as alternations so this file does not match itself.
const SUPPRESSION =
	/(?:eslint|oxlint)-disable|biome-(?:ignore)|@ts-(?:ignore|expect-error|nocheck)/u;
const OTHER_PACKAGE_MANAGER = /\b(?:npm|npx|pnpm|pnpx|yarn)\b/u;
const SOURCE = new Glob(
	"**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts,vue,svelte,astro}",
);
const MANIFEST = new Glob("**/package.json");
const RE_EXPORT =
	/export\s+(?:type\s+)?(?:\*(?:\s+as\s+[\w$]+)?|\{[^}]*\})\s+from\s+["'](\.{1,2}\/[^"']+)["']/gu;
const SPECIFIER_ENDINGS = [
	"",
	".ts",
	".tsx",
	".mts",
	".js",
	".mjs",
	"/index.ts",
	"/index.js",
];
const UNDOCUMENTED =
	/^(.+?):(\d+):\d+: .* \[Error\/jsdoc-js\(require-jsdoc\)\]$/u;
// `.npmrc` is absent on purpose: Bun reads it for registry auth.
const FOREIGN_FILES = [
	".pnpmfile.cjs",
	".yarnrc",
	".yarnrc.yml",
	"bun.lockb",
	"npm-shrinkwrap.json",
	"package-lock.json",
	"pnpm-lock.yaml",
	"pnpm-workspace.yaml",
	"yarn.lock",
];

export const findSuppressions = async (
	cwd: string,
	paths: readonly string[],
): Promise<string[]> => {
	const texts = await Promise.all(
		paths.map((path) => Bun.file(join(cwd, path)).text()),
	);
	return paths.flatMap((path, index) =>
		(texts[index] ?? "")
			.split("\n")
			.flatMap((line, row) =>
				SUPPRESSION.test(line)
					? [`${path}:${row + 1}: suppression comment`]
					: [],
			),
	);
};

const findForeignFiles = async (cwd: string): Promise<string[]> => {
	const present = await Promise.all(
		FOREIGN_FILES.map((path) => Bun.file(join(cwd, path)).exists()),
	);
	return FOREIGN_FILES.filter((_, index) => present[index] === true).map(
		(path) => `${path}: not used in a Bun-only repo`,
	);
};

const findForeignScripts = async (cwd: string): Promise<string[]> => {
	const manifest: unknown = await Bun.file(join(cwd, "package.json")).json();
	const scripts =
		typeof manifest === "object" &&
		manifest !== null &&
		"scripts" in manifest &&
		typeof manifest.scripts === "object" &&
		manifest.scripts !== null
			? manifest.scripts
			: {};
	return Object.entries(scripts)
		.filter(
			([, command]) =>
				typeof command === "string" && OTHER_PACKAGE_MANAGER.test(command),
		)
		.map(
			([name]) =>
				`package.json: script "${name}" calls another package manager`,
		);
};

/** `bun fmt` sorts dependencies but not catalogs, so the guard checks them. */
export const findUnsortedCatalogs = (manifest: unknown): string[] => {
	const workspaces =
		typeof manifest === "object" &&
		manifest !== null &&
		"workspaces" in manifest
			? manifest.workspaces
			: undefined;
	if (typeof workspaces !== "object" || workspaces === null) {
		return [];
	}
	const named =
		"catalogs" in workspaces &&
		typeof workspaces.catalogs === "object" &&
		workspaces.catalogs !== null
			? Object.entries(workspaces.catalogs)
			: [];
	const catalogs: [string, unknown][] = [
		["catalog", "catalog" in workspaces ? workspaces.catalog : undefined],
		...named.map(([name, value]): [string, unknown] => [
			`catalogs.${name}`,
			value,
		]),
	];
	return catalogs.flatMap(([name, catalog]) => {
		if (typeof catalog !== "object" || catalog === null) {
			return [];
		}
		const keys = Object.keys(catalog);
		return keys.every(
			(key, index) => index === 0 || (keys[index - 1] ?? "") < key,
		)
			? []
			: [`package.json: sort workspaces.${name} by package name`];
	});
};

// The editor runs this package while `bun check` runs the TypeScript built into Bun.
const findTypescriptDrift = async (cwd: string): Promise<string[]> => {
	const file = Bun.file(join(cwd, "node_modules/typescript/package.json"));
	if (!(await file.exists())) {
		return [];
	}
	const manifest: unknown = await file.json();
	const installed =
		typeof manifest === "object" && manifest !== null && "version" in manifest
			? manifest.version
			: undefined;
	const builtIn = process.versions["typescript"];
	return installed === builtIn
		? []
		: [
				`typescript: ${String(installed)} is installed but \`bun check\` runs ${String(builtIn)}; set the catalog to ${String(builtIn)}`,
			];
};

const ENTRY_KEYS = new Set(["exports", "main", "module"]);

/** The string leaves of `exports`, `main`, and `module`: the files other packages import. */
const entryPoints = (manifest: unknown): string[] => {
	const leaves = (value: unknown): string[] => {
		if (typeof value === "string") {
			return [value];
		}
		return typeof value === "object" && value !== null
			? Object.values(value).flatMap((child) => leaves(child))
			: [];
	};
	return typeof manifest === "object" && manifest !== null
		? Object.entries(manifest)
				.filter(([key]) => ENTRY_KEYS.has(key))
				.flatMap(([, value]) => leaves(value))
		: [];
};

/** The files `path` re-exports from, among `sources`. */
const reExported = (
	path: string,
	text: string,
	sources: ReadonlySet<string>,
): string[] =>
	[...text.matchAll(RE_EXPORT)].flatMap(([, specifier = ""]) => {
		const base = join(dirname(path), specifier);
		return SPECIFIER_ENDINGS.map((ending) => base + ending)
			.filter((candidate) => sources.has(candidate))
			.slice(0, 1);
	});

/**
 * The tracked source files each package exposes: its entry points and, through
 * relative re-exports, the files they re-export from.
 */
const listApiFiles = async (
	cwd: string,
	ignore: readonly string[],
): Promise<string[]> => {
	const sources = new Set(await listFiles(cwd, SOURCE, ignore));
	const manifests = await listFiles(cwd, MANIFEST, ignore);
	const entries = await Promise.all(
		manifests.map(async (path) => {
			const manifest: unknown = await Bun.file(join(cwd, path)).json();
			return entryPoints(manifest).map((entry) => join(dirname(path), entry));
		}),
	);
	const found = new Set<string>();
	const visit = async (paths: readonly string[]): Promise<void> => {
		const fresh = [...new Set(paths)].filter(
			(path) => sources.has(path) && !found.has(path),
		);
		if (fresh.length === 0) {
			return;
		}
		for (const path of fresh) {
			found.add(path);
		}
		const texts = await Promise.all(
			fresh.map((path) => Bun.file(join(cwd, path)).text()),
		);
		await visit(
			fresh.flatMap((path, index) =>
				reExported(path, texts[index] ?? "", sources),
			),
		);
	};
	await visit(entries.flat());
	return [...found].toSorted();
};

/** Exported declarations in the package API that lack a JSDoc comment. */
const findUndocumentedApi = async (
	cwd: string,
	ignore: readonly string[],
): Promise<string[]> => {
	const files = await listApiFiles(cwd, ignore);
	if (files.length === 0) {
		return [];
	}
	const oxlint = join(
		dirname(fileURLToPath(import.meta.resolve("oxlint/package.json"))),
		"bin/oxlint",
	);
	const config = resolve(import.meta.dir, "../oxlintrc.api.json");
	const result =
		await $`bun ${oxlint} --disable-nested-config -c ${config} -f unix ${files}`
			.cwd(cwd)
			.nothrow()
			.quiet();
	const problems = result.stdout
		.toString()
		.split("\n")
		.flatMap((line) => {
			const [, path, row] = UNDOCUMENTED.exec(line) ?? [];
			return path === undefined
				? []
				: [`${path}:${row}: add a JSDoc comment; the package exports this`];
		});
	return result.exitCode === 0 || problems.length > 0
		? problems.toSorted()
		: [`oxlint failed on the package API: ${result.stderr.toString().trim()}`];
};

/** The files adopt would still change, except the ones the project owns. */
const findDrift = async (cwd: string): Promise<string[]> => {
	const { steps } = await planAdoption(resolve(import.meta.dir, ".."), cwd, {});
	return steps
		.filter((step) => !PROJECT_FILES.has(step.path))
		.map(
			(step) =>
				`${step.path}: differs from what typescript-bedrock writes; run typescript-bedrock adopt --dry-run to see the change`,
		);
};

export const findProblems = async (cwd: string): Promise<string[]> => {
	const ignore = await readOxlintIgnores(cwd);
	return [
		...(await findSuppressions(cwd, await listFiles(cwd, SOURCE, ignore))),
		...(await findUndocumentedApi(cwd, ignore)),
		...(await findForeignFiles(cwd)),
		...(await findForeignScripts(cwd)),
		...(await findTypescriptDrift(cwd)),
		...findUnsortedCatalogs(await Bun.file(join(cwd, "package.json")).json()),
		...(await findDrift(cwd)),
	];
};

export const guard = async (cwd: string): Promise<number> => {
	const problems = await findProblems(cwd);
	for (const problem of problems) {
		console.error(problem);
	}
	if (problems.length === 0) {
		return 0;
	}
	console.error("Fix the cause instead of suppressing it. See AGENTS.md.");
	return 1;
};
