import { availableParallelism } from "node:os";

/** The lizard release bedrock-lizard runs. */
export const LIZARD_VERSION = "1.26.1";

/** One function as lizard measures it. */
export interface FunctionMeasure {
	readonly path: string;
	readonly line: number;
	readonly name: string;
	readonly nloc: number;
	readonly ccn: number;
	readonly params: number;
}

/** One file as lizard measures it: its code lines, without blanks and comments. */
export interface FileMeasure {
	readonly path: string;
	readonly nloc: number;
}

/** Ways to run the pinned lizard, tried in order. A lizard on PATH counts only at that version. */
const RUNNERS: readonly (readonly string[])[] = [
	["lizard"],
	["uvx", "--from", `lizard==${LIZARD_VERSION}`, "lizard"],
	["pipx", "run", "--spec", `lizard==${LIZARD_VERSION}`, "lizard"]
];

const stdoutOf = async (
	argv: readonly string[],
	cwd?: string
): Promise<{
	readonly ok: boolean;
	readonly stdout: string;
	readonly stderr: string;
}> => {
	const child = Bun.spawn([...argv], {
		...(cwd === undefined ? {} : { cwd }),
		stdout: "pipe",
		stderr: "pipe"
	});
	const [stdout, stderr, code] = await Promise.all([
		new Response(child.stdout).text(),
		new Response(child.stderr).text(),
		child.exited
	]);
	return { ok: code === 0, stdout, stderr };
};

/** The command that runs lizard {@link LIZARD_VERSION}, or undefined when no runner is installed. */
export const findLizard = async (): Promise<readonly string[] | undefined> => {
	const usable = await Promise.all(
		RUNNERS.map(async (runner) => {
			if (Bun.which(runner[0] ?? "") === null) {
				return false;
			}
			if (runner.length > 1) {
				return true;
			}
			const { stdout } = await stdoutOf([...runner, "--version"]);
			return stdout.trim() === LIZARD_VERSION;
		})
	);
	return RUNNERS.find((_, index) => usable[index] === true);
};

/** Splits one CSV record, as Python's csv module writes it. */
const csvFields = (line: string): string[] => {
	const fields: string[] = [];
	let field = "";
	let quoted = false;
	for (let index = 0; index < line.length; index += 1) {
		const char = line.charAt(index);
		if (quoted && char === '"' && line.charAt(index + 1) === '"') {
			field += '"';
			index += 1;
		} else if (char === '"') {
			quoted = !quoted;
		} else if (char === "," && !quoted) {
			fields.push(field);
			field = "";
		} else {
			field += char;
		}
	}
	fields.push(field);
	return fields;
};

const relative = (path: string): string => path.replace(/^\.\//u, "");

/**
 * The functions in lizard's `--csv` output, whose columns are nloc, CCN,
 * tokens, parameters, length, location, file, name, long name, start, end.
 */
export const parseFunctions = (csv: string): FunctionMeasure[] =>
	csv
		.split("\n")
		.filter((line) => line.trim() !== "")
		.map((line) => {
			const fields = csvFields(line);
			const at = (column: number): string => fields[column] ?? "";
			return {
				path: relative(at(6)),
				line: Number(at(9)),
				name: at(7),
				nloc: Number(at(0)),
				ccn: Number(at(1)),
				params: Number(at(3))
			};
		});

const unescapeXml = (text: string): string =>
	text
		.replaceAll("&lt;", "<")
		.replaceAll("&gt;", ">")
		.replaceAll("&quot;", '"')
		.replaceAll("&apos;", "'")
		.replaceAll("&amp;", "&");

/**
 * The files in lizard's `--xml` output: each `<item>` of the File measure,
 * whose values are its number, NCSS (code lines), CCN, and function count.
 */
export const parseFiles = (xml: string): FileMeasure[] => {
	const section =
		/<measure type="File">([\s\S]*?)<\/measure>/u.exec(xml)?.[1] ?? "";
	return [
		...section.matchAll(
			/<item name="([^"]*)">\s*<value>[^<]*<\/value>\s*<value>(\d+)<\/value>/gu
		)
	].map(([, name = "", nloc = "0"]) => ({
		path: relative(unescapeXml(name)),
		nloc: Number(nloc)
	}));
};

/** What lizard measures under `root` for `languages`. */
export interface Measures {
	readonly functions: readonly FunctionMeasure[];
	readonly files: readonly FileMeasure[];
}

/** Runs `lizard` over `root` once for the functions and once for the files. */
export const measure = async (
	lizard: readonly string[],
	root: string,
	languages: readonly string[]
): Promise<Measures> => {
	const args = [
		...languages.flatMap((language) => ["-l", language]),
		// bedrock applies the bounds itself, so lizard's own thresholds must not fail the run.
		"--ignore_warnings",
		"-1",
		"-t",
		String(availableParallelism())
	];
	const [csv, xml] = await Promise.all(
		["--csv", "--xml"].map((format) =>
			stdoutOf([...lizard, ...args, format, "."], root)
		)
	);
	const failed = [csv, xml].find((result) => result?.ok !== true);
	if (failed !== undefined) {
		throw new Error(`lizard failed: ${failed.stderr.trim()}`);
	}
	return {
		functions: parseFunctions(csv?.stdout ?? ""),
		files: parseFiles(xml?.stdout ?? "")
	};
};
