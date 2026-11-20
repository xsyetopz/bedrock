import { describe, expect, test } from "bun:test";

import type { On } from "./engine.ts";
import {
	BUILD,
	BUILT,
	engine,
	INSTALLED,
	next,
	offline,
	OK,
	shellMissing
} from "./fake-engine.ts";
import {
	announce,
	answerRun,
	bedrockArgs,
	blockForeign,
	foreignPackageManager,
	register,
	runBedrock,
	TOOL
} from "./register.ts";

describe("bedrockArgs", () => {
	test.each([
		["adopt", ["adopt", "--dry-run"]],
		["init app typescript", ["init", "app", "typescript", "--dry-run"]],
		["update .", ["update", ".", "--dry-run"]]
	])("previews %j", (input, args) => {
		expect(bedrockArgs(input)).toEqual(args);
	});

	test.each([
		["adopt --yes", ["adopt", "--yes"]],
		["adopt --dry-run", ["adopt", "--dry-run"]],
		["guard  base", ["guard", "base"]],
		["", ["help"]]
	])("passes %j through", (input, args) => {
		expect(bedrockArgs(input)).toEqual(args);
	});
});
describe("runBedrock", () => {
	test("runs bedrock in the session folder with a dry run", async () => {
		const calls: string[][] = [];
		const $ = engine({ calls, answer: () => ({ ...OK, stdout: "plan\n" }) });

		const outcome = await runBedrock($, "adopt rust");

		expect(calls).toEqual([
			BUILD,
			[
				"sh",
				"-c",
				'cd "$1" && shift && exec "$@"',
				"sh",
				"/work",
				BUILT,
				"adopt",
				"rust",
				"--dry-run"
			]
		]);
		expect(outcome).toEqual({
			text: "$ bedrock adopt rust --dry-run\nplan",
			isError: false
		});
	});

	test("reports the exit code of a failed run as an error", async () => {
		const $ = engine({
			answer: (argv) =>
				argv[0] === "go" ? OK : { exitCode: 2, stdout: "", stderr: "no\n" }
		});

		const outcome = await runBedrock($, "guard");

		expect(outcome).toEqual({
			text: "$ bedrock guard\nno\n(bedrock exited 2)",
			isError: true
		});
	});

	test("reports a runner it cannot find as an error", async () => {
		const $ = engine({
			root: INSTALLED,
			files: [],
			answer: offline("offline")
		});

		const outcome = await runBedrock($, "guard");

		expect(outcome.isError).toBe(true);
		expect(outcome.text).toContain("offline");
	});

	test("reports a run that cannot start as an error", async () => {
		const $ = engine({ answer: shellMissing });

		const outcome = await runBedrock($, "adopt rust --yes");

		expect(outcome.isError).toBe(true);
		expect(outcome.text).toContain("bedrock did not run");
		expect(outcome.text).toContain("ENOENT");
	});
});

describe("foreignPackageManager", () => {
	test.each([
		["npm install", "npm"],
		["cd app && npx tsc", "npx"],
		["FOO=1 pnpm i", "pnpm"],
		["sudo yarn add x", "yarn"],
		["echo $(pnpx x)", "pnpx"],
		["ls\nyarn", "yarn"]
	])("finds the manager in %j", (command, manager) => {
		expect(foreignPackageManager(command)).toBe(manager);
	});

	test.each([
		"bun add npm-run-all",
		"bunx tsc",
		"grep yarn README.md",
		"cat package-lock.json"
	])("passes %j", (command) => {
		expect(foreignPackageManager(command)).toBeUndefined();
	});
});

describe("hooks", () => {
	test("announce registers the run tool up front, then continues", async () => {
		const tools: unknown[] = [];

		const result = await announce(engine({ tools }), {}, next());

		expect(result).toBe("ran");
		expect(tools).toEqual([
			expect.objectContaining({ name: TOOL, isDeferred: false })
		]);
	});

	test("answerRun returns bedrock's output for the call", async () => {
		const calls: string[][] = [];

		const answer = await answerRun(
			engine({ calls }),
			{ args: "clean" },
			next()
		);

		expect(calls.at(-1)?.slice(5)).toEqual([BUILT, "clean"]);
		expect(answer).toEqual({ result: "$ bedrock clean\n" });
	});

	test("answerRun marks a failed run as an error", async () => {
		const answer = await answerRun(
			engine({ answer: shellMissing }),
			{ args: "guard" },
			next()
		);

		expect(answer).toHaveProperty("isError", true);
	});

	test("blockForeign denies npm in a Bun repo only", async () => {
		const bun = engine({ files: ["/work/bun.lock"] });
		expect(
			await blockForeign(bun, { command: "npm ci" }, next())
		).toHaveProperty("deny");
		expect(await blockForeign(bun, { command: "bun ci" }, next())).toBe("ran");
		expect(await blockForeign(engine(), { command: "npm ci" }, next())).toBe(
			"ran"
		);
	});

	test("register wires each hook to its event", () => {
		const wired: unknown[][] = [];
		const on: On = (...args: unknown[]) => {
			wired.push(args);
		};
		register(on);
		expect(wired).toEqual([
			["session.start", announce],
			["tool.run", { tool: `mcp__bedrock__${TOOL}` }, answerRun],
			["tool.call", { tool: "Bash" }, blockForeign]
		]);
	});
});
