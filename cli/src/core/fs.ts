import { lstat, readlink } from "node:fs/promises";
import { join } from "node:path";

import { $, type Glob } from "bun";

/** The file's text, or undefined when it is missing or not a file. */
export const readText = async (path: string): Promise<string | undefined> => {
	const file = Bun.file(path);
	try {
		return (await file.exists()) ? await file.text() : undefined;
	} catch {
		return undefined;
	}
};

/** Whether anything, including a dangling link, sits at `path`. */
export const exists = async (path: string): Promise<boolean> => {
	try {
		await lstat(path);
		return true;
	} catch {
		return false;
	}
};

/** Whether `path` is a regular file, not a link. */
export const isRegularFile = async (path: string): Promise<boolean> => {
	try {
		const meta = await lstat(path);
		return meta.isFile();
	} catch {
		return false;
	}
};

/** Where the symlink at `path` points, or null when it is not a link. */
export const linkTarget = async (path: string): Promise<string | null> => {
	try {
		return await readlink(path);
	} catch {
		return null;
	}
};

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

/** Directories no check reads: dependencies, build output, and agent worktrees. */
const SKIPPED = new Set([
	".build",
	".claude",
	".git",
	".gradle",
	"build",
	"node_modules",
	"target",
]);

/**
 * The files under `root` that `glob` matches, without the ones git ignores
 * and those in {@link SKIPPED} directories.
 */
export const listFiles = async (
	root: string,
	glob: Glob,
): Promise<string[]> => {
	const git = await $`git ls-files --cached --others --exclude-standard -z`
		.cwd(root)
		.nothrow()
		.quiet();
	const listed =
		git.exitCode === 0
			? git.stdout.toString().split("\0")
			: await Array.fromAsync(glob.scan({ cwd: root, dot: true }));
	const candidates = listed.filter(
		(path) =>
			path !== "" &&
			glob.match(path) &&
			!path.split("/").some((part) => SKIPPED.has(part)),
	);
	// The index still lists a file deleted since the last commit.
	const present = await Promise.all(
		candidates.map((path) => isRegularFile(join(root, path))),
	);
	return candidates.filter((_, index) => present[index] === true);
};
