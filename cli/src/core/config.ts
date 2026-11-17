import { join } from "node:path";

import { readText } from "./fs.ts";

/** The file that records which bedrock version and languages a repo uses. */
export const CONFIG_PATH = ".bedrock.json";

/** The version of a repo that runs bedrock from its own source tree. */
export const SOURCE_VERSION = "source";

/** Per-language settings, such as `{ "cxx": true }` for Swift. */
export type LanguageOptions = Readonly<Record<string, string | boolean>>;

/** What `.bedrock.json` holds. */
export interface Config {
	/** The release `just setup` installs, or {@link SOURCE_VERSION}. */
	readonly version: string;
	/** The adopted languages, by id, with their settings. */
	readonly languages: Readonly<Record<string, LanguageOptions>>;
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
	typeof value === "object" && value !== null && !Array.isArray(value);

const isOptions = (value: unknown): value is LanguageOptions =>
	isRecord(value) &&
	Object.values(value).every(
		(entry) => typeof entry === "string" || typeof entry === "boolean",
	);

/** The languages map, or undefined when any entry is malformed. */
const parseLanguages = (
	value: unknown,
): Record<string, LanguageOptions> | undefined => {
	if (!isRecord(value)) {
		return undefined;
	}
	const entries = Object.entries(value);
	const valid = entries.flatMap(([id, options]): [string, LanguageOptions][] =>
		isOptions(options) ? [[id, options]] : [],
	);
	return valid.length === entries.length
		? Object.fromEntries(valid)
		: undefined;
};

/** Reads `.bedrock.json`, or returns undefined when the repo has none. */
export const readConfig = async (root: string): Promise<Config | undefined> => {
	const text = await readText(join(root, CONFIG_PATH));
	if (text === undefined) {
		return undefined;
	}
	const data: unknown = JSON.parse(text);
	const languages = isRecord(data)
		? parseLanguages(data["languages"])
		: undefined;
	const version = isRecord(data) ? data["version"] : undefined;
	if (typeof version !== "string" || languages === undefined) {
		throw new Error(
			`${CONFIG_PATH}: expected { "version": string, "languages": { <id>: { <option>: string | boolean } } }`,
		);
	}
	return { version, languages };
};

/** One language's options on one line, spaced as `bun fmt` writes them. */
const inline = (options: LanguageOptions): string => {
	const entries = Object.entries(options).map(
		([key, value]) => `${JSON.stringify(key)}: ${JSON.stringify(value)}`,
	);
	return entries.length === 0 ? "{}" : `{ ${entries.join(", ")} }`;
};

/**
 * The text of `.bedrock.json`. `version` comes first on its own line, since
 * the justfile reads it with sed to find the binary before bedrock runs.
 */
export const formatConfig = (config: Config): string => {
	const languages = Object.keys(config.languages)
		.toSorted()
		.map(
			(id) =>
				`\t\t${JSON.stringify(id)}: ${inline(config.languages[id] ?? {})}`,
		);
	return [
		"{",
		`\t"version": ${JSON.stringify(config.version)},`,
		languages.length === 0
			? '\t"languages": {}'
			: `\t"languages": {\n${languages.join(",\n")}\n\t}`,
		"}",
		"",
	].join("\n");
};
