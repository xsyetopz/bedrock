import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { $ } from "bun";

/** A fresh git repository in the temp directory, with `files` committed. */
export const tempRepo = async (
	files: Readonly<Record<string, string>> = {},
): Promise<string> => {
	const root = await mkdtemp(join(tmpdir(), "bedrock-test-"));
	await $`git init -q`.cwd(root).quiet();
	await Promise.all(
		Object.entries(files).map(([path, content]) =>
			Bun.write(join(root, path), content),
		),
	);
	await $`git add -A`.cwd(root).quiet();
	await $`git -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q --allow-empty -m init --no-verify`
		.cwd(root)
		.quiet();
	return root;
};
