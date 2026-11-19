#!/usr/bin/env bun
import { adopt } from "./commands/adopt.ts";
import { clean } from "./commands/clean.ts";
import { commitMsg } from "./commands/commit-msg.ts";
import { guard } from "./commands/guard.ts";
import { languages } from "./commands/languages.ts";
import { runSubcommand, subcommandUsage } from "./commands/subcommand.ts";
import { LANGUAGES } from "./languages/index.ts";
import { VERSION } from "./version.ts";

const COMMAND_LINES: readonly (readonly [string, string])[] = [
	["init <dir> <lang>[:<kind>]...", "Start a repo."],
	["adopt [<dir>] [<lang>...]", "Move an existing repo onto bedrock."],
	["update [<dir>]", "Rewrite the managed files with this release."],
	["guard [base|<lang>...]", "Report what the linters cannot see."],
	["clean [<dir>]", "Delete *.bak files older releases left."],
	["languages [--staged | --unpushed]", "List the adopted languages."],
	["commit-msg <file>", "Check a commit message against Conventional Commits."],
	["version", "Print the version."],
	...LANGUAGES.flatMap((language) =>
		Object.entries(language.subcommands ?? {}).map(
			([name, command]) =>
				[subcommandUsage(language.id, name, command), command.summary] as const,
		),
	),
];

// Summaries start two spaces past the longest usage, so a new subcommand never runs into its text.
const width = Math.max(...COMMAND_LINES.map(([usage]) => usage.length)) + 2;

const USAGE = `bedrock ${VERSION}: one setup for TypeScript (Bun), Rust, Swift, C and C++, JVM, Go, and Haxe repos.

Usage: bedrock <command> [args]

Commands:
${COMMAND_LINES.map(([usage, summary]) => `  ${usage.padEnd(width)}${summary}`).join("\n")}

Languages: ${LANGUAGES.map((language) => `${language.id}${language.kinds.length > 0 ? ` (kinds: ${language.kinds.join(", ")})` : ""}`).join("; ")}.
Run bedrock <command> --help for its options.`;

const COMMANDS: Readonly<
	Record<string, (args: readonly string[]) => Promise<number>>
> = {
	adopt: (args) => adopt("adopt", args),
	clean,
	"commit-msg": commitMsg,
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
