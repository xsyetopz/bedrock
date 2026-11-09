import { isArray, isRecord } from "./json.ts";
import {
	COMMENT,
	type Entry,
	type Path,
	scan,
	type Section,
} from "./toml-scan.ts";

interface Merge {
	readonly lines: string[];
	readonly own: readonly string[];
	readonly base: unknown;
	readonly repo: unknown;
	readonly after: Map<number, string[]>;
	readonly appended: string[];
	readonly notes: string[];
}

const get = (value: unknown, path: Path): unknown => {
	let node = value;
	for (const part of path) {
		if (typeof part === "number") {
			node = isArray(node) ? node[part] : undefined;
		} else {
			node = isRecord(node) ? node[part] : undefined;
		}
	}
	return node;
};

const asList = (value: unknown): readonly unknown[] =>
	isArray(value) ? value : [];

const has = (list: readonly unknown[], value: unknown): boolean =>
	list.some((item) => Bun.deepEquals(item, value));

const toml = (value: unknown): string => JSON.stringify(value);

const valuesOn = (line: string): readonly unknown[] => {
	try {
		// The newline keeps a trailing comment from swallowing the `]`.
		return asList(get(Bun.TOML.parse(`v = [${line}\n]`), ["v"]));
	} catch {
		return [];
	}
};

const insert = (merge: Merge, line: number, text: readonly string[]): void => {
	if (text.length > 0) {
		merge.after.set(line, [...(merge.after.get(line) ?? []), ...text]);
	}
};

const raw = (merge: Merge, from: number, end: number): string =>
	merge.own.slice(from, end + 1).join("\n");

/** The repo's lines that hold an entry the template array lacks, with their comments. */
const carriedLines = (
	merge: Merge,
	entry: Entry,
	mine: readonly unknown[],
): { lines: string[]; values: unknown[] } => {
	const lines: string[] = [];
	const values: unknown[] = [];
	let comments: string[] = [];
	for (const line of merge.own.slice(entry.start + 1, entry.end)) {
		if (COMMENT.test(line)) {
			comments.push(line);
			continue;
		}
		const fresh = valuesOn(line).filter((value) => !has(mine, value));
		if (fresh.length > 0) {
			lines.push(...comments, line);
			values.push(...fresh);
		}
		comments = [];
	}
	return { lines, values };
};

const mergeArray = (
	merge: Merge,
	entry: Entry,
	at: Entry,
	mine: readonly unknown[],
): void => {
	const theirs = asList(get(merge.repo, entry.path));
	const extra = theirs.filter((value) => !has(mine, value));
	if (extra.length === 0) {
		return;
	}
	if (at.start === at.end) {
		const values = [...mine, ...extra].map((value) => toml(value));
		merge.lines[at.start] = `${at.key} = [${values.join(", ")}]`;
		return;
	}
	const indent = /^\s*/u.exec(merge.lines[at.end - 1] ?? "")?.[0] ?? "";
	const carried = carriedLines(merge, entry, mine);
	const rest = extra.filter((value) => !has(carried.values, value));
	insert(merge, at.end - 1, [
		...carried.lines,
		...rest.map((value) => `${indent}${toml(value)},`),
	]);
};

const mergeTable = (merge: Merge, section: Section, table: Section): void => {
	const keys = new Map(
		table.entries.map((entry) => [JSON.stringify(entry.path), entry]),
	);
	const added: string[] = [];
	for (const entry of section.entries) {
		const mine = get(merge.base, entry.path);
		const theirs = get(merge.repo, entry.path);
		const at = keys.get(JSON.stringify(entry.path));
		if (mine === undefined) {
			added.push(raw(merge, entry.from, entry.end));
		} else if (at !== undefined && isArray(mine) && isArray(theirs)) {
			mergeArray(merge, entry, at, mine);
		} else if (!Bun.deepEquals(mine, theirs)) {
			const name = entry.path.join(".");
			merge.notes.push(
				`bunfig.toml: kept the template's \`${name} = ${toml(mine)}\`; the repo had \`${name} = ${toml(theirs)}\`.`,
			);
		}
	}
	insert(merge, table.end, added);
};

const mergeSection = (
	merge: Merge,
	section: Section,
	tables: Map<string, Section>,
): void => {
	const table = tables.get(JSON.stringify(section.path));
	const isNew = section.array
		? !has(
				asList(get(merge.base, section.path.slice(0, -1))),
				get(merge.repo, section.path),
			)
		: table === undefined;
	if (isNew) {
		merge.appended.push(raw(merge, section.from, section.end));
	} else if (!section.array && table !== undefined) {
		mergeTable(merge, section, table);
	}
};

const render = (merge: Merge): string => {
	const out = [...(merge.after.get(-1) ?? [])];
	for (const [index, line] of merge.lines.entries()) {
		out.push(line, ...(merge.after.get(index) ?? []));
	}
	let text = out.join("\n").replace(/\n*$/u, "\n");
	for (const block of merge.appended) {
		text += `\n${block}\n`;
	}
	return text;
};

/**
 * Writes the template's bunfig.toml, then adds what the repo's copy adds to it:
 * each key a table lacks, each entry a template array lacks, and each table or
 * `[[…]]` element the template lacks, with the comments right above them.
 * Where both set a value, the template's wins, since bunfig.toml cannot
 * extend a shared file; each such value gets a note.
 */
export const mergeBunfig = (
	template: string,
	current?: string,
): { content: string; notes: string[] } => {
	if (current === undefined) {
		return { content: template, notes: [] };
	}
	let repo: unknown;
	try {
		repo = Bun.TOML.parse(current);
	} catch {
		return {
			content: template,
			notes: [
				"bunfig.toml: the repo's copy is not valid TOML, so adopt wrote the template. Move its settings over from bunfig.toml.bak.",
			],
		};
	}
	const merge: Merge = {
		lines: template.split("\n"),
		own: current.split("\n"),
		base: Bun.TOML.parse(template),
		repo,
		after: new Map(),
		appended: [],
		notes: [],
	};
	const tables = new Map(
		scan(merge.lines).map((section) => [JSON.stringify(section.path), section]),
	);
	for (const section of scan(merge.own)) {
		mergeSection(merge, section, tables);
	}
	return { content: render(merge), notes: merge.notes };
};
