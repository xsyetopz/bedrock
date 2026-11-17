import { rm } from "node:fs/promises";
import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import type { Context } from "../../core/language.ts";
import { inOrder } from "../../core/order.ts";
import { lockfilesIn } from "./legacy.ts";

/**
 * Installs the dependencies, which also migrates another package manager's
 * lockfile into `bun.lock`. Those lockfiles go only after `bun.lock` exists, so
 * a failed install loses nothing.
 */
export const installDependencies = async ({ root }: Context): Promise<void> => {
	const install = Bun.spawn(["bun", "install"], {
		cwd: root,
		stdin: "inherit",
		stdout: "inherit",
		stderr: "inherit",
	});
	if ((await install.exited) !== 0) {
		throw new Error("bun install failed; fix it, then run `just setup`.");
	}
	if (await exists(join(root, "bun.lock"))) {
		await inOrder(await lockfilesIn(root), (path) =>
			rm(join(root, path), { force: true, recursive: true }),
		);
	}
};
