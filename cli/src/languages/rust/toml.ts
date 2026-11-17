/**
 * Line-oriented TOML editing that keeps comments and formatting. A file splits
 * into sections, and a section into units: one `key = value` entry (however
 * many lines its value spans), one comment line, or one blank line.
 */

/** One table of a TOML file with the comments that introduce it. */
export interface Section {
	/** Dotted table name, such as `workspace.lints.rust`; undefined before the first header. */
	readonly name: string | undefined;
	/** Comment lines directly above the header. */
	readonly leading: readonly string[];
	/** The header line, or "" for the text before the first header. */
	readonly header: string;
	readonly units: readonly string[];
}

interface ScanState {
	readonly depth: number;
	/** The delimiter of a multi-line string that is still open. */
	readonly multi: string | undefined;
}

/** The index after the quoted string that starts at `from`. */
const skipQuoted = (line: string, from: number): number => {
	const quote = line[from];
	let at = from + 1;
	while (at < line.length && line[at] !== quote) {
		at += quote === '"' && line[at] === "\\" ? 2 : 1;
	}
	return at + 1;
};

const scanLine = (line: string, from: ScanState): ScanState => {
	let { depth, multi } = from;
	let at = 0;
	while (at < line.length) {
		const char = line[at];
		const triple = line.slice(at, at + 3);
		if (multi !== undefined) {
			const close = line.indexOf(multi, at);
			at = close === -1 ? line.length : close + 3;
			multi = close === -1 ? multi : undefined;
		} else if (char === "#") {
			break;
		} else if (triple === '"""' || triple === "'''") {
			multi = triple;
			at += 3;
		} else if (char === '"' || char === "'") {
			at = skipQuoted(line, at);
		} else {
			depth += "[{".includes(char ?? "") ? 1 : 0;
			depth -= "]}".includes(char ?? "") ? 1 : 0;
			at += 1;
		}
	}
	return { depth, multi };
};

/** Groups physical lines into units that start outside any array, table, or string. */
const toUnits = (text: string): string[] => {
	const lines = text.split(/(?<=\n)/u);
	const units: string[] = [];
	let state: ScanState = { depth: 0, multi: undefined };
	let open = false;
	for (const line of lines) {
		if (open) {
			units[units.length - 1] += line;
		} else {
			units.push(line);
		}
		state = scanLine(line, state);
		open = state.depth > 0 || state.multi !== undefined;
	}
	return units;
};

const isHeader = (unit: string): boolean => unit.trimStart().startsWith("[");

const isComment = (unit: string): boolean => unit.trimStart().startsWith("#");

