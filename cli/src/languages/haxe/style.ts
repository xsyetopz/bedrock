import { $ } from "bun";

import type { Subcommand } from "../../core/language.ts";
import { passes, run } from "../swift/shell.ts";

/**
 * The top directories, or top-level files, that hold the Haxe sources git
 * tracks or would track in `root`, as `-s` arguments for checkstyle and the
 * formatter. The local haxelib repo holds libraries, not sources.
 */
export const sourceRoots = async (root: string): Promise<string[]> => {
	const listed =
		await $`git ls-files -z --cached --others --exclude-standard -- ${"*.hx"}`
			.cwd(root)
			.nothrow()
			.quiet();
	const tops = listed.stdout
		.toString()
		.split("\0")
		.filter((path) => path !== "")
		.map((path) => path.split("/")[0] ?? path)
		.filter((top) => top !== ".haxelib");
	return [...new Set(tops)].toSorted();
};

/** `-s <root>` for each of `roots`. */
const sourceArgs = (roots: readonly string[]): string[] =>
	roots.flatMap((top) => ["-s", top]);

/**
 * The errors a checkstyle text report counts, from its summary line, such as
 * `Total Issues: 3 (Errors: 1, Warnings: 2, Infos: 0)`, or 0 for `No issues
 * found.`; undefined when it printed neither, so it did not finish.
 */
export const checkstyleErrors = (report: string): number | undefined => {
	const total = /Total Issues: \d+ \(Errors: (\d+)/u.exec(report);
	if (total !== null) {
		return Number(total[1]);
	}
	return report.includes("No issues found.") ? 0 : undefined;
};

/**
 * `bedrock haxe lint`: runs the pinned checkstyle over the Haxe sources with
 * `checkstyle.json`. It fails on an error-level finding; warnings only print.
 * The summary decides, since `--exitcode` gives the error count as the exit
 * status, which wraps to 0 at 256 errors.
 */
export const haxeLint: Subcommand = {
	summary: "Run checkstyle over the Haxe sources.",
	run: async ({ root }) => {
		const roots = await sourceRoots(root);
		if (roots.length === 0) {
			return 0;
		}
		const result = await run(
			[
				"haxelib",
				"run",
				"checkstyle",
				...sourceArgs(roots),
				"-c",
				"checkstyle.json",
				"--nostyle",
			],
			root,
		);
		process.stdout.write(result.stdout);
		process.stderr.write(result.stderr);
		const errors = checkstyleErrors(result.stdout);
		if (errors === undefined) {
			console.error(
				"checkstyle did not finish: run just haxe::setup, which installs it, then check checkstyle.json",
			);
			return 1;
		}
		return errors === 0 ? 0 : 1;
	},
};

/**
 * `bedrock haxe format [--check]`: runs the pinned formatter over the Haxe
 * sources, which reads `hxformat.json` where one sits; with `--check` it only
 * fails when a file is not formatted.
 */
export const haxeFormat: Subcommand = {
	args: "[--check]",
	summary: "Format the Haxe sources, or with --check fail if one is not.",
	run: async ({ root }, args) => {
		const roots = await sourceRoots(root);
		if (roots.length === 0) {
			return 0;
		}
		const check = args.includes("--check") ? ["--check"] : [];
		const formatted = await passes(root, [
			"haxelib",
			"run",
			"formatter",
			...sourceArgs(roots),
			...check,
		]);
		return formatted ? 0 : 1;
	},
};
