import { rm } from "node:fs/promises";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import type { Context } from "../../core/language.ts";
import { inOrder } from "../../core/order.ts";
import { lockfilesIn } from "./legacy.ts";

/** Runs a command in `root` with the terminal attached, or throws `failure`. */
const runOrThrow = async (
	root: string,
	command: readonly string[],
	failure: string
): Promise<void> => {
	const child = Bun.spawn([...command], {
		cwd: root,
		stdin: "inherit",
		stdout: "inherit",
		stderr: "inherit"
	});
	if ((await child.exited) !== 0) {
		throw new Error(failure);
	}
};

/**
 * Installs the dependencies, which also migrates another package manager's
 * lockfile into `bun.lock`, then formats the tree. Those lockfiles go only
 * after `bun.lock` exists, so a failed install loses nothing. Formatting
 * here keeps the adopt commit apart from the code changes that follow it, and
 * lets `just verify` pass its format check on the first run.
 */
export const installDependencies = async ({ root }: Context): Promise<void> => {
	await runOrThrow(
		root,
		["bun", "install"],
		"bun install failed; fix it, then run `just setup`."
	);
	if (await exists(join(root, "bun.lock"))) {
		await inOrder(await lockfilesIn(root), (path) =>
			rm(join(root, path), { force: true, recursive: true })
		);
	}
	await runOrThrow(
		root,
		["bun", "fmt"],
		"bun fmt failed; fix what it reports, then run `just format`."
	);
};