/** `workspace.lints."a b"` from the header line `[workspace.lints."a b"]`. */
const tableName = (header: string): string => {
	const inner = header
		.trim()
		.replaceAll(/\s+#.*$/gu, "")
		.replaceAll(/^\[\[?|\]\]?$/gu, "");
	const parts = inner.match(/"[^"]*"|'[^']*'|[^.\s"']+/gu) ?? [];
	return parts.map((part) => part.replaceAll(/^["']|["']$/gu, "")).join(".");
};

/** Splits TOML text into sections; the first holds what precedes any header. */
export const parseSections = (text: string): Section[] => {
	const sections: {
		name: string | undefined;
		leading: string[];
		header: string;
		units: string[];
	}[] = [{ name: undefined, leading: [], header: "", units: [] }];
	for (const unit of toUnits(text)) {
		if (isHeader(unit)) {
			const previous = sections.at(-1);
			const leading: string[] = [];
			while (isComment(previous?.units.at(-1) ?? "")) {
				leading.unshift(previous?.units.pop() ?? "");
			}
			sections.push({
				name: tableName(unit),
				leading,
				header: unit,
				units: [],
			});
		} else {
			sections.at(-1)?.units.push(unit);
		}
	}
	return sections;
};

/** The text of `sections`, with a newline after every unit but possibly the last. */
export const renderSections = (sections: readonly Section[]): string => {
	const parts = sections.flatMap((section) => [
		...section.leading,
		...(section.header === "" ? [] : [section.header]),
		...section.units,
	]);
	return parts
		.map((part, index) =>
			part.endsWith("\n") || index === parts.length - 1 ? part : `${part}\n`,
		)
		.join("");
};

/** `section` with other units. */
export const withUnits = (
	section: Section,
	units: readonly string[],
): Section => ({
	name: section.name,
	leading: section.leading,
	header: section.header,
	units,
});

/** The key at the start of an entry, such as `edition.workspace`, or undefined for comments and blanks. */
export const keyOf = (unit: string): string | undefined => {
	const match =
		/^\s*((?:"[^"]*"|'[^']*'|[\w-]+)(?:\s*\.\s*(?:"[^"]*"|'[^']*'|[\w-]+))*)\s*=/u.exec(
			unit,
		);
	return match?.[1]?.replaceAll(/\s*\.\s*/gu, ".").replaceAll(/["']/gu, "");
};

const isBlank = (unit: string): boolean => unit.trim() === "";

/** Whether the section holds no entry, only comments and blank lines. */
export const hasNoEntries = (section: Section): boolean =>
	section.units.every((unit) => keyOf(unit) === undefined);

/** `section` without the entries whose key `drop` accepts. */
export const withoutKeys = (
	section: Section,
	drop: (key: string) => boolean,
): Section => ({
	...section,
	units: section.units.filter((unit) => {
		const key = keyOf(unit);
		return key === undefined || !drop(key);
	}),
});

/** `section` with `key = value`: replaced in place, else after the last entry. */
export const withKey = (
	section: Section,
	key: string,
	value: string,
): Section => {
	const line = `${key} = ${value}\n`;
	const at = section.units.findIndex((unit) => keyOf(unit) === key);
	if (at !== -1) {
		return { ...section, units: section.units.with(at, line) };
	}
	const last = section.units.findLastIndex((unit) => keyOf(unit) !== undefined);
	return { ...section, units: section.units.toSpliced(last + 1, 0, line) };
};

/** A section built from a header name and entry lines. */
export const makeSection = (
	name: string,
	entries: readonly string[],
): Section => ({
	name,
	leading: [],
	header: `[${name}]\n`,
	units: entries.map((entry) => `${entry}\n`),
});

/**
 * `sections` with one blank line between neighbors and none at the end, which
 * keeps an insert or removal from leaving gaps or piling blank lines up.
 */
export const tidy = (sections: readonly Section[]): Section[] =>
	sections.map((section, index) => {
		const units = [...section.units];
		while (isBlank(units.at(-1) ?? "x")) {
			units.pop();
		}
		const followed = index < sections.length - 1;
		const filled = section.header !== "" || units.length > 0;
		return {
			...section,
			units: [...units, ...(followed && filled ? ["\n"] : [])],
		};
	});

/** Removes every section for which `drop` is true; returns where the first one sat. */
export const removeSections = (
	sections: readonly Section[],
	drop: (name: string) => boolean,
): { sections: Section[]; at: number | undefined } => {
	const first = sections.findIndex(
		(section) => section.name !== undefined && drop(section.name),
	);
	return {
		sections: sections.filter(
			(section) => section.name === undefined || !drop(section.name),
		),
		at: first === -1 ? undefined : first,
	};
};

/** The section called `name`, if the file has one. */
export const findSection = (
	sections: readonly Section[],
	name: string,
): Section | undefined => sections.find((section) => section.name === name);

/** `sections` with `section` replacing the one of the same name, or appended. */
export const upsertSection = (
	sections: readonly Section[],
	section: Section,
): Section[] => {
	const at = sections.findIndex((entry) => entry.name === section.name);
	return at === -1 ? [...sections, section] : sections.with(at, section);
};
