import { asRecord, asStrings } from "./json.ts";

/** The ignore list each tool config keeps, which a repo may extend. */
export const IGNORE_KEYS: Readonly<Record<string, string>> = {
	".markdownlint-cli2.jsonc": "ignores",
	".oxlintrc.json": "ignorePatterns",
};

const arrayOf = (key: string): RegExp =>
	new RegExp(`("${key}"\\s*:\\s*)\\[([^\\]]*)\\]`, "u");

/** The entries the template lists under `key`. */
const templateEntries = (template: string, key: string): string[] => {
	const found = arrayOf(key).exec(template);
	return found === null
		? []
		: asStrings(Bun.JSONC.parse(`[${found[2] ?? ""}]`));
};

/**
 * The template's entries that `current`, a config the repo already has,
 * dropped from its list. Bedrock owns them, so adopt puts them back.
 */
export const droppedEntries = (
	template: string,
	current: unknown,
	key: string,
): string[] => {
	if (current === undefined) {
		return [];
	}
	const kept = new Set(asStrings(asRecord(current)[key]));
	return templateEntries(template, key).filter((entry) => !kept.has(entry));
};

/**
 * Writes the template's config with the entries `current` lists under `key`
 * and the template lacks added after the template's own, in the template's
 * layout and with its comments.
 */
export const mergeIgnoreList = (
	template: string,
	current: unknown,
	key: string,
): string => {
	const found = arrayOf(key).exec(template);
	if (found === null) {
		return template;
	}
	const have = new Set(templateEntries(template, key));
	const extra = [
		...new Set(asStrings(asRecord(current)[key]).filter((e) => !have.has(e))),
	];
	if (extra.length === 0) {
		return template;
	}
	const items = [...have, ...extra].map((entry) => JSON.stringify(entry));
	const original = found[2] ?? "";
	// A JSONC list keeps the trailing comma `bun fmt` writes, or the formatter
	// and the next adopt rewrite each other; a JSON list cannot have one.
	const trailing = original.trimEnd().endsWith(",") ? "," : "";
	const body = original.includes("\n")
		? `\n${items.map((item) => `\t\t${item}`).join(",\n")}${trailing}\n\t`
		: items.join(", ");
	return template.replace(
		arrayOf(key),
		(_, head: string) => `${head}[${body}]`,
	);
};
