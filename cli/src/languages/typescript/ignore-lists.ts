import { asRecord, asStrings } from "./json.ts";

/** The ignore list each tool config keeps, which a repo may extend. */
export const IGNORE_KEYS: Readonly<Record<string, string>> = {
	".markdownlint-cli2.jsonc": "ignores",
	".oxlintrc.json": "ignorePatterns",
};

const arrayOf = (key: string): RegExp =>
	new RegExp(`("${key}"\\s*:\\s*)\\[([^\\]]*)\\]`, "u");

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
	const have = new Set(asStrings(Bun.JSONC.parse(`[${found[2] ?? ""}]`)));
	const extra = [
		...new Set(asStrings(asRecord(current)[key]).filter((e) => !have.has(e))),
	];
	if (extra.length === 0) {
		return template;
	}
	const items = [...have, ...extra].map((entry) => JSON.stringify(entry));
	const body = (found[2] ?? "").includes("\n")
		? `\n${items.map((item) => `\t\t${item}`).join(",\n")}\n\t`
		: items.join(", ");
	return template.replace(
		arrayOf(key),
		(_, head: string) => `${head}[${body}]`,
	);
};
