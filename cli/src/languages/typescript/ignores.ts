import { join } from "node:path";

import { Glob } from "bun";

import { listFiles } from "../../core/fs.ts";
import { asRecord, asStrings, readJsonc } from "./json.ts";

/** The globs a gitignore-style `pattern` stands for: the path itself and what is under it. */
const globsOf = (pattern: string): string[] => {
	const dir = pattern.replace(/\/+$/u, "");
	const anchored = dir.startsWith("/") || dir.includes("/", 1);
	const base = dir.replace(/^\//u, "");
	const glob = anchored ? base : `**/${base}`;
	return [glob, `${glob}/**`];
};

/**
 * Whether `path`, relative to the repo root, matches `patterns`: gitignore-style
 * entries (a bare name matches at any depth, `dir/` matches a directory, a
 * later `!entry` re-includes) that Bun's glob syntax can read.
 */
export const isIgnored = (
	path: string,
	patterns: readonly string[],
): boolean => {
	let ignored = false;
	for (const entry of patterns) {
		const negated = entry.startsWith("!");
		const pattern = negated ? entry.slice(1) : entry;
		if (
			pattern !== "" &&
			globsOf(pattern).some((glob) => new Glob(glob).match(path))
		) {
			ignored = !negated;
		}
	}
	return ignored;
};

/** The `ignorePatterns` of the repo's `.oxlintrc.json`. */
export const readOxlintIgnores = async (root: string): Promise<string[]> =>
	asStrings(
		asRecord(await readJsonc(join(root, ".oxlintrc.json")))["ignorePatterns"],
	);

/** The `[test] pathIgnorePatterns` of the repo's `bunfig.toml`: the paths `bun test` skips. */
export const readBunfigTestIgnores = async (
	root: string,
): Promise<string[]> => {
	const file = Bun.file(join(root, "bunfig.toml"));
	if (!(await file.exists())) {
		return [];
	}
	const patterns = asRecord(
		asRecord(Bun.TOML.parse(await file.text()))["test"],
	)["pathIgnorePatterns"];
	return typeof patterns === "string" ? [patterns] : asStrings(patterns);
};

/** The files under `root` that `glob` matches, minus git-ignored files, skipped directories, and `ignore` entries. */
export const listSources = async (
	root: string,
	glob: Glob,
	ignore: readonly string[] = [],
): Promise<string[]> => {
	const files = await listFiles(root, glob);
	return files.filter((path) => !isIgnored(path, ignore));
};
