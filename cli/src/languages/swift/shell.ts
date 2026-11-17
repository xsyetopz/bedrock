/** What a finished child process printed, with its exit code. */
export interface Run {
	readonly code: number;
	readonly stdout: string;
	readonly stderr: string;
}

/**
 * The environment for SwiftPM and Xcode tools. A git hook in a worktree sets
 * GIT_DIR and its siblings, which send SwiftPM's own git checkouts to the wrong
 * repository, so they are dropped.
 */
export const toolEnv = (): Record<string, string> =>
	Object.fromEntries(
		Object.entries(process.env).flatMap(([key, value]) =>
			value === undefined ||
			["GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"].includes(key)
				? []
				: [[key, value]],
		),
	);

/** Runs a command in `cwd` and captures its output; a missing executable is code 127. */
export const run = async (
	command: readonly string[],
	cwd: string,
): Promise<Run> => {
	try {
		const child = Bun.spawn([...command], {
			cwd,
			env: toolEnv(),
			stdout: "pipe",
			stderr: "pipe",
		});
		const [stdout, stderr, code] = await Promise.all([
			new Response(child.stdout).text(),
			new Response(child.stderr).text(),
			child.exited,
		]);
		return { code, stdout, stderr };
	} catch (error) {
		return {
			code: 127,
			stdout: "",
			stderr: error instanceof Error ? error.message : String(error),
		};
	}
};

/** The first non-blank line of a tool's error output, for a finding. */
export const firstLine = (text: string): string =>
	text
		.split("\n")
		.map((line) => line.trim())
		.find((line) => line !== "") ?? "no output";
