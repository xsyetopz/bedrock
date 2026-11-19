/** Source text as an array of Unicode code points, so indices match Swift's scalars. */
export type Scalars = readonly string[];

/** One source line with the brace depth where it starts and how many braces it opens. */
export interface Line {
	readonly text: string;
	/** Whether the first non-blank character is code, not inside a comment or string. */
	readonly code: boolean;
	readonly depth: number;
	readonly opens: number;
}

/** An identifier in code, with the offsets of its first and one-past-last scalar. */
export interface Token {
	readonly name: string;
	readonly start: number;
	readonly end: number;
}

/** Splits text into code points. */
export const scalars = (text: string): Scalars => Array.from(text, (c) => c);

/** Whether the scalar is white space. */
export const isSpace = (c: string | undefined): boolean =>
	c !== undefined && /^\s$/u.test(c);

/** Whether the scalar can be part of an identifier, `$0` included. */
export const isIdent = (c: string | undefined): boolean =>
	c !== undefined && (c === "_" || c === "$" || /^[\p{L}\p{N}]$/u.test(c));

/** Whether the character can be part of a declared name. */
export const isWord = (c: string | undefined): boolean =>
	c !== undefined && (c === "_" || /^[\p{L}\p{N}]$/u.test(c));

/** The longest prefix of `text` whose characters satisfy `test`. */
export const prefixWhile = (
	text: string,
	test: (c: string) => boolean
): string => {
	let length = 0;
	for (const c of text) {
		if (!test(c)) {
			break;
		}
		length += c.length;
	}
	return text.slice(0, length);
};

const startsAt = (b: Scalars, index: number, text: string): boolean => {
	for (let offset = 0; offset < text.length; offset += 1) {
		if (b[index + offset] !== text[offset]) {
			return false;
		}
	}
	return true;
};

const blockCommentEnd = (b: Scalars, from: number): number => {
	let index = from + 2;
	let depth = 1;
	while (index < b.length && depth > 0) {
		let step = 0;
		if (startsAt(b, index, "/*")) {
			step = 1;
		} else if (startsAt(b, index, "*/")) {
			step = -1;
		}
		depth += step;
		index += step === 0 ? 1 : 2;
	}
	return index;
};

const stringEnd = (b: Scalars, from: number, quote: number): number => {
	const hashes = "#".repeat(quote - from);
	const quotes = startsAt(b, quote, '"""') ? '"""' : '"';
	let index = quote + quotes.length;
	while (index < b.length) {
		if (startsAt(b, index, quotes + hashes)) {
			return index + quotes.length + hashes.length;
		}
		if (quotes.length === 1 && b[index] === "\n") {
			return index;
		}
		if (startsAt(b, index, `\\${hashes}`)) {
			index += 1 + hashes.length;
			if (startsAt(b, index, "(")) {
				index = matchClose(b, index) ?? b.length;
			}
		}
		index += 1;
	}
	return b.length;
};

/** The index just past the comment or string literal that starts at `from`, if one does. */
export const trivia = (b: Scalars, from: number): number | undefined => {
	if (startsAt(b, from, "//")) {
		const newline = b.indexOf("\n", from);
		return newline === -1 ? b.length : newline;
	}
	if (startsAt(b, from, "/*")) {
		return blockCommentEnd(b, from);
	}
	let quote = from;
	while (b[quote] === "#") {
		quote += 1;
	}
	return startsAt(b, quote, '"') ? stringEnd(b, from, quote) : undefined;
};

/** The index of the bracket that closes the one at `open`. */
export const matchClose = (b: Scalars, open: number): number | undefined => {
	let depth = 0;
	let index = open;
	while (index < b.length) {
		const skip = trivia(b, index);
		if (skip !== undefined) {
			index = skip;
			continue;
		}
		depth += "([{".includes(b[index] ?? "") ? 1 : 0;
		depth -= ")]}".includes(b[index] ?? "") ? 1 : 0;
		if (depth === 0) {
			return index;
		}
		index += 1;
	}
	return undefined;
};

