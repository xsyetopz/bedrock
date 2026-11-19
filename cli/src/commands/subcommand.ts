import { readConfig } from "../core/config.ts";
import type { Subcommand } from "../core/language.ts";
import { contextFor } from "../core/plan.ts";
import { findLanguage } from "../languages/index.ts";
import { resolveRoot, select } from "./select.ts";

/** A language subcommand as its usage line names it, such as `jvm j2k <path>...`. */
export const subcommandUsage = (
	id: string,
	name: string,
	command: Subcommand,
): string => [id, name, command.args ?? ""].join(" ").trim();

/** Runs `bedrock <language> <subcommand>` in the current repository. */
export const runSubcommand = async (
	id: string,
	args: readonly string[],
): Promise<number | undefined> => {
	const language = findLanguage(id);
	const [name = "", ...rest] = args;
	const command = language?.subcommands?.[name];
	if (language === undefined || command === undefined) {
		return undefined;
	}
	// Without this, `--help` reaches the subcommand as a path to convert.
	if (rest.some((arg) => arg === "-h" || arg === "--help")) {
		console.info(
			`Usage: bedrock ${subcommandUsage(language.id, name, command)}\n\n${command.summary}`,
		);
		return 0;
	}
	const root = resolveRoot();
	const selected = select(await readConfig(root), [{ language }]);
	const options =
		selected.find((entry) => entry.language.id === language.id)?.options ?? {};
	return command.run(
		contextFor(root, selected, language, options, false),
		rest,
	);
};
