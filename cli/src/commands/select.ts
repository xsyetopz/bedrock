import { resolve } from "node:path";

import type { Config, LanguageOptions } from "../core/config.ts";
import { existing } from "../core/fs.ts";
import type { Language } from "../core/language.ts";
import type { Selected } from "../core/plan.ts";
import { findLanguage, LANGUAGES } from "../languages/index.ts";

/** A language named on the command line, as `<name>` or `<name>:<kind>[,<kind>]`. */
export interface Named {
	readonly language: Language;
	readonly kind?: string;
}

/** Parses `<name>[:<kind>]`, or throws with the names bedrock knows. */
export const parseNamed = (arg: string): Named => {
	const [name = "", kind] = arg.split(":", 2);
	const language = findLanguage(name);
	if (language === undefined) {
		const known = LANGUAGES.map((entry) =>
			[entry.id, ...entry.aliases.filter((alias) => alias !== entry.id)].join(
				", ",
			),
		);
		throw new Error(`Unknown language ${name}. Known: ${known.join("; ")}.`);
	}
	if (kind !== undefined) {
		const unknown = kind
			.split(",")
			.filter((entry) => !language.kinds.includes(entry));
		if (unknown.length > 0) {
			throw new Error(
				`${language.id} has no kind ${unknown.join(", ")}. Kinds: ${language.kinds.join(", ") || "none"}.`,
			);
		}
	}
	return kind === undefined ? { language } : { language, kind };
};

/** The languages whose marker files sit at `root`. */
export const detect = async (root: string): Promise<Language[]> => {
	const found = await Promise.all(
		LANGUAGES.map(async (language) => {
			const markers = await existing(root, language.markers);
			return markers.length > 0;
		}),
	);
	return LANGUAGES.filter((_, index) => found[index] === true);
};

/**
 * The languages a run covers: those `.bedrock.json` lists, plus those named,
 * plus the ones they require, in block order.
 */
export const select = (
	config: Config | undefined,
	named: readonly Named[],
	flags: LanguageOptions = {},
): Selected[] => {
	const options = new Map<string, LanguageOptions>(
		Object.entries(config?.languages ?? {}),
	);
	const add = (language: Language, extra: LanguageOptions): void => {
		options.set(language.id, { ...options.get(language.id), ...extra });
		for (const id of language.requires ?? []) {
			const required = findLanguage(id);
			if (required !== undefined && !options.has(required.id)) {
				add(required, {});
			}
		}
	};
	for (const entry of named) {
		add(entry.language, entry.kind === undefined ? {} : { kind: entry.kind });
	}
	if (flags["cxx"] === true) {
		const cpp = findLanguage("cpp");
		if (cpp !== undefined) {
			add(cpp, {});
		}
	}
	for (const id of options.keys()) {
		const language = findLanguage(id);
		if (language !== undefined) {
			add(language, {});
		}
	}
	return LANGUAGES.filter((language) => options.has(language.id)).map(
		(language) => ({
			language,
			options: options.get(language.id) ?? {},
		}),
	);
};

/** The tools the selected languages need that are not on PATH, with how to install each. */
export const missingTools = (selected: readonly Selected[]): string[] => {
	const tools: Record<string, string> = {
		git: "the system's package manager, such as brew install git",
		just: "brew install just, or cargo install --locked just",
	};
	for (const { language } of selected) {
		Object.assign(tools, language.tools);
	}
	return Object.entries(tools)
		.filter(([tool]) => Bun.which(tool) === null)
		.map(([tool, how]) => `${tool}: ${how}`)
		.toSorted();
};

/** `root` resolved against the working directory. */
export const resolveRoot = (arg?: string): string => resolve(arg ?? ".");
