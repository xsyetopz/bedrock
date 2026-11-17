import { type Token, tokens } from "./lexer.ts";
import { closing } from "./modules.ts";

/** The attribute body between the brackets, after `#` or `#!`. */
interface Attribute {
	readonly line: number;
	readonly inner: readonly Token[];
	/** The index of the token after the closing bracket. */
	readonly next: number;
}

const attributeAt = (
	all: readonly Token[],
	index: number,
): Attribute | undefined => {
	const hash = all[index];
	if (hash?.text !== "#") {
		return undefined;
	}
	const open = all[index + 1]?.text === "!" ? index + 2 : index + 1;
	const end = closing(all, open);
	return end === undefined || all[open]?.text !== "["
		? undefined
		: { line: hash.line, inner: all.slice(open + 1, end), next: end + 1 };
};

/** Whether the attribute's arguments have a top-level `reason = …`. */
const hasReason = (inner: readonly Token[]): boolean =>
	inner.some(
		(token, index) => token.text === "reason" && inner[index + 1]?.text === "=",
	);

/** The message for one `allow` or `expect` attribute that breaks the rule, if any. */
const problemOf = (attribute: Attribute): string | undefined => {
	const word = attribute.inner[0]?.text;
	if (word === "allow") {
		return `#[allow] hides a lint without saying why; use #[expect(lint, reason = "…")]`;
	}
	return word === "expect" && !hasReason(attribute.inner)
		? `#[expect] without a reason; add reason = "…"`
		: undefined;
};

/**
 * Lint suppressions in `source` that skip the rule: `#[allow]` anywhere, and
 * `#[expect]` without a `reason`. Returns `line: message` pairs.
 */
export const suppressionProblems = (
	source: string,
): { line: number; message: string }[] => {
	const all = tokens(source);
	const found: { line: number; message: string }[] = [];
	let index = 0;
	while (index < all.length) {
		const attribute = attributeAt(all, index);
		const message = attribute === undefined ? undefined : problemOf(attribute);
		if (attribute !== undefined && message !== undefined) {
			found.push({ line: attribute.line, message });
		}
		index = attribute === undefined ? index + 1 : attribute.next;
	}
	return found;
};
