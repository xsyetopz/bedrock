import { readdir, rmdir, unlink } from "node:fs/promises";
import { basename, dirname, extname, join } from "node:path";

import { exists } from "../../core/fs.ts";
import { inOrder } from "../../core/order.ts";
import { type Token, tokens } from "./lexer.ts";

/** A `mod name;` declaration, whose body lives in another file. */
export interface OutOfLine {
	/** The module name without any `r#` prefix. */
	readonly name: string;
	/** The module name as written. */
	readonly written: string;
	readonly line: number;
	/** The `#[path = "…"]` value, if the declaration has one. */
	readonly path: string | undefined;
	/** Offsets of that `#[path = "…"]` attribute, which an inline module drops. */
	readonly pathAttribute: readonly [number, number] | undefined;
	/** Whether a `#[cfg(test)]`-style attribute gates it. */
	readonly testOnly: boolean;
	/** Offsets from `mod` through `;`. */
	readonly span: readonly [number, number];
}

/** The index of the bracket that closes the `[` or `(` at `openAt`. */
export const closing = (
	all: readonly Token[],
	openAt: number,
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
	from: number,
): Pick<OutOfLine, "testOnly" | "path" | "pathAttribute"> & {
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

/** Every `mod name;` in `source`, outside comments and strings. */
export const declarations = (source: string): OutOfLine[] => {
	const all = tokens(source);
	const found: OutOfLine[] = [];
	let at = 0;
	while (at < all.length) {
		const attrs = attributes(all, at);
		const cursor = skipModifiers(all, attrs.next);
		const [keyword, name, semicolon] = all.slice(cursor, cursor + 3);
		if (
			keyword?.text === "mod" &&
			name?.kind === "ident" &&
			semicolon?.text === ";"
		) {
			found.push({
				name: name.text.replace(/^r#/u, ""),
				written: name.text,
				line: keyword.line,
				path: attrs.path,
				pathAttribute: attrs.pathAttribute,
				testOnly: attrs.testOnly,
				span: [keyword.start, semicolon.end],
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
 * it rather than in a directory named after it.
 */
const ownsDirectory = (file: string): boolean =>
	["lib.rs", "main.rs", "mod.rs", "build.rs"].includes(basename(file)) ||
	["bin", "tests", "examples", "benches"].includes(basename(dirname(file)));

/** The file that holds the body of `declaration`, declared in `file`. */
export const childFile = async (
	file: string,
	declaration: OutOfLine,
): Promise<string | undefined> => {
	const directory = dirname(file);
	if (declaration.path !== undefined) {
		const candidate = join(directory, declaration.path);
		return (await exists(candidate)) ? candidate : undefined;
	}
	const nested = join(directory, basename(file, extname(file)));
	const bases = ownsDirectory(file) ? [directory, nested] : [nested, directory];
	const candidates = bases.flatMap((base) => [
		join(base, `${declaration.name}.rs`),
		join(base, declaration.name, "mod.rs"),
	]);
	const present = await Promise.all(candidates.map((path) => exists(path)));
	return candidates.find((_, index) => present[index] === true);
};

/** Drops the `#[path]` attribute of a module that now has its body inline. */
const dropPathAttribute = (
	source: string,
	[start, end]: readonly [number, number],
): string => {
	const tail = source.slice(end);
	const blanks = /^[^\S\n]*\n?/u.exec(tail)?.[0].length ?? 0;
	return source.slice(0, start) + source.slice(end + blanks);
};

const removeWithEmptyParent = async (file: string): Promise<void> => {
	await unlink(file);
	const parent = dirname(file);
	const left = await readdir(parent);
	if (left.length === 0) {
		await rmdir(parent);
	}
};

/** A test module with the file that holds its body, or why it has none. */
type Loaded =
	| {
			readonly declaration: OutOfLine;
			readonly child: string;
			readonly body: string;
	  }
	| { readonly declaration: OutOfLine; readonly problem: string };

const load = async (file: string, declaration: OutOfLine): Promise<Loaded> => {
	const place = `${file}:${declaration.line}`;
	const child = await childFile(file, declaration);
	if (child === undefined) {
		return {
			declaration,
			problem: `${place}: cannot find the file for mod ${declaration.name}`,
		};
	}
	const body = await Bun.file(child).text();
	return declarations(body).length > 0
		? {
				declaration,
				problem: `${place}: ${child} declares its own out-of-line modules; move them first`,
			}
		: { declaration, child, body };
};

/** `source` with the module's body in place of its `mod name;`. */
const spliceInline = (
	source: string,
	found: OutOfLine,
	body: string,
): string => {
	const [from, to] = found.span;
	const inline = `mod ${found.written} {\n${body.trimEnd()}\n}`;
	const spliced = source.slice(0, from) + inline + source.slice(to);
	return found.pathAttribute === undefined
		? spliced
		: dropPathAttribute(spliced, found.pathAttribute);
};

/**
 * Moves every out-of-line test module of `file` inline as
 * `mod name { … }`, and deletes the files that held the bodies. Returns one
 * message per module it could not move.
 */
export const inlineTests = async (file: string): Promise<string[]> => {
	const original = await Bun.file(file).text();
	const wanted = declarations(original).filter((found) => found.testOnly);
	const loaded = await Promise.all(wanted.map((found) => load(file, found)));
	let source = original;
	const moved: string[] = [];
	const problems: string[] = [];
	// Last first, so the offsets of the earlier modules stay valid.
	for (const entry of loaded.toReversed()) {
		if ("problem" in entry) {
			problems.push(entry.problem);
		} else {
			source = spliceInline(source, entry.declaration, entry.body);
			moved.push(entry.child);
		}
	}
	if (moved.length > 0) {
		await Bun.write(file, source);
		await inOrder(moved, removeWithEmptyParent);
	}
	return problems.toReversed();
};
