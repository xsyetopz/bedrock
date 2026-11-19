import { basename, dirname, extname, join } from "node:path";

import { exists } from "../../core/fs.ts";
import { type Token, tokens } from "./lexer.ts";

/** A `mod name;` or `mod name { … }` declaration. */
export interface Module {
	/** The module name without any `r#` prefix. */
	readonly name: string;
	readonly line: number;
	/** The `#[path = "…"]` value, if the declaration has one. */
	readonly path: string | undefined;
	/** Offsets of that `#[path = "…"]` attribute. */
	readonly pathAttribute: readonly [number, number] | undefined;
	/** Whether a `#[cfg(test)]`-style attribute gates it. */
	readonly testOnly: boolean;
	/** Offset of `mod`. */
	readonly start: number;
	/** Offsets of the `{` and `}` of an inline body, or `undefined` for `mod name;`. */
	readonly body: readonly [number, number] | undefined;
	/** How many braces enclose the declaration. */
	readonly depth: number;
}

/** The index of the bracket that closes the `[` or `(` at `openAt`. */
export const closing = (
	all: readonly Token[],
	openAt: number
): number | undefined => {
	const open = all[openAt]?.text;
	const close = open === "[" ? "]" : ")";
	if (open !== "[" && open !== "(") {
		return undefined;
	}
	let depth = 0;
	for (let index = openAt; index < all.length; index += 1) {
		const text = all[index]?.text;
		depth += text === open ? 1 : 0;
		depth -= text === close ? 1 : 0;
		if (depth === 0) {
			return index;
		}
	}
	return undefined;
};

/** Splits the tokens inside `( … )` at top-level commas. */
const splitArguments = (group: readonly Token[]): Token[][] => {
	const parts: Token[][] = [[]];
	let depth = 0;
	for (const token of group) {
		depth += token.text === "(" ? 1 : 0;
		depth -= token.text === ")" ? 1 : 0;
		if (token.text === "," && depth === 0) {
			parts.push([]);
		} else {
			parts.at(-1)?.push(token);
		}
	}
	return parts.filter((part) => part.length > 0);
};

/**
 * Whether the cfg predicate holds only when compiling tests: `all` needs one
 * such part, `any` needs every part to be one, and `not` never is.
 */
const impliesTest = (predicate: readonly Token[]): boolean => {
	const [word, open, ...rest] = predicate;
	if (predicate.length === 1) {
		return word?.text === "test";
	}
	if (open?.text !== "(" || rest.at(-1)?.text !== ")") {
		return false;
	}
	const parts = splitArguments(rest.slice(0, -1));
	if (word?.text === "all") {
		return parts.some((part) => impliesTest(part));
	}
	return (
		word?.text === "any" &&
		parts.length > 0 &&
		parts.every((part) => impliesTest(part))
	);
};

const isTestCfg = (inner: readonly Token[]): boolean =>
	inner[0]?.text === "cfg" &&
	inner[1]?.text === "(" &&
	inner.at(-1)?.text === ")" &&
	impliesTest(inner.slice(2, -1));

const pathValue = (inner: readonly Token[]): string | undefined => {
	const [name, equals, value] = inner;
	return inner.length === 3 &&
		name?.text === "path" &&
		equals?.text === "=" &&
		value?.kind === "literal" &&
		value.text.startsWith('"')
		? value.text.slice(1, -1)
		: undefined;
};

/** The attributes in front of an item, and the index of the item's first token. */
const attributes = (
	all: readonly Token[],
	from: number
): Pick<Module, "testOnly" | "path" | "pathAttribute"> & {
	next: number;
} => {
	let testOnly = false;
	let path: string | undefined;
	let pathAttribute: [number, number] | undefined;
	let cursor = from;
	while (all[cursor]?.text === "#") {
		const end = closing(all, cursor + 1);
		const hash = all[cursor];
		const last = end === undefined ? undefined : all[end];
		if (end === undefined || hash === undefined || last === undefined) {
			break;
		}
		const inner = all.slice(cursor + 2, end);
		testOnly ||= isTestCfg(inner);
		const found = pathValue(inner);
		if (path === undefined && found !== undefined) {
			path = found;
			pathAttribute = [hash.start, last.end];
		}
		cursor = end + 1;
	}
	return { testOnly, path, pathAttribute, next: cursor };
};

