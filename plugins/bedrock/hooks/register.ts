import type {
	BashCall,
	BedrockCall,
	Engine,
	Hook,
	On,
	ProcessResult
} from "./engine.ts";
import { findBedrock } from "./runner.ts";

// A package manager in command position: at a line start or after ; & | ( $( or `, past env assignments and sudo, exec, or command.
const FOREIGN =
	/(?:^|[;&|(`]|\$\()\s*(?:\w+=\S*\s+)*(?:(?:sudo|exec|command)\s+)?(npm|npx|pnpm|pnpx|yarn)(?=\s|$)/mu;

/** Commands that change files: the plugin previews them unless the call passes --yes. */
const PLANNED = new Set(["adopt", "init", "update"]);

/** The tool the `/bedrock` and `/adopt` skills call; Claude Code prefixes the name with `mcp__bedrock__`. */
export const TOOL = "run";

/**
 * The bedrock arguments for one call. A planned command without --yes gets
 * --dry-run, so a call never changes files without asking for it.
 */
export const bedrockArgs = (input: string): string[] => {
	const args = input.split(/\s+/u).filter((word) => word !== "");
	const [command = "help"] = args;
	if (
		PLANNED.has(command) &&
		!args.includes("--yes") &&
		!args.includes("--dry-run")
	) {
		return [...args, "--dry-run"];
	}
	return args.length === 0 ? ["help"] : args;
};

/** What one bedrock run gives the model: its output, and whether it failed. */
export interface Outcome {
	readonly text: string;
	readonly isError: boolean;
}

/** Runs bedrock with a command line in the session's folder. */
export const runBedrock = async (
	$: Engine,
	input: string
): Promise<Outcome> => {
	const args = bedrockArgs(input);
	let result: ProcessResult;
	try {
		const command = await findBedrock($);
		const cwd = await $.session.cwd();
		result = await $.process.run(
			[
				"sh",
				"-c",
				'cd "$1" && shift && exec "$@"',
				"sh",
				cwd,
				...command,
				...args
			],
			{ timeoutMs: 600_000 }
		);
	} catch (error) {
		const reason = error instanceof Error ? error.message : String(error);
		return { text: `bedrock did not run: ${reason}`, isError: true };
	}
	const { exitCode, stdout, stderr } = result;
	const ran = `$ bedrock ${args.join(" ")}\n${`${stdout}${stderr}`.trim()}`;
	return exitCode === 0
		? { text: ran, isError: false }
		: { text: `${ran}\n(bedrock exited ${exitCode})`, isError: true };
};

/** The package manager a Bash command runs, if it is one Bun replaces. */
export const foreignPackageManager = (command: string): string | undefined =>
	FOREIGN.exec(command)?.[1];

/** Registers the tool the skills call, then continues. */
export const announce: Hook<unknown> = async ($, e, next) => {
	await $.tool.register({
		name: TOOL,
		description:
			"Runs the bedrock CLI in the session's folder with one command line, such as `adopt`, `update`, `guard rust`, or `adopt ts --yes`, and returns its output. `adopt`, `init`, and `update` print their plan and change nothing unless the line has `--yes`.",
		inputSchema: {
			type: "object",
			properties: {
				args: {
					type: "string",
					description: "The bedrock command line, without `bedrock`."
				}
			},
			required: ["args"]
		},
		isDeferred: false
	});
	return next(e);
};

/** Answers an allowed call of the tool with bedrock's output. */
export const answerRun: Hook<BedrockCall> = async ($, e) => {
	const { text, isError } = await runBedrock($, e.args);
	return isError ? { result: text, isError } : { result: text };
};

/** In a repo with `bun.lock`, Bun is the package manager. */
export const blockForeign: Hook<BashCall> = async ($, e, next) => {
	const manager = foreignPackageManager(e.command);
	if (
		manager === undefined ||
		!(await $.fs.exists(`${await $.session.cwd()}/bun.lock`))
	) {
		return next(e);
	}
	return {
		deny: `${$.plugin.name}: this repo uses Bun. Run \`bun\` in place of ${manager}, or \`bunx\` for a one-off package.`
	};
};

/** Wires each hook to its event. */
export const register = (on: On): void => {
	on("session.start", announce);
	on("tool.run", { tool: "mcp__bedrock__run" }, answerRun);
	on("tool.call", { tool: "Bash" }, blockForeign);
};
