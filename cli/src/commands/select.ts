import { resolve } from "node:path";

import { Glob } from "bun";

import type { Config, LanguageOptions } from "../core/config.ts";
import { existing } from "../core/fs.ts";
import type { Language } from "../core/language.ts";
import type { Selected } from "../core/plan.ts";
import { templateDir } from "../core/templates.ts";
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
				", "
			)
		);
		throw new Error(`Unknown language ${name}. Known: ${known.join("; ")}.`);
	}
	if (kind !== undefined) {
		const unknown = kind
			.split(",")
			.filter((entry) => !language.kinds.includes(entry));
		if (unknown.length > 0) {
			throw new Error(
				`${language.id} has no kind ${unknown.join(", ")}. Kinds: ${language.kinds.join(", ") || "none"}.`
			);
		}
	}
	return kind === undefined ? { language } : { language, kind };
};

/** Whether a file matches the glob `pattern` under `root`. */
const matches = async (root: string, pattern: string): Promise<boolean> => {
	for await (const _ of new Glob(pattern).scan({
		cwd: root,
		onlyFiles: true
	})) {
		return true;
	}
	return false;
};

/** Whether any of `language`'s markers, a path or a glob, other than those `ignored`, is at `root`. */
const hasMarker = async (
	root: string,
	language: Language,
	ignored: readonly string[] = []
): Promise<boolean> => {
	const markers = language.markers.filter(
		(marker) => !ignored.includes(marker)
	);
	const globs = markers.filter((marker) => /[*?{[]/u.test(marker));
	const paths = markers.filter((marker) => !globs.includes(marker));
	const present = await existing(root, paths);
	if (present.length > 0) {
		return true;
	}
	const found = await Promise.all(globs.map((glob) => matches(root, glob)));
	return found.includes(true);
};

/** The languages whose marker files sit at `root`. */
export const detect = async (root: string): Promise<Language[]> => {
	const found = await Promise.all(
		LANGUAGES.map((language) => hasMarker(root, language))
	);
	return LANGUAGES.filter((_, index) => found[index] === true);
};

/**
 * The languages a run leaves out that no marker file at `root` points to. A
 * repo that dropped one keeps no block or file of it; one whose markers
 * remain, such as a repo adopted before bedrock recorded its languages, keeps it.
 * Markers bedrock writes itself, such as gradlew, don't count: they outlive
 * the code they were written for.
 */
export const dropped = async (
	root: string,
	selected: readonly Selected[]
): Promise<Language[]> => {
	const left = LANGUAGES.filter(
		(language) => !selected.some((entry) => entry.language === language)
	);
	const marked = await Promise.all(
		left.map(async (language) =>
			hasMarker(
				root,
				language,
				await templateDir(`${language.id}/files`).paths()
			)
		)
	);
	return left.filter((_, index) => marked[index] !== true);
};

/** A Java package name, which the group becomes for the starter code. */
const GROUP = /^[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)*$/u;

/** The jvm options `--group` adds to, or an error when the run has no jvm or the group is no package name. */
const jvmOptions = (
	options: ReadonlyMap<string, LanguageOptions>,
	group: string
): LanguageOptions => {
	const current = options.get("jvm");
	if (current === undefined) {
		throw new Error("--group sets the jvm group; name jvm too.");
	}
	if (!GROUP.test(group)) {
		throw new Error(
			`--group ${group} is no Java package name, such as com.example.app.`
		);
	}
	return current;
};

/**
 * The languages a run covers: those `.editorconfig` lists, plus those named,
 * in block order. Each stands alone, so a repo mixes any of them.
 */
export const select = (
	config: Config | undefined,
	named: readonly Named[],
	flags: LanguageOptions = {}
): Selected[] => {
	const options = new Map<string, LanguageOptions>(
		Object.entries(config?.languages ?? {})
	);
	const add = (id: string, extra: LanguageOptions): void => {
		options.set(id, { ...options.get(id), ...extra });
	};
	for (const entry of named) {
		add(
			entry.language.id,
			entry.kind === undefined ? {} : { kind: entry.kind }
		);
	}
	if (flags["cxx"] === true) {
		add("cpp", {});
	}
	if (typeof flags["group"] === "string") {
		const group = flags["group"];
		options.set("jvm", { ...jvmOptions(options, group), group });
	}
	return LANGUAGES.filter((language) => options.has(language.id)).map(
		(language) => ({
			language,
			options: options.get(language.id) ?? {}
		})
	);
};

/** The tools the selected languages need that are not on PATH, with how to install each. */
export const missingTools = (selected: readonly Selected[]): string[] => {
	const tools: Record<string, string> = {
		git: "the system's package manager, such as brew install git",
		just: "brew install just, or cargo install --locked just"
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
