import { describe, expect, test } from "bun:test";

import type { Engine, On } from "./engine.ts";
import {
	BUILD,
	BUILT,
	engine,
	INSTALLED,
	next,
	offline,
	OK,
	shellMissing,
	typed
} from "./fake-engine.ts";
import {
	announce,
	answerAdopt,
	answerBedrock,
	bedrockArgs,
	blockForeign,
	foreignPackageManager,
	register,
	runBedrock
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

		const result = await runBedrock($, "adopt rust");

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
		expect(result.text).toContain("plan\n");
	});

	test("reports the exit code of a failed run", async () => {
		const $ = engine({
			answer: (argv) =>
				argv[0] === "go" ? OK : { exitCode: 2, stdout: "", stderr: "no\n" }
		});

		const { text } = await runBedrock($, "guard");

		expect(text).toContain("no\n");
		expect(text).toContain("2");
	});

	test("reports a runner it cannot find", async () => {
		const $ = engine({
			root: INSTALLED,
			files: [],
			answer: offline("offline")
		});

		const result = await runBedrock($, "guard");

		expect(result.text).toContain("offline");
		expect(result.context).toEqual([]);
	});

	test("reports a run that cannot start with the command to retype", async () => {
		const $ = engine({ answer: shellMissing });

		const result = await runBedrock($, "adopt rust --yes");

		expect(result.text).toContain("ENOENT");
		expect(result.text).toContain("/bedrock adopt rust --yes");
	});

	test("names the typed command when bedrock cannot be found", async () => {
		const $ = engine({
			root: INSTALLED,
			files: [],
			answer: offline("offline")
		});

		const { text } = await runBedrock($, "init go --yes");

		expect(text).toContain("/bedrock init go --yes");
	});

	test("puts a typed command back in the prompt box when nothing ran", async () => {
		const fills: string[] = [];
		const $ = engine({ answer: shellMissing, fills });

		const { text } = await runBedrock(
			$,
			"adopt rust --yes",
			typed("adopt", "rust --yes")
		);

		expect(fills).toEqual(["/adopt rust --yes"]);
		expect(text).toContain("back in the prompt box");
	});

	test("keeps a draft the user typed since", async () => {
		const fills: string[] = [];
		const $ = engine({ answer: shellMissing, draft: "next idea", fills });

		const { text } = await runBedrock($, "guard", typed("bedrock", "guard"));

		expect(fills).toEqual([]);
		expect(text).toContain("To try again, run: /bedrock guard");
	});

	test("restores no command it did not get from the prompt", async () => {
		const fills: string[] = [];
		const $ = engine({ answer: shellMissing, fills });

		await runBedrock($, "guard", {
			...typed("bedrock", "guard"),
			origin: { kind: "plugin" }
		});

		expect(fills).toEqual([]);
	});

	test("leaves the prompt box alone after bedrock ran", async () => {
		const fills: string[] = [];
		const $ = engine({
			answer: (argv) =>
				argv[0] === "go" ? OK : { exitCode: 1, stdout: "", stderr: "no\n" },
			fills
		});

		await runBedrock($, "guard", typed("bedrock", "guard"));

		expect(fills).toEqual([]);
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
	test("announce registers /bedrock and /adopt, then continues", async () => {
		const registered: unknown[] = [];
		const $: Engine = {
			...engine(),
			command: {
				register: (command) => {
					registered.push(command);
					return Promise.resolve({});
				}
			}
		};

		expect(await announce($, {}, next())).toBe("ran");
		expect(registered).toEqual([
			expect.objectContaining({ name: "bedrock" }),
			expect.objectContaining({ name: "adopt" })
		]);
	});

	test("answerBedrock passes the typed command through", async () => {
		const calls: string[][] = [];
		await answerBedrock(engine({ calls }), typed("bedrock", "clean"), next());
		expect(calls.at(-1)?.slice(5)).toEqual([BUILT, "clean"]);
	});

	test("answerAdopt previews adopt with the typed arguments", async () => {
		const calls: string[][] = [];
		await answerAdopt(engine({ calls }), typed("adopt", "rust"), next());
		expect(calls.at(-1)?.slice(6)).toEqual(["adopt", "rust", "--dry-run"]);
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
			["command.run", { command: "bedrock" }, answerBedrock],
			["command.run", { command: "adopt" }, answerAdopt],
			["tool.call", { tool: "Bash" }, blockForeign]
		]);
	});
});