/** The index after `pub`, `pub(crate)`, and `unsafe` modifiers. */
const skipModifiers = (all: readonly Token[], from: number): number => {
	let cursor = from;
	if (all[cursor]?.text === "pub") {
		cursor += 1;
		const end = closing(all, cursor);
		cursor = end === undefined ? cursor : end + 1;
	}
	return all[cursor]?.text === "unsafe" ? cursor + 1 : cursor;
};

/** The index of the `}` that closes the `{` at `openAt`. */
const bracesEnd = (
	all: readonly Token[],
	openAt: number
): number | undefined => {
	let depth = 0;
	for (let index = openAt; index < all.length; index += 1) {
		const text = all[index]?.text;
		depth += text === "{" ? 1 : 0;
		depth -= text === "}" ? 1 : 0;
		if (depth === 0) {
			return index;
		}
	}
	return undefined;
};

/** How many braces enclose each token. */
const depths = (all: readonly Token[]): number[] => {
	let depth = 0;
	return all.map((token) => {
		depth = token.text === "}" ? Math.max(0, depth - 1) : depth;
		const here = depth;
		depth += token.text === "{" ? 1 : 0;
		return here;
	});
};

/** Every `mod name;` and `mod name { … }` in `source`, in order, outside comments and strings. */
export const modules = (source: string): Module[] => {
	const all = tokens(source);
	const enclosing = depths(all);
	const found: Module[] = [];
	let at = 0;
	while (at < all.length) {
		const attrs = attributes(all, at);
		const cursor = skipModifiers(all, attrs.next);
		const [keyword, name, after] = all.slice(cursor, cursor + 3);
		const end = after?.text === "{" ? bracesEnd(all, cursor + 2) : undefined;
		const close = end === undefined ? undefined : all[end];
		const isMod = keyword?.text === "mod" && name?.kind === "ident";
		if (
			isMod &&
			after !== undefined &&
			(after.text === ";" || close !== undefined)
		) {
			found.push({
				name: name.text.replace(/^r#/u, ""),
				line: keyword.line,
				path: attrs.path,
				pathAttribute: attrs.pathAttribute,
				testOnly: attrs.testOnly,
				start: keyword.start,
				body: close === undefined ? undefined : [after.start, close.start],
				depth: enclosing[cursor] ?? 0
			});
			at = cursor + 3;
		} else {
			at = Math.max(cursor, at + 1);
		}
	}
	return found;
};

/**
 * Whether `file` is a crate root or a `mod.rs`, whose child modules sit beside
 * it rather than in a directory named after it. A target root counts too:
 * `tests/x.rs`, `examples/x.rs`, `benches/x.rs`, or `src/bin/x.rs` in a
 * package. A directory merely named `tests` inside `src/` is an ordinary
 * module directory.
 */
const ownsDirectory = async (file: string): Promise<boolean> => {
	if (["lib.rs", "main.rs", "mod.rs", "build.rs"].includes(basename(file))) {
		return true;
	}
	const parent = dirname(file);
	const kind = basename(parent);
	const targets: Record<string, string> = {
		tests: dirname(parent),
		examples: dirname(parent),
		benches: dirname(parent),
		bin: dirname(dirname(parent))
	};
	const owner = targets[kind];
	return owner !== undefined && (await exists(join(owner, "Cargo.toml")));
};

/**
 * The directory where the child modules of `file` sit: beside a `mod.rs`,
 * `lib.rs`, or `main.rs`, and in a directory named for the file otherwise.
 */
export const moduleDirectory = async (file: string): Promise<string> =>
	(await ownsDirectory(file))
		? dirname(file)
		: join(dirname(file), basename(file, extname(file)));

/** The file that holds the body of `declaration`, declared in `file`. */
export const childFile = async (
	file: string,
	declaration: Module
): Promise<string | undefined> => {
	if (declaration.path !== undefined) {
		const candidate = join(dirname(file), declaration.path);
		return (await exists(candidate)) ? candidate : undefined;
	}
	const base = await moduleDirectory(file);
	const candidates = [
		join(base, `${declaration.name}.rs`),
		join(base, declaration.name, "mod.rs")
	];
	const present = await Promise.all(candidates.map((path) => exists(path)));
	return candidates.find((_, index) => present[index] === true);
};
