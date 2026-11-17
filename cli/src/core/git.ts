import { join } from "node:path";

import { $ } from "bun";

/** Whether `root` sits inside a git work tree. */
export const isRepository = async (root: string): Promise<boolean> => {
	const result = await $`git rev-parse --is-inside-work-tree`
		.cwd(root)
		.nothrow()
		.quiet();
	return result.exitCode === 0;
};

/** Trimmed stdout of a git command, or "" when it fails. */
const gitOutput = async (root: string, args: readonly string[]) => {
	const result = await $`git ${args}`.cwd(root).nothrow().quiet();
	return result.exitCode === 0 ? result.stdout.toString().trim() : "";
};

/**
 * Whether git runs the hooks in `.githooks`. A Claude Code worktree stores
 * `core.hooksPath` as an absolute path into the main checkout, so both the
 * main checkout's and this work tree's `.githooks` count.
 */
export const hooksOn = async (root: string): Promise<boolean> => {
	const [common, configured] = await Promise.all([
		gitOutput(root, [
			"rev-parse",
			"--path-format=absolute",
			"--git-common-dir",
		]),
		gitOutput(root, ["config", "core.hooksPath"]),
	]);
	if (common === "" || configured === "") {
		return false;
	}
	const main = join(common, "..");
	const resolved = configured.startsWith("/")
		? configured
		: join(main, configured);
	return (
		resolved === join(main, ".githooks") || resolved === join(root, ".githooks")
	);
};

/**
 * The paths among `paths` that hold work git does not have: modified,
 * staged, or untracked. Ignored files are build output and do not count.
 */
export const uncommitted = async (
	root: string,
	paths: readonly string[],
): Promise<string[]> => {
	if (paths.length === 0) {
		return [];
	}
	const status =
		await $`git status --porcelain=v1 -z --untracked-files=all -- ${paths}`
			.cwd(root)
			.quiet();
	return status.stdout
		.toString()
		.split("\0")
		.filter((entry) => entry.length > 3)
		.map((entry) => entry.slice(3))
		.toSorted();
};

/**
 * The files among `paths` whose exact content is a blob in the repository's
 * object store, so deleting them loses nothing git cannot restore.
 */
export const heldByGit = async (
	root: string,
	paths: readonly string[],
): Promise<Set<string>> => {
	if (paths.length === 0) {
		return new Set();
	}
	const hashed =
		await $`git hash-object --stdin-paths < ${new Response(paths.join("\n"))}`
			.cwd(root)
			.quiet();
	const hashes = hashed.stdout.toString().trim().split("\n");
	const checked =
		await $`git cat-file --batch-check < ${new Response(hashes.join("\n"))}`
			.cwd(root)
			.nothrow()
			.quiet();
	const lines = checked.stdout.toString().trim().split("\n");
	return new Set(
		paths.filter((_, index) => lines[index]?.endsWith(" missing") === false),
	);
};

/** The staged paths, added, copied, modified, or renamed. */
export const stagedPaths = async (root: string): Promise<string[]> => {
	const diff = await $`git diff --cached --name-only -z --diff-filter=ACMR`
		.cwd(root)
		.quiet();
	return diff.stdout
		.toString()
		.split("\0")
		.filter((path) => path !== "");
};
