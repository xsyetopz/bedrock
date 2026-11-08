export type Syntax = "markdown" | "hash";

export interface Block {
	readonly name: string;
	readonly content: string;
}

const markers = (syntax: Syntax, name: string): [string, string] =>
	syntax === "markdown"
		? [`<!-- ${name} -->`, `<!-- /${name} -->`]
		: [`# >>> ${name}`, `# <<< ${name}`];

/** Every bedrock's block, in the order a file lists them, so a file comes out the same whichever bedrock adopts first. */
const ORDER = [
	"bedrock",
	"typescript-bedrock",
	"rust-bedrock",
	"swift-bedrock",
	"swift-bedrock-cxx",
	"jvm-bedrock",
];

/** Where the first line starting with `marker` begins in `text`, or -1. */
const lineStart = (text: string, marker: string): number =>
	text.startsWith(marker) ? 0 : text.indexOf(`\n${marker}`) + 1 || -1;

/** Where the first block of a bedrock after `name` starts, or Infinity. */
const laterBlock = (text: string, syntax: Syntax, name: string): number =>
	Math.min(
		...ORDER.slice(ORDER.indexOf(name) + 1)
			.map((later) => lineStart(text, markers(syntax, later)[0]))
			.filter((index) => index !== -1),
	);

/**
 * Returns `current` with each block replaced or, when missing, added in
 * bedrock order. A missing file starts as `header`. Matches rust-bedrock byte
 * for byte.
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
		const start = text.indexOf(open);
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
