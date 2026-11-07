#!/usr/bin/env bun
import { adopt } from "./adopt/command.ts";
import { guard } from "./guard.ts";

const USAGE = `Usage: typescript-bedrock <command> [args]

Commands:
  adopt <target>     Set up a repo with typescript-bedrock. See \`adopt --help\`.
  guard              Report suppression comments, foreign lockfiles and scripts,
                     TypeScript drift, and unsorted catalogs, and files that differ
                     from what adopt writes.`;

const COMMANDS: Record<string, (args: string[]) => Promise<number>> = {
	adopt,
	guard: () => guard(process.cwd()),
};

const [name = "", ...args] = Bun.argv.slice(2);
const command = COMMANDS[name];
if (command === undefined) {
	console.info(USAGE);
	process.exitCode = ["-h", "--help"].includes(name) ? 0 : 2;
} else {
	try {
		process.exitCode = await command(args);
	} catch (error) {
		console.error(error instanceof Error ? error.message : error);
		process.exitCode = 1;
	}
}
