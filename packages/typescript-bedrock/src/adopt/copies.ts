import { join } from "node:path";

import { mergeBunfig } from "./bunfig.ts";
import { IGNORE_KEYS, mergeIgnoreList } from "./ignore-lists.ts";
import { exists, readJsonc } from "./json.ts";
import type { Step } from "./steps.ts";

/** The paths under `root` that exist. */
export const existing = async (
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
export const storedPath = (path: string): string =>
	STORED_WITH_SUFFIX.has(path) ? `${path}.template` : path;

/** The template's text, plus the ignore entries the repo's copy adds to it. */
const contentOf = async (
	dir: string,
	target: string,
	path: string,
): Promise<string> => {
	const template = await Bun.file(join(dir, storedPath(path))).text();
	const key = IGNORE_KEYS[path];
	return key === undefined
		? template
		: mergeIgnoreList(template, await readJsonc(join(target, path)), key);
};

const copyFile = async (
	dir: string,
	target: string,
	path: string,
): Promise<Step[]> => {
	const content = await contentOf(dir, target, path);
	// A repo's own layout of its ignore list stays when the data matches.
	if (
		path in IGNORE_KEYS &&
		Bun.deepEquals(
			Bun.JSONC.parse(content),
			await readJsonc(join(target, path)),
		)
	) {
		return [];
	}
	return [
		{ kind: "write", path, content, executable: path.startsWith(".githooks/") },
	];
};

/** Writes each of `paths` from `dir`; with `overwrite` off, only the ones `target` lacks. */
export const copies = async (
	dir: string,
	target: string,
	paths: readonly string[],
	overwrite: boolean,
): Promise<Step[]> => {
	const present = new Set(overwrite ? [] : await existing(target, paths));
	const steps = await Promise.all(
		paths
			.filter((path) => !present.has(path))
			.map((path) => copyFile(dir, target, path)),
	);
	return steps.flat();
};

/** The template's `bunfig.toml` plus the settings the repo's copy adds. */
export const planBunfig = async (
	filesRoot: string,
	target: string,
): Promise<{ steps: Step[]; notes: string[] }> => {
	const file = Bun.file(join(target, "bunfig.toml"));
	const merged = mergeBunfig(
		await Bun.file(join(filesRoot, storedPath("bunfig.toml"))).text(),
		(await file.exists()) ? await file.text() : undefined,
	);
	return {
		steps: [{ kind: "write", path: "bunfig.toml", content: merged.content }],
		notes: merged.notes,
	};
};