/** For each scalar, whether it is code, not inside a comment or string. */
export const codeMask = (b: Scalars): boolean[] => {
	const mask = Array.from({ length: b.length }, () => true);
	let index = 0;
	while (index < b.length) {
		const end = trivia(b, index);
		mask.fill(false, index, end ?? index);
		index = end ?? index + 1;
	}
	return mask;
};

/** Whether `<` at `at` opens a generic argument list that ends at the returned index. */
const genericEnd = (b: Scalars, at: number): number | undefined => {
	if (b[at] !== "<" || !isIdent(b[at - 1])) {
		return undefined;
	}
	const end = b.indexOf(">", at);
	if (end === -1) {
		return undefined;
	}
	const inside = b
		.slice(at, end)
		.every((c) => isIdent(c) || isSpace(c) || "<,.?[]():&".includes(c));
	return inside ? end : undefined;
};

/**
 * Indices outside strings, comments, and brackets, or undefined when a comment
 * is in the way.
 */
export const topLevel = (b: Scalars): number[] | undefined => {
	const found: number[] = [];
	let depth = 0;
	let index = 0;
	while (index < b.length) {
		const skip = trivia(b, index);
		if (skip !== undefined) {
			if (b[index] === "/") {
				return undefined;
			}
			index = skip;
			continue;
		}
		// The commas of a generic argument list must not split arguments.
		const generic = depth === 0 ? genericEnd(b, index) : undefined;
		if (generic !== undefined) {
			index = generic + 1;
			continue;
		}
		depth -= ")]}".includes(b[index] ?? "") ? 1 : 0;
		if (depth === 0) {
			found.push(index);
		}
		depth += "([{".includes(b[index] ?? "") ? 1 : 0;
		index += 1;
	}
	return found;
};

/** Every identifier outside comments and strings. */
export const identifiers = (b: Scalars): Token[] => {
	const mask = codeMask(b);
	const tokens: Token[] = [];
	let index = 0;
	while (index < b.length) {
		if (mask[index] !== true || !isIdent(b[index])) {
			index += 1;
			continue;
		}
		let end = index;
		while (end < b.length && isIdent(b[end])) {
			end += 1;
		}
		if (!/^[0-9]$/u.test(b[index] ?? "")) {
			tokens.push({ name: b.slice(index, end).join(""), start: index, end });
		}
		index = end;
	}
	return tokens;
};

/** The text after `index`, without leading white space and cut short. */
export const ahead = (b: Scalars, index: number): string => {
	let start = index;
	while (start < b.length && isSpace(b[start])) {
		start += 1;
	}
	return b.slice(start, start + 8).join("");
};

/** The last non-blank scalar before `index`. */
export const behind = (b: Scalars, index: number): string | undefined =>
	b.slice(0, index).findLast((c) => !isSpace(c));

/** Splits source into lines, tracking brace depth outside strings and comments. */
export const parseLines = (source: string): Line[] => {
	const b = scalars(source);
	const mask = codeMask(b);
	const lines: Line[] = [];
	let depth = 0;
	let start = 0;
	while (start <= b.length) {
		const newline = b.indexOf("\n", start);
		const end = newline === -1 ? b.length : newline;
		let first: number | undefined;
		let opens = 0;
		let braces = 0;
		for (let k = start; k < end; k += 1) {
			if (first === undefined && !" \t\r".includes(b[k] ?? "")) {
				first = k;
			}
			if (mask[k] === true && (b[k] === "{" || b[k] === "}")) {
				braces += 1;
				opens += b[k] === "{" ? 1 : 0;
			}
		}
		lines.push({
			text: b.slice(start, end).join(""),
			code: first === undefined ? false : mask[first] === true,
			depth,
			opens
		});
		depth = Math.max(0, depth + 2 * opens - braces);
		start = end + 1;
	}
	return lines;
};
