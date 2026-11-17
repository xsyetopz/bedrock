import { rm } from "node:fs/promises";
import { join } from "node:path";
import { parseArgs } from "node:util";

import { Glob } from "bun";

import { heldByGit, isRepository } from "../core/git.ts";
import { resolveRoot } from "./select.ts";

const USAGE = `Usage: bedrock clean [<dir>] [--force]

Deletes the *.bak files that bedrock releases before 0.2 left beside each
file they replaced. A backup whose content git already holds goes without
asking. One git does not hold is listed, and goes only with --force.`;

/** Directories that hold dependencies or build output, never a backup to review. */
const SKIPPED =
	/(?:^|\/)(?:\.build|\.git|\.gradle|build|node_modules|target)\//u;

/** The `*.bak` files under `root`, including the ones git ignores. */
export const findBackups = async (root: string): Promise<string[]> => {
	const found = await Array.fromAsync(
		new Glob("**/*.bak").scan({
			cwd: root,
			dot: true,
			onlyFiles: true,
			followSymlinks: false,
		}),
	);
	return found.filter((path) => !SKIPPED.test(path)).toSorted();
};

/** What a cleanup removed and what it left for a person to review. */
export interface Cleanup {
	readonly removed: readonly string[];
	readonly kept: readonly string[];
}

/** Deletes each backup git can restore, and each other one when `force` is set. */
export const removeBackups = async (
	root: string,
	force: boolean,
): Promise<Cleanup> => {
	const backups = await findBackups(root);
	const held = (await isRepository(root))
		? await heldByGit(root, backups)
		: new Set<string>();
	const removed = backups.filter((path) => force || held.has(path));
	await Promise.all(
		removed.map((path) => rm(join(root, path), { force: true })),
	);
	return { removed, kept: backups.filter((path) => !removed.includes(path)) };
};

/** Prints a cleanup's outcome. */
export const reportCleanup = ({ removed, kept }: Cleanup): void => {
	for (const path of removed) {
		console.info(`  delete  ${path}`);
	}
	for (const path of kept) {
		console.info(
			`  keep    ${path}: git does not hold this content. Review it, then run \`bedrock clean --force\`.`,
		);
	}
};

/** `bedrock clean`. */
export const clean = async (args: readonly string[]): Promise<number> => {
	const { values, positionals } = parseArgs({
		args: [...args],
		allowPositionals: true,
		options: {
			force: { type: "boolean", default: false },
			help: { type: "boolean", short: "h", default: false },
		},
	});
	if (values.help) {
		console.info(USAGE);
		return 0;
	}
	const result = await removeBackups(resolveRoot(positionals[0]), values.force);
	if (result.removed.length === 0 && result.kept.length === 0) {
		console.info("No *.bak files.");
	}
	reportCleanup(result);
	return result.kept.length === 0 ? 0 : 1;
};
