import type { BashCall, Engine, Hook, On, ProcessResult } from "./engine.ts";
import { findBedrock } from "./runner.ts";

// A package manager in command position: at a line start or after ; & | ( $( or `, past env assignments and sudo, exec, or command.
const FOREIGN =
	/(?:^|[;&|(`]|\$\()\s*(?:\w+=\S*\s+)*(?:(?:sudo|exec|command)\s+)?(npm|npx|pnpm|pnpx|yarn)(?=\s|$)/mu;

/** Commands that change files: the plugin previews them unless the user passes --yes. */
const PLANNED = new Set(["adopt", "init", "update"]);

/**
 * The bedrock arguments for one slash command. A planned command without
 * --yes gets --dry-run, so typing it never changes files by surprise.
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

const applyHint = (args: readonly string[]): string =>
	args.includes("--dry-run") && PLANNED.has(args[0] ?? "")
		? `\n\nTo apply this plan, run /bedrock ${args.filter((arg) => arg !== "--dry-run").join(" ")} --yes`
		: "";

/**
 * Runs bedrock with the arguments of `/bedrock` or `/adopt`, in the session's folder.
 * A run that can't start reports the typed command, since the prompt it came from is gone.
 */
export const runBedrock = async (
	$: Engine,
	input: string
): Promise<{ text: string; context: string[] }> => {
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
		const typed = `/bedrock ${input.trim()}`.trim();
		return {
			text: `${reason}\n\nNothing ran. To try again, run: ${typed}`,
			context: []
		};
	}
	const { exitCode, stdout, stderr } = result;
	const status = exitCode === 0 ? "" : `\n(bedrock exited ${exitCode})`;
	const text = `${`${stdout}${stderr}`.trim()}${status}${exitCode === 0 ? applyHint(args) : ""}`;
	return { text, context: [`/bedrock ${args.join(" ")}:\n${text}`] };
};

/** The package manager a Bash command runs, if it is one Bun replaces. */
export const foreignPackageManager = (command: string): string | undefined =>
	FOREIGN.exec(command)?.[1];

/** Registers the slash commands, then continues. */
export const announce: Hook<unknown> = async ($, e, next) => {
	await Promise.all([
		$.command.register({
			name: "bedrock",
			description:
				"Run bedrock: init, adopt, update, guard, clean. Plans print first; --yes applies them."
		}),
		$.command.register({
			name: "adopt",
			description:
				"Set this repo up with bedrock for its languages. Prints the plan; --yes applies it."
		})
	]);
	return next(e);
};

/** `/bedrock <command> [args]`. */
export const answerBedrock: Hook<{ readonly args: string }> = ($, e) =>
	runBedrock($, e.args);

/** `/adopt [args]`, short for `/bedrock adopt [args]`. */
export const answerAdopt: Hook<{ readonly args: string }> = ($, e) =>
	runBedrock($, `adopt ${e.args}`);

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
		deny: `${$.plugin.name}: this repo uses Bun. Run bun in place of ${manager}, or bunx for a one-off package.`
	};
};

/** Wires each hook to its event. */
export const register = (on: On): void => {
	on("session.start", announce);
	on("command.run", { command: "bedrock" }, answerBedrock);
	on("command.run", { command: "adopt" }, answerAdopt);
	on("tool.call", { tool: "Bash" }, blockForeign);
};
