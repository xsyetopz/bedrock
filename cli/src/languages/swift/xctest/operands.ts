import {
	isSpace,
	isWord,
	prefixWhile,
	type Scalars,
	scalars,
	topLevel,
} from "./lexer.ts";

const OPERATORS = new Set("+*/%<>=&|^~:,;");

/** Whether the top-level character at `index` makes the text a compound expression. */
const isCompound = (b: Scalars, index: number): boolean => {
	const c = b[index] ?? "";
	const next = b[index + 1] ?? " ";
	if (OPERATORS.has(c)) {
		return true;
	}
	switch (c) {
		case "-": {
			return index !== 0;
		}
		case "!": {
			return index !== 0 && next === "=";
		}
		case "?": {
			return !".([)".includes(next) && index + 1 !== b.length;
		}
		case ".": {
			return next === "." || (index > 0 && b[index - 1] === ".");
		}
		default: {
			return isSpace(c);
		}
	}
};

const needsParens = (text: string): boolean => {
	const b = scalars(text);
	return (topLevel(b) ?? []).some((index) => isCompound(b, index));
};

/** Parenthesizes text that is a compound expression. */
export const wrap = (text: string): string =>
	needsParens(text) ? `(${text})` : text;

/** The top-level, trimmed arguments, or undefined when a comment sits inside them. */
export const splitArguments = (b: Scalars): string[] | undefined => {
	const tops = topLevel(b);
	if (tops === undefined) {
		return undefined;
	}
	const cuts = [-1, ...tops.filter((index) => b[index] === ","), b.length];
	const parts = cuts.slice(1).map((cut, index) =>
		b
			.slice((cuts[index] ?? 0) + 1, cut)
			.join("")
			.trim(),
	);
	if (parts.at(-1) === "") {
		parts.pop();
	}
	return parts;
};

/** The `file:`, `line:`, or `accuracy:` label that starts an argument. */
const labelOf = (part: string): [string, string] | undefined => {
	const name = prefixWhile(part, isWord);
	const after = part.slice(name.length).replace(/^ +/u, "");
	if (
		!["file", "line", "accuracy"].includes(name) ||
		!after.startsWith(":") ||
		after.startsWith("::")
	) {
		return undefined;
	}
	return [name, after.slice(1).trim()];
};

/** Arguments of one assertion call, without the `file:` and `line:` labels. */
export interface Arguments {
	readonly positional: string[];
	readonly accuracy: string | undefined;
}

export const parseArguments = (
	parts: readonly string[],
	rewrite: (text: string) => string,
): Arguments => {
	const positional: string[] = [];
	let accuracy: string | undefined;
	for (const part of parts.map((text) => rewrite(text))) {
		const label = labelOf(part);
		if (label?.[0] === "accuracy") {
			accuracy = label[1];
		} else if (label === undefined) {
			positional.push(part);
		}
	}
	return { positional, accuracy };
};
