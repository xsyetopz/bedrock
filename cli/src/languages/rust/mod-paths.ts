import { type Token, tokens } from "./lexer.ts";
import { closing } from "./modules.ts";

const INCLUDES = new Set(["include", "include_str", "include_bytes"]);

/** The literal of an `include!("…")`-style call whose name is at `index`. */
const includeLiteral = (
	all: readonly Token[],
	index: number,
): Token | undefined => {
	const opens = ["(", "[", "{"].includes(all[index + 2]?.text ?? "");
	const closes = [")", "]", "}"].includes(all[index + 4]?.text ?? "");
	const literal = all[index + 3];
	return opens && closes && literal?.kind === "literal" ? literal : undefined;
};

/** Whether the item an attribute ending at `end` decorates is an inline module. */
const precedesInlineModule = (all: readonly Token[], end: number): boolean =>
	all.slice(end + 1).find((token) => token.text === ";" || token.text === "{")
		?.text === "{";

/**
 * The relative path in a plain string literal: `undefined` for one with
 * escapes or that is no string, and `null` for an absolute one, which a
 * rename does not change.
 */
const relative = (literal: Token): string | null | undefined => {
	const { text } = literal;
	if (!(text.startsWith('"') && text.endsWith('"')) || text.includes("\\")) {
		return undefined;
	}
	return text.startsWith('"/') ? null : text.slice(1, -1);
};

/** The `#[path = "…"]` literal, at depth 0 only, of the attribute whose `#` is at `index`. */
const pathLiteral = (
	all: readonly Token[],
	index: number,
	depth: number,
): { end: number; literal?: Token | undefined } | undefined => {
	const open = index + (all[index + 1]?.text === "!" ? 2 : 1);
	if (all[open]?.text !== "[") {
		return { end: index };
	}
	const end = closing(all, open);
	if (end === undefined) {
		return undefined;
	}
	const inner = all.slice(open + 1, end);
	const names = inner.some(
		(part, at) => part.text === "path" && inner[at + 1]?.text === "=",
	);
	if (!names || depth > 0) {
		return { end };
	}
	return inner.length !== 3 || precedesInlineModule(all, end)
		? undefined
		: { end, literal: inner[2] };
};

/**
 * The path literal the token at `index` introduces, if any, and the index to
 * resume from; `undefined` when it names a path that cannot be rewritten.
 */
const literalAt = (
	all: readonly Token[],
	index: number,
	depth: number,
):
	| { literal?: Token | undefined; next: number; include?: boolean }
	| undefined => {
	const token = all[index];
	if (
		token?.kind === "ident" &&
		INCLUDES.has(token.text) &&
		all[index + 1]?.text === "!"
	) {
		const literal = includeLiteral(all, index);
		return literal === undefined
			? undefined
			: { literal, next: index, include: true };
	}
	if (token?.text === "#") {
		const attribute = pathLiteral(all, index, depth);
		return attribute === undefined
			? undefined
			: { literal: attribute.literal, next: attribute.end };
	}
	return { next: index };
};

/** The prefix each kind of relative path gains. */
export interface Prefixes {
	/** For `include!`, `include_str!`, and `include_bytes!`, which resolve from the file. */
	readonly include: string;
	/** For `#[path = "…"]` on a `mod x;`, which resolves from the module directory. */
	readonly path: string;
}

/**
 * Rewrites the relative paths in `source` for a file in another place: each
 * `#[path = "…"]` on a `mod x;` and each `include!`-style literal gains
 * `prefix`, one string for both or one per kind. A path inside an inline `mod x { }`
 * keeps its text, since rustc resolves it through the module's own directory,
 * which does not move. Returns `undefined` when a path cannot be rewritten
 * safely: one that is computed, escaped, set through `cfg_attr`, or given to
 * an inline module.
 */
export const prefixPaths = (
	source: string,
	prefix: string | Prefixes,
): string | undefined => {
	const { include, path } =
		typeof prefix === "string" ? { include: prefix, path: prefix } : prefix;
	const all = tokens(source);
	const edits: { literal: Token; value: string }[] = [];
	let depth = 0;
	for (let index = 0; index < all.length; index += 1) {
		depth += all[index]?.text === "{" ? 1 : 0;
		depth = all[index]?.text === "}" ? Math.max(0, depth - 1) : depth;
		const found = literalAt(all, index, depth);
		if (found === undefined) {
			return undefined;
		}
		index = found.next;
		const value = found.literal === undefined ? null : relative(found.literal);
		if (value === undefined) {
			return undefined;
		}
		if (found.literal !== undefined && value !== null) {
			const added = found.include === true ? include : path;
			edits.push({ literal: found.literal, value: `${added}${value}` });
		}
	}
	let text = source;
	for (const { literal, value } of edits.toReversed()) {
		text = `${text.slice(0, literal.start)}"${value}"${text.slice(literal.end)}`;
	}
	return text;
};
