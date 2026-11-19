import { join } from "node:path";

import { $ } from "bun";

import { readText } from "../../core/fs.ts";

/** Flags that pick an output target, in the long and the older one-dash form. */
const TARGETS = new Set([
	"js",
	"lua",
	"swf",
	"neko",
	"php",
	"cpp",
	"cppia",
	"cs",
	"java",
	"jvm",
	"python",
	"hl",
	"interp",
	"run",
	"x"
]);

/** Flags after which a compilation runs something: the interpreter, or `--cmd`. */
const RUNS = new Set(["interp", "run", "x", "cmd"]);

/** What an hxml file, with the hxml files it includes, asks of the compiler. */
export interface Hxml {
	/** Whether it names an output target, so it compiles as written. */
	readonly target: boolean;
	/** Whether a compilation of it runs a program: `--interp`, `--run`, or `--cmd`. */
	readonly runs: boolean;
}

/**
 * The lines of an hxml text that carry arguments: trimmed, without blanks and
 * `#` comments. Each is a flag with its value, such as `-cp src`, or a bare
 * argument, such as a class path or an included hxml file.
 */
export const hxmlLines = (text: string): string[] =>
	text
		.split(/\r?\n/u)
		.map((line) => line.trim())
		.filter((line) => line !== "" && !line.startsWith("#"));

/** A line's flag without its dashes, such as `cp` for `-cp src`, or undefined for a bare argument. */
const flagOf = (line: string): string | undefined =>
	/^--?([\w-]+)/u.exec(line)?.[1];

/**
 * Reads the hxml file at `path`, relative to `root`, which the compiler runs
 * from. An included hxml file adds its flags, as the compiler reads it in
 * place; one already read, or missing, adds none.
 */
export const readHxml = async (
	root: string,
	path: string,
	seen: ReadonlySet<string> = new Set()
): Promise<Hxml> => {
	const lines = hxmlLines((await readText(join(root, path))) ?? "");
	const flags = lines.flatMap((line) => {
		const flag = flagOf(line);
		return flag === undefined ? [] : [flag];
	});
	const reading = new Set([...seen, path]);
	const included = await Promise.all(
		lines
			.filter(
				(line) =>
					flagOf(line) === undefined &&
					line.endsWith(".hxml") &&
					!reading.has(line)
			)
			.map((line) => readHxml(root, line, reading))
	);
	return {
		target:
			flags.some((flag) => TARGETS.has(flag)) ||
			included.some((hxml) => hxml.target),
		runs:
			flags.some((flag) => RUNS.has(flag)) || included.some((hxml) => hxml.runs)
	};
};

/** The hxml files of `root` git tracks or would track, outside the local haxelib repo. */
export const hxmlFiles = async (root: string): Promise<string[]> => {
	const listed =
		await $`git ls-files -z --cached --others --exclude-standard -- ${"*.hxml"}`
			.cwd(root)
			.nothrow()
			.quiet();
	return listed.stdout
		.toString()
		.split("\0")
		.filter((path) => path !== "" && !path.startsWith(".haxelib/"))
		.toSorted();
};

/** An hxml file of a repo, by its path from the root, with what it asks of the compiler. */
export interface HxmlBuild extends Hxml {
	readonly path: string;
}

/** Each hxml file of `root` with what it asks of the compiler. */
export const hxmlBuilds = async (root: string): Promise<HxmlBuild[]> => {
	const paths = await hxmlFiles(root);
	return Promise.all(
		paths.map(async (path) => {
			const hxml = await readHxml(root, path);
			return { path, target: hxml.target, runs: hxml.runs };
		})
	);
};

/** Whether `path` names a test build, such as `test.hxml`, `tests.hxml`, or `test-native.hxml`. */
export const isTestHxml = (path: string): boolean =>
	/^tests?\b/iu.test(path.split("/").at(-1) ?? "");
