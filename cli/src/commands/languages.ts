import { parseArgs } from "node:util";

import { Glob } from "bun";

import { CONFIG_PATH, readConfig } from "../core/config.ts";
import { stagedPaths, unpushedPaths } from "../core/git.ts";
import type { Language } from "../core/language.ts";
import { select } from "./select.ts";

const USAGE = `Usage: bedrock languages [--staged | --unpushed]

Prints the id of each language .bedrock.json lists, one per line. With
--staged, only those whose files the commit changes, so the pre-commit hook
skips the toolchains a commit does not touch. With --unpushed, only those whose
files HEAD's commits change that no remote has yet, for the pre-push hook.`;

/** Lists the paths the flags narrow the languages to; none means every language. */
const listerFor = (
	staged: boolean,
	unpushed: boolean,
): ((root: string) => Promise<string[] | undefined>) | undefined => {
	if (staged) {
		return stagedPaths;
	}
	return unpushed ? unpushedPaths : undefined;
};

/** Base files whose change can alter every language's checks. */
const SHARED = new Set([CONFIG_PATH, "justfile"]);

/** Whether a staged path concerns `language`: its sources, or its own recipes and CI. */
export const touches = (language: Language, path: string): boolean =>
	SHARED.has(path) ||
	path === `.just/${language.id}.just` ||
	path === `.github/workflows/${language.id}.yml` ||
	language.sources.some((pattern) => new Glob(pattern).match(path));

/** `bedrock languages`. */
export const languages = async (args: readonly string[]): Promise<number> => {
	const { values } = parseArgs({
		args: [...args],
		options: {
			help: { type: "boolean", short: "h", default: false },
			staged: { type: "boolean", default: false },
			unpushed: { type: "boolean", default: false },
		},
	});
	if (values.help) {
		console.info(USAGE);
		return 0;
	}
	const root = process.cwd();
	const selected = select(await readConfig(root), []);
	const paths = await listerFor(values.staged, values.unpushed)?.(root);
	for (const { language } of selected) {
		if (paths === undefined || paths.some((path) => touches(language, path))) {
			console.info(language.id);
		}
	}
	return 0;
};
