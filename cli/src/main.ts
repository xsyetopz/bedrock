#!/usr/bin/env bun
import { adopt } from "./commands/adopt.ts";
import { clean } from "./commands/clean.ts";
import { guard } from "./commands/guard.ts";
import { languages } from "./commands/languages.ts";
import { resolveRoot, select } from "./commands/select.ts";
import { readConfig } from "./core/config.ts";
import { contextFor } from "./core/plan.ts";
import { findLanguage, LANGUAGES } from "./languages/index.ts";
import { VERSION } from "./version.ts";

const subcommandLines = LANGUAGES.flatMap((language) =>
	Object.entries(language.subcommands ?? {}).map(
		([name, command]) =>
			`  ${`${language.id} ${name}`.padEnd(20)}${command.summary}`,
	),
);

const USAGE = `bedrock ${VERSION}: one setup for TypeScript (Bun), Rust, Swift, C++, and JVM repos.

Usage: bedrock <command> [args]

Commands:
  init <dir> <lang>[:<kind>]...  Start a repo.
  adopt [<dir>] [<lang>...]      Move an existing repo onto bedrock.
  update [<dir>]                 Rewrite the managed files with this release.
  guard [base|<lang>...]         Report what the linters cannot see.
  clean [<dir>]                  Delete *.bak files older releases left.
  languages [--staged]           List the adopted languages.
  version                        Print the version.
${subcommandLines.join("\n")}

Languages: ${LANGUAGES.map((language) => `${language.id}${language.kinds.length > 0 ? ` (kinds: ${language.kinds.join(", ")})` : ""}`).join("; ")}.
Run bedrock <command> --help for its options.`;

/** Runs `bedrock <language> <subcommand>` in the current repository. */
const runSubcommand = async (
	id: string,
	args: readonly string[],
): Promise<number | undefined> => {
	const language = findLanguage(id);
	const [name = "", ...rest] = args;
	const command = language?.subcommands?.[name];
	if (language === undefined || command === undefined) {
		return undefined;
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

const COMMANDS: Readonly<
	Record<string, (args: readonly string[]) => Promise<number>>
> = {
	adopt: (args) => adopt("adopt", args),
	clean,
	guard,
	init: (args) => adopt("init", args),
	languages,
	update: (args) => adopt("update", args),
	version: () => {
		console.info(VERSION);
		return Promise.resolve(0);
	},
};

const run = async (argv: readonly string[]): Promise<number> => {
	const [name = "", ...args] = argv;
	const command = COMMANDS[name];
	if (command !== undefined) {
		return command(args);
	}
	const subcommand = await runSubcommand(name, args);
	if (subcommand !== undefined) {
		return subcommand;
	}
	console.info(USAGE);
	return ["", "-h", "--help", "help"].includes(name) ? 0 : 2;
};

try {
	process.exitCode = await run(Bun.argv.slice(2));
} catch (error) {
	console.error(error instanceof Error ? error.message : String(error));
	process.exitCode = 1;
}
