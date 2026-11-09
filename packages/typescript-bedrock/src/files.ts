import { join } from "node:path";

import { $, type Glob } from "bun";

/** The files under `cwd` that match `glob`, without the ones git ignores and without node_modules. */
export const listFiles = async (cwd: string, glob: Glob): Promise<string[]> => {
	const git = await $`git ls-files --cached --others --exclude-standard`
		.cwd(cwd)
		.nothrow()
		.quiet();
	const listed = (
		git.exitCode === 0
			? git.stdout.toString().split("\n")
			: await Array.fromAsync(glob.scan({ cwd }))
	).filter(
		(path) => glob.match(path) && !path.split("/").includes("node_modules"),
	);
	// The index still lists a file deleted since the last commit.
	const present = await Promise.all(
		listed.map((path) => Bun.file(join(cwd, path)).exists()),
	);
	return listed.filter((_, index) => present[index] === true);
};
