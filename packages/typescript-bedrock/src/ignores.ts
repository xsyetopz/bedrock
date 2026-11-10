import { join } from "node:path";

import { Glob } from "bun";

import { asRecord, asStrings, readJsonc } from "./adopt/json.ts";

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

/** The `ignorePatterns` of the repo's `.oxlintrc.json`, which the repo may extend. */
export const readOxlintIgnores = async (cwd: string): Promise<string[]> =>
	asStrings(
		asRecord(await readJsonc(join(cwd, ".oxlintrc.json")))["ignorePatterns"],
	);

/** The `[test] pathIgnorePatterns` of the repo's `bunfig.toml`: the paths `bun test` skips. */
export const readBunfigTestIgnores = async (cwd: string): Promise<string[]> => {
	const file = Bun.file(join(cwd, "bunfig.toml"));
	if (!(await file.exists())) {
		return [];
	}
	const patterns = asRecord(
		asRecord(Bun.TOML.parse(await file.text()))["test"],
	)["pathIgnorePatterns"];
	return typeof patterns === "string" ? [patterns] : asStrings(patterns);
};
