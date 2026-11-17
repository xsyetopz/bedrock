import { mkdir, mkdtemp, rm } from "node:fs/promises";
import { dirname, join } from "node:path";

import { $ } from "bun";

import type { Context, Finding } from "../../../core/language.ts";
import { listSources } from "../ignores.ts";
import { asRecord, readJsonc } from "../json.ts";
import { sourcesOf } from "./suppressions.ts";

const MANIFEST = new Bun.Glob("**/package.json");
const ENTRY_KEYS = new Set(["exports", "main", "module"]);
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
const CACHE = "node_modules/.cache";

/** The string leaves of `exports`, `main`, and `module`: the files other packages import. */
const entryPoints = (manifest: unknown): string[] => {
	const leaves = (value: unknown): string[] =>
		typeof value === "string"
			? [value]
			: Object.values(asRecord(value)).flatMap((child) => leaves(child));
	return Object.entries(asRecord(manifest))
		.filter(([key]) => ENTRY_KEYS.has(key))
		.flatMap(([, value]) => leaves(value));
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
 * The source files each package exposes: its entry points and, through
 * relative re-exports, the files they re-export from.
 */
const listApiFiles = async (
	root: string,
	ignore: readonly string[],
): Promise<string[]> => {
	const sources = new Set(await sourcesOf(root, ignore));
	const manifestPaths = await listManifests(root, ignore);
	const manifests = await Promise.all(
		manifestPaths.map(async (path) =>
			entryPoints(await readJsonc(join(root, path))).map((entry) =>
				join(dirname(path), entry),
			),
		),
	);
	const found = new Set<string>();
	const visit = async (paths: readonly string[]): Promise<void> => {
		const fresh = [...new Set(paths)].filter(
			(path) => sources.has(path) && !found.has(path),
		);
		for (const path of fresh) {
			found.add(path);
		}
		const texts = await Promise.all(
			fresh.map((path) => Bun.file(join(root, path)).text()),
		);
		const next = fresh.flatMap((path, index) =>
			reExported(path, texts[index] ?? "", sources),
		);
		if (next.length > 0) {
			await visit(next);
		}
	};
	await visit(manifests.flat());
	return [...found].toSorted();
};

const listManifests = (
	root: string,
	ignore: readonly string[],
): Promise<string[]> => listSources(root, MANIFEST, ignore);

/** The API config, written where oxlint resolves the jsdoc plugin from the repo's `node_modules`. */
const apiConfig = async (
	context: Context,
	dir: string,
): Promise<string | undefined> => {
	const template = await context.templates("merge").read("oxlintrc.api.json");
	if (template === undefined) {
		return undefined;
	}
	const path = join(dir, "oxlintrc.api.json");
	await Bun.write(path, template);
	return path;
};

/** Exported declarations in the package API that lack a JSDoc comment. */
export const findUndocumentedApi = async (
	context: Context,
	ignore: readonly string[],
): Promise<Finding[]> => {
	const files = await listApiFiles(context.root, ignore);
	if (files.length === 0) {
		return [];
	}
	// Under node_modules so the config's bare plugin specifier resolves from the repo.
	await mkdir(join(context.root, CACHE), { recursive: true });
	const dir = await mkdtemp(join(context.root, CACHE, "bedrock-api-"));
	try {
		const config = await apiConfig(context, dir);
		if (config === undefined) {
			return [];
		}
		const result =
			await $`bun run oxlint --disable-nested-config -c ${config} -f unix ${files}`
				.cwd(context.root)
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
			: [
					`oxlint failed on the package API: ${result.stderr.toString().trim()}`,
				];
	} finally {
		await rm(dir, { recursive: true, force: true });
	}
};
