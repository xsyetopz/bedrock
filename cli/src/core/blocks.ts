/** Comment syntax of a file that holds marked blocks. */
export type Syntax = "markdown" | "hash" | "slash";

/** A named, marked region of a shared file that one part of bedrock owns. */
export interface Block {
	readonly name: string;
	readonly content: string;
}

/**
 * Block names, in the order a file lists them. Repos adopted before the
 * single CLI carry these names, so they stay as they are.
 */
export const BLOCK_ORDER = [
	"bedrock",
	"typescript-bedrock",
	"rust-bedrock",
	"swift-bedrock",
	"swift-bedrock-cxx",
	"jvm-bedrock",
	"go-bedrock",
] as const;

export const markers = (syntax: Syntax, name: string): [string, string] => {
	if (syntax === "markdown") {
		return [`<!-- ${name} -->`, `<!-- /${name} -->`];
	}
	const prefix = syntax === "hash" ? "#" : "//";
	return [`${prefix} >>> ${name}`, `${prefix} <<< ${name}`];
};

/** Where the first line starting with `marker` begins in `text`, or -1. */
const lineStart = (text: string, marker: string): number =>
	text.startsWith(marker) ? 0 : text.indexOf(`\n${marker}`) + 1 || -1;

/** Where the first block listed after `name` starts, or Infinity. */
const laterBlock = (text: string, syntax: Syntax, name: string): number => {
	const order: readonly string[] = BLOCK_ORDER;
	return Math.min(
		...order
			.slice(order.indexOf(name) + 1)
			.map((later) => lineStart(text, markers(syntax, later)[0]))
			.filter((index) => index !== -1),
	);
};

/** `text` without the block `name`, or `text` when it has none. */
export const removeBlock = (
	text: string,
	syntax: Syntax,
	name: string,
): string => {
	const [open, close] = markers(syntax, name);
	const start = lineStart(text, open);
	const closeAt = start === -1 ? -1 : text.indexOf(close, start);
	if (closeAt === -1) {
		return text;
	}
	let end = closeAt + close.length;
	end += text[end] === "\n" ? 1 : 0;
	// The blank line that separated the block from the next one goes with it.
	const separated = start === 0 || text.slice(start - 2, start) === "\n\n";
	end += separated && text[end] === "\n" ? 1 : 0;
	return `${text.slice(0, start)}${text.slice(end)}`;
};

/**
 * Returns `current` with each block replaced or, when missing, added in
 * {@link BLOCK_ORDER}. A missing file starts as `header`.
 */
export const upsert = (
	current: string | undefined,
	header: string,
	syntax: Syntax,
	blocks: readonly Block[],
): string => {
	let text = current ?? header;
	for (const block of blocks) {
		const [open, close] = markers(syntax, block.name);
		const marked = `${open}\n${block.content}${close}\n`;
		const start = lineStart(text, open);
		const closeAt = start === -1 ? -1 : text.indexOf(close, start);
		const next = laterBlock(text, syntax, block.name);
		if (closeAt === -1 && Number.isFinite(next)) {
			text = `${text.slice(0, next)}${marked}\n${text.slice(next)}`;
		} else if (closeAt === -1) {
			if (text !== "" && !text.endsWith("\n\n")) {
				text += text.endsWith("\n") ? "\n" : "\n\n";
			}
			text += marked;
		} else {
			const end = closeAt + close.length;
			text = `${text.slice(0, start)}${marked}${text.slice(text[end] === "\n" ? end + 1 : end)}`;
		}
	}
	return text;
};
