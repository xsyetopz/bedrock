import { join } from "node:path";

import { parseEditorconfig } from "./editorconfig/index.ts";
import { readText } from "./fs.ts";

/** The file whose bedrock block records the release, the languages, and their options. */
export const CONFIG_PATH = ".editorconfig";

/** Where older releases kept the release and languages; adopt moves them into {@link CONFIG_PATH}. */
export const LEGACY_CONFIG_PATH = ".bedrock.json";

/** The version of a repo that runs bedrock from its own source tree. */
export const SOURCE_VERSION = "source";

/** Per-language settings, such as `{ cxx: true }` for Swift. */
export type LanguageOptions = Readonly<Record<string, string | boolean>>;

/** The release and languages a repo records. */
export interface Config {
	/** The release `just setup` installs, or {@link SOURCE_VERSION}. */
	readonly version: string;
	/** The adopted languages, by id, with their settings. */
	readonly languages: Readonly<Record<string, LanguageOptions>>;
}

const VERSION_KEY = "bedrock_version";
const LANGUAGES_KEY = "bedrock_languages";

const isRecord = (value: unknown): value is Record<string, unknown> =>
	typeof value === "object" && value !== null && !Array.isArray(value);

const optionValue = (value: string): string | boolean =>
	value === "true" || value === "false" ? value === "true" : value;

/** The config in the `[*]` sections of an `.editorconfig`, or undefined when it has no `bedrock_version`. */
const fromEditorconfig = (text: string): Config | undefined => {
	const pairs = new Map(
		parseEditorconfig(text)
			.filter((section) => section.glob === "*")
			.flatMap((section) => section.pairs)
	);
	const version = pairs.get(VERSION_KEY);
	if (version === undefined) {
		return undefined;
	}
	const ids = (pairs.get(LANGUAGES_KEY) ?? "")
		.split(",")
		.map((id) => id.trim())
		.filter((id) => id !== "");
	const languages = Object.fromEntries(
		ids.map((id) => {
			const prefix = `bedrock_${id}_`;
			const options = [...pairs]
				.filter(([key]) => key.startsWith(prefix))
				.map(([key, value]) => [key.slice(prefix.length), optionValue(value)]);
			return [id, Object.fromEntries(options)];
		})
	);
	return { version, languages };
};

/** One language's options from a `.bedrock.json`, or undefined when they are not strings and booleans. */
const legacyOptions = (options: unknown): LanguageOptions | undefined => {
	if (!isRecord(options)) {
		return undefined;
	}
	const entries = Object.entries(options).filter(
		(entry): entry is [string, string | boolean] =>
			typeof entry[1] === "string" || typeof entry[1] === "boolean"
	);
	return entries.length === Object.keys(options).length
		? Object.fromEntries(entries)
		: undefined;
};

/** The `.bedrock.json` an older release wrote. */
const fromLegacy = (text: string): Config => {
	const data: unknown = JSON.parse(text);
	const version = isRecord(data) ? data["version"] : undefined;
	const languages = isRecord(data) ? data["languages"] : undefined;
	const entries = isRecord(languages)
		? Object.entries(languages).map(
				([id, options]) => [id, legacyOptions(options)] as const
			)
		: [];
	const valid = entries.filter(
		(entry): entry is readonly [string, LanguageOptions] =>
			entry[1] !== undefined
	);
	if (
		typeof version !== "string" ||
		!isRecord(languages) ||
		valid.length !== entries.length
	) {
		throw new Error(
			`${LEGACY_CONFIG_PATH}: expected { "version": string, "languages": { <id>: { <option>: string | boolean } } }`
		);
	}
	return { version, languages: Object.fromEntries(valid) };
};

/**
 * The repo's config from the bedrock block of `.editorconfig`, else from a
 * `.bedrock.json` an older release wrote, or undefined when it has neither.
 */
export const readConfig = async (root: string): Promise<Config | undefined> => {
	const [editorconfig, legacy] = await Promise.all([
		readText(join(root, CONFIG_PATH)),
		readText(join(root, LEGACY_CONFIG_PATH))
	]);
	const current =
		editorconfig === undefined ? undefined : fromEditorconfig(editorconfig);
	if (current !== undefined || legacy === undefined) {
		return current;
	}
	return fromLegacy(legacy);
};

/**
 * The `bedrock_*` pairs the bedrock block of `.editorconfig` opens its `[*]`
 * section with. `bedrock_version` keeps its own line, `key = value`, since
 * the justfile reads it with sed to find the binary before bedrock runs.
 */
export const configPairs = (config: Config): string => {
	const ids = Object.keys(config.languages).toSorted();
	const options = ids.flatMap((id) =>
		Object.entries(config.languages[id] ?? {})
			.toSorted(([a], [b]) => a.localeCompare(b))
			.map(([key, value]) => `bedrock_${id}_${key} = ${String(value)}`)
	);
	return [
		`${VERSION_KEY} = ${config.version}`,
		`${LANGUAGES_KEY} =${ids.length === 0 ? "" : ` ${ids.join(", ")}`}`,
		...options
	].join("\n");
};
