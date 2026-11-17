/** One element of a parsed XML file. */
export interface XmlElement {
	readonly name: string;
	readonly attributes: Readonly<Record<string, string>>;
	readonly children: readonly XmlElement[];
	/** The text of the element and everything below it, like the DOM's `textContent`. */
	readonly text: string;
}

interface Open {
	readonly name: string;
	readonly attributes: Record<string, string>;
	readonly children: Open[];
	text: string;
}

const TOKEN =
	/<!--[\s\S]*?-->|<!\[CDATA\[([\s\S]*?)\]\]>|<\?[\s\S]*?\?>|<!DOCTYPE[\s\S]*?>|<\/([^\s>]+)\s*>|<([^\s/>!?]+)((?:\s+[^\s=/>]+\s*=\s*(?:"[^"]*"|'[^']*'))*)\s*(\/?)>|([^<]+)/gu;
const ATTRIBUTE = /([^\s=]+)\s*=\s*(?:"([^"]*)"|'([^']*)')/gu;

const ENTITIES: Readonly<Record<string, string>> = {
	amp: "&",
	apos: "'",
	gt: ">",
	lt: "<",
	quot: '"',
};

const decode = (text: string): string =>
	text.replaceAll(/&(#x[0-9a-f]+|#\d+|[a-z]+);/giu, (whole, entity: string) => {
		if (entity.startsWith("#")) {
			const code =
				entity[1]?.toLowerCase() === "x"
					? Number.parseInt(entity.slice(2), 16)
					: Math.trunc(Number(entity.slice(1)));
			return String.fromCodePoint(code);
		}
		return ENTITIES[entity] ?? whole;
	});

const attributesOf = (source: string): Record<string, string> =>
	Object.fromEntries(
		[...source.matchAll(ATTRIBUTE)].map((match) => [
			match[1] ?? "",
			decode(match[2] ?? match[3] ?? ""),
		]),
	);

const freeze = (open: Open): XmlElement => ({
	name: open.name,
	attributes: open.attributes,
	children: open.children.map((child) => freeze(child)),
	text: open.text,
});

/**
 * Parses an XML file into its root element. Maven, Ant, and Ivy files have no
 * DTD, so a DOCTYPE is refused, which keeps a hostile file from reading others.
 */
export const parseXml = (source: string): XmlElement => {
	if (/<!DOCTYPE/iu.test(source)) {
		throw new Error("XML with a DOCTYPE is not read");
	}
	const stack: Open[] = [];
	let root: Open | undefined;
	for (const match of source.matchAll(TOKEN)) {
		const [, cdata, closing, opening, attributes = "", selfClosed, text] =
			match;
		const current = stack.at(-1);
		if (opening !== undefined) {
			const element: Open = {
				name: opening,
				attributes: attributesOf(attributes),
				children: [],
				text: "",
			};
			if (current === undefined) {
				root ??= element;
			} else {
				current.children.push(element);
			}
			if (selfClosed !== "/") {
				stack.push(element);
			}
		} else if (closing === undefined) {
			const chunk = cdata ?? (text === undefined ? "" : decode(text));
			for (const open of stack) {
				open.text += chunk;
			}
		} else {
			stack.pop();
		}
	}
	if (root === undefined) {
		throw new Error("XML has no root element");
	}
	return freeze(root);
};

/** The elements directly below `element`, all of them or only those called `name`. */
export const childrenOf = (
	element: XmlElement | undefined,
	name?: string,
): readonly XmlElement[] =>
	(element?.children ?? []).filter(
		(child) => name === undefined || child.name === name,
	);

/** The first child called `name`. */
export const childOf = (
	element: XmlElement | undefined,
	name: string,
): XmlElement | undefined => childrenOf(element, name)[0];

/** The trimmed text of the child called `name`. */
export const textOf = (
	element: XmlElement | undefined,
	name: string,
): string | undefined => childOf(element, name)?.text.trim();

/** Every element called `name` below `element`, in document order. */
export const descendants = (element: XmlElement, name: string): XmlElement[] =>
	element.children.flatMap((child) => [
		...(child.name === name ? [child] : []),
		...descendants(child, name),
	]);
