export type Path = readonly (string | number)[];

export interface Entry {
	readonly path: Path;
	readonly key: string;
	/** The first line of the comments right above the key. */
	readonly from: number;
	readonly start: number;
	readonly end: number;
}

export interface Section {
	readonly path: Path;
	readonly array: boolean;
	readonly from: number;
	readonly entries: Entry[];
	end: number;
}

const HEADER = /^\s*(\[\[?)\s*([^\]]+?)\s*\]\]?\s*(?:#.*)?$/u;
const KEY = /^\s*([^\s#=[][^=]*?)\s*=/u;
export const COMMENT = /^\s*#/u;

const keyPath = (text: string): string[] =>
	text.split(".").map((part) => part.trim().replace(/^(["'])(.*)\1$/u, "$2"));

/** How many arrays the line opens minus how many it closes, outside strings and comments. */
const brackets = (line: string): number => {
	let depth = 0;
	let quote = "";
	for (const char of line) {
		if (quote !== "") {
			quote = char === quote ? "" : quote;
		} else if (char === '"' || char === "'") {
			quote = char;
		} else if (char === "#") {
			break;
		} else {
			depth += Number(char === "[") - Number(char === "]");
		}
	}
	return depth;
};

/** The last line of the value that starts on `start`. */
const valueEnd = (
	lines: readonly string[],
	start: number,
	key: string,
): number => {
	let end = start;
	let depth = brackets((lines[start] ?? "").slice(key.length));
	while (depth > 0 && end + 1 < lines.length) {
		end += 1;
		depth += brackets(lines[end] ?? "");
	}
	return end;
};

const startSection = (
	header: RegExpExecArray,
	from: number,
	start: number,
	counts: Map<string, number>,
): Section => {
	const name = header[2] ?? "";
	const array = header[1] === "[[";
	const count = counts.get(name) ?? 0;
	counts.set(name, count + 1);
	return {
		path: array ? [...keyPath(name), count] : keyPath(name),
		array,
		from,
		entries: [],
		end: start,
	};
};

/** Splits a TOML file into its root and its tables, each with the line range of every key. */
export const scan = (lines: readonly string[]): Section[] => {
	let section: Section = {
		path: [],
		array: false,
		from: 0,
		entries: [],
		end: -1,
	};
	const sections = [section];
	const counts = new Map<string, number>();
	let from = -1;
	for (let index = 0; index < lines.length; index += 1) {
		const line = lines[index] ?? "";
		if (COMMENT.test(line)) {
			from = from < 0 ? index : from;
			continue;
		}
		const header = HEADER.exec(line);
		const key = KEY.exec(line);
		if (header !== null) {
			section = startSection(header, from < 0 ? index : from, index, counts);
			sections.push(section);
		} else if (key !== null) {
			const end = valueEnd(lines, index, key[0]);
			section.entries.push({
				path: [...section.path, ...keyPath(key[1] ?? "")],
				key: key[1] ?? "",
				from: from < 0 ? index : from,
				start: index,
				end,
			});
			section.end = end;
			index = end;
		}
		from = -1;
	}
	return sections;
};
