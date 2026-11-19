import { join } from "node:path";

import { readText } from "../fs.ts";
import { globMatcher } from "./glob.ts";

/** One `[glob]` section of an `.editorconfig`, with its pairs in file order. */
export interface Section {
	readonly glob: string;
	readonly pairs: readonly (readonly [string, string])[];
}

const SECTION = /^\[(.*)\]$/u;
const PAIR = /^([^=]+?)\s*=\s*(.*)$/u;

/**
 * The sections of an `.editorconfig`, as the EditorConfig spec reads them:
 * full-line `#` and `;` comments, keys lowercased, and the pairs before the
 * first section, such as `root`, dropped.
 */
export const parseEditorconfig = (text: string): Section[] => {
	const sections: { glob: string; pairs: [string, string][] }[] = [];
	for (const raw of text.split(/\r?\n/u)) {
		const line = raw.trim();
		const header = SECTION.exec(line);
		const pair = PAIR.exec(line);
		if (line === "" || line.startsWith("#") || line.startsWith(";")) {
			continue;
		}
		if (header !== null) {
			sections.push({ glob: header[1] ?? "", pairs: [] });
		} else if (pair !== null) {
			sections
				.at(-1)
				?.pairs.push([(pair[1] ?? "").toLowerCase(), pair[2] ?? ""]);
		}
	}
	return sections;
};

const matchers = new Map<string, (path: string) => boolean>();

const matches = (glob: string, path: string): boolean => {
	let matcher = matchers.get(glob);
	if (matcher === undefined) {
		matcher = globMatcher(glob);
		matchers.set(glob, matcher);
	}
	return matcher(path);
};

const apply = (
	sections: readonly Section[],
	path: string,
	properties: Map<string, string>,
): Map<string, string> => {
	for (const section of sections.filter(({ glob }) => matches(glob, path))) {
		for (const [key, value] of section.pairs) {
			if (value.toLowerCase() === "unset") {
				properties.delete(key);
			} else {
				properties.set(key, value);
			}
		}
	}
	return properties;
};

/**
 * The properties that apply to `path`, relative to the `.editorconfig`: each
 * matching section in order, a later pair overriding an earlier one, and
 * `unset` removing a pair.
 */
export const propertiesFor = (
	sections: readonly Section[],
	path: string,
): Map<string, string> => apply(sections, path, new Map());

/** One `.editorconfig` on disk. */
export interface EditorconfigFile {
	/** Its path, relative to the repo root. */
	readonly path: string;
	/** Its directory, relative to the repo root; `""` for the root itself. */
	readonly dir: string;
	/** Whether its preamble sets `root = true`, which ends the search upward. */
	readonly root: boolean;
	readonly sections: readonly Section[];
}

/** The `.editorconfig` files of one repo, read as the paths that need them ask. */
export interface EditorconfigTree {
	/** The properties of `path`, relative to the repo root. */
	readonly properties: (path: string) => Promise<Map<string, string>>;
	/** Every `.editorconfig` read so far, the root one always among them. */
	readonly files: () => Promise<EditorconfigFile[]>;
}

const declaresRoot = (text: string): boolean => {
	const preamble = text.split(/^\s*\[/mu, 1)[0] ?? "";
	return /^\s*root\s*=\s*true\s*$/imu.test(preamble);
};

/** The `.editorconfig` in `dir` under `root`, or undefined when it has none. */
const readEditorconfig = async (
	root: string,
	dir: string,
): Promise<EditorconfigFile | undefined> => {
	const path = dir === "" ? ".editorconfig" : `${dir}/.editorconfig`;
	const text = await readText(join(root, path));
	return text === undefined
		? undefined
		: {
				path,
				dir,
				root: declaresRoot(text),
				sections: parseEditorconfig(text),
			};
};

/** `""` and each directory above `path`, from the top down. */
const ancestors = (path: string): string[] => {
	const parts = path.split("/").slice(0, -1);
	return ["", ...parts.map((_, index) => parts.slice(0, index + 1).join("/"))];
};

const present = async (
	pending: readonly Promise<EditorconfigFile | undefined>[],
): Promise<EditorconfigFile[]> => {
	const files = await Promise.all(pending);
	return files.filter((file) => file !== undefined);
};

/** Applies `files`, from the nearest `root = true` down, to `path`. */
const cascade = (
	files: readonly EditorconfigFile[],
	path: string,
): Map<string, string> => {
	const properties = new Map<string, string>();
	const top = Math.max(
		files.findLastIndex((file) => file.root),
		0,
	);
	for (const file of files.slice(top)) {
		const relative = file.dir === "" ? path : path.slice(file.dir.length + 1);
		apply(file.sections, relative, properties);
	}
	return properties;
};

/**
 * The `.editorconfig` files under `root`, each read once. A path's
 * properties come from the files in its directory and each one above it, up
 * to the nearest that sets `root = true` or the repo root, applied from the
 * top down, so a nested file overrides the root one.
 */
export const editorconfigTree = (root: string): EditorconfigTree => {
	const loaded = new Map<string, Promise<EditorconfigFile | undefined>>();
	const load = (dir: string): Promise<EditorconfigFile | undefined> => {
		const cached = loaded.get(dir);
		if (cached !== undefined) {
			return cached;
		}
		const file = readEditorconfig(root, dir);
		loaded.set(dir, file);
		return file;
	};
	return {
		properties: async (path) => {
			const files = await present(ancestors(path).map((dir) => load(dir)));
			return cascade(files, path);
		},
		files: async () => {
			await load("");
			return present([...loaded.values()]);
		},
	};
};
