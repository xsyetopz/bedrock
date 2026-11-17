import { describe, expect, test } from "bun:test";

import cli from "../../../package.json" with { type: "json" };
import type { Engine, Next, On, ProcessResult } from "./engine.ts";
import {
	announce,
	answerAdopt,
	answerBedrock,
	bedrockArgs,
	blockForeign,
	foreignPackageManager,
	register,
	runBedrock,
} from "./register.ts";
import { assetName, findBedrock, VERSION } from "./runner.ts";

const CHECKOUT = "/src/bedrock";
const ROOT = `${CHECKOUT}/plugins/bedrock`;
const INSTALLED = "/home/me/.claude/plugins/cache/bedrock";
const MAIN = `${CHECKOUT}/cli/src/main.ts`;

const OK: ProcessResult = { exitCode: 0, stdout: "", stderr: "" };

interface Fake {
	readonly root?: string;
	readonly files?: readonly string[];
	readonly calls?: string[][];
	/** Answers one process call; the default succeeds with no output. */
	readonly answer?: (argv: readonly string[]) => ProcessResult;
}

const engine = ({
	root = ROOT,
	files = [MAIN],
	calls = [],
	answer = () => OK,
}: Fake = {}): Engine => ({
	plugin: { name: "bedrock", root },
	command: { register: () => Promise.resolve({}) },
	fs: { exists: (path) => Promise.resolve(files.includes(path)) },
	process: {
		run: (argv) => {
			calls.push([...argv]);
			return Promise.resolve(answer(argv));
		},
	},
	session: { cwd: () => Promise.resolve("/work") },
});

const next =
	<E>(): Next<E> =>
	() =>
		Promise.resolve("ran");

/** The machine and PATH a plugin installed alone sees. */
const machine =
	(onPath: string) =>
	(argv: readonly string[]): ProcessResult => {
		const replies: Record<string, string> = {
			"bedrock version": onPath,
			"uname -m": "arm64",
			"uname -s": "Darwin",
		};
		return { ...OK, stdout: replies[argv.join(" ")] ?? "" };
	};

/** A machine where the release download fails with `stderr`. */
const offline =
	(stderr: string) =>
	(argv: readonly string[]): ProcessResult =>
		argv[0] === "sh" ? { exitCode: 1, stdout: "", stderr } : machine("")(argv);

describe("version", () => {
	test("the plugin runs the release the CLI builds", () => {
		expect(VERSION).toBe(cli.version);
	});
});

describe("bedrockArgs", () => {
	test.each([
		["adopt", ["adopt", "--dry-run"]],
		["init app typescript", ["init", "app", "typescript", "--dry-run"]],
		["update .", ["update", ".", "--dry-run"]],
	])("previews %j", (input, args) => {
		expect(bedrockArgs(input)).toEqual(args);
	});

	test.each([
		["adopt --yes", ["adopt", "--yes"]],
		["adopt --dry-run", ["adopt", "--dry-run"]],
		["guard  base", ["guard", "base"]],
		["", ["help"]],
	])("passes %j through", (input, args) => {
		expect(bedrockArgs(input)).toEqual(args);
	});
});

describe("assetName", () => {
	test.each([
		["Darwin", "arm64", "bedrock-darwin-arm64"],
		["Linux", "aarch64", "bedrock-linux-arm64"],
		["Linux", "x86_64", "bedrock-linux-x64"],
	])("names the %s %s asset", (system, arch, asset) => {
		expect(assetName(system, arch)).toBe(asset);
	});
});

describe("findBedrock", () => {
	test("runs the source tree when the plugin sits in a bedrock checkout", async () => {
		expect(await findBedrock(engine())).toEqual(["bun", MAIN]);
	});

	test("runs bedrock on PATH when its version matches", async () => {
		const $ = engine({ root: INSTALLED, files: [], answer: machine(VERSION) });
		expect(await findBedrock($)).toEqual(["bedrock"]);
	});

	test("downloads the pinned release once", async () => {
		const calls: string[][] = [];
		const $ = engine({
			root: INSTALLED,
			files: [],
			calls,
			answer: machine("0.0.1"),
		});
		const binary = `${INSTALLED}/.bedrock/${VERSION}/bedrock-darwin-arm64`;

		expect(await findBedrock($)).toEqual([binary]);
		expect(calls.at(-1)?.slice(3)).toEqual([
			"sh",
			binary,
			`https://github.com/xsyetopz/bedrock/releases/download/v${VERSION}`,
			"bedrock-darwin-arm64",
		]);
	});

	test("reuses a downloaded release", async () => {
		const calls: string[][] = [];
		const binary = `${INSTALLED}/.bedrock/${VERSION}/bedrock-darwin-arm64`;
		const $ = engine({
			root: INSTALLED,
			files: [binary],
			calls,
			answer: machine(""),
		});

		expect(await findBedrock($)).toEqual([binary]);
		expect(calls.map((argv) => argv[0])).toEqual(["bedrock", "uname", "uname"]);
	});

	test("reports a failed download", async () => {
		const $ = engine({ root: INSTALLED, files: [], answer: offline("404\n") });

		const failure = await findBedrock($).catch((error: unknown) => error);

		expect(failure).toHaveProperty(
			"message",
			`Downloading bedrock ${VERSION} (bedrock-darwin-arm64) failed: 404`,
		);
	});
});

describe("runBedrock", () => {
	test("runs bedrock in the session folder and says how to apply a plan", async () => {
		const calls: string[][] = [];
		const $ = engine({ calls, answer: () => ({ ...OK, stdout: "plan\n" }) });

		const result = await runBedrock($, "adopt rust");

		expect(calls).toEqual([
			[
				"sh",
				"-c",
				'cd "$1" && shift && exec "$@"',
				"sh",
				"/work",
				"bun",
				MAIN,
				"adopt",
				"rust",
				"--dry-run",
			],
		]);
		expect(result.text).toBe(
			"plan\n\nTo apply this plan, run /bedrock adopt rust --yes",
		);
	});

	test("reports the exit code of a failed run", async () => {
		const $ = engine({
			answer: () => ({ exitCode: 2, stdout: "", stderr: "no\n" }),
		});

		const { text } = await runBedrock($, "guard");

		expect(text).toBe("no\n(bedrock exited 2)");
	});

	test("reports a runner it cannot find", async () => {
		const $ = engine({
			root: INSTALLED,
			files: [],
			answer: offline("offline"),
		});

		expect(await runBedrock($, "guard")).toEqual({
			text: `Downloading bedrock ${VERSION} (bedrock-darwin-arm64) failed: offline`,
			context: [],
		});
	});
});

describe("foreignPackageManager", () => {
	test.each([
		["npm install", "npm"],
		["cd app && npx tsc", "npx"],
		["FOO=1 pnpm i", "pnpm"],
		["sudo yarn add x", "yarn"],
		["echo $(pnpx x)", "pnpx"],
		["ls\nyarn", "yarn"],
	])("finds the manager in %j", (command, manager) => {
		expect(foreignPackageManager(command)).toBe(manager);
	});

	test.each([
		"bun add npm-run-all",
		"bunx tsc",
		"grep yarn README.md",
		"cat package-lock.json",
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
				},
			},
		};

		expect(await announce($, {}, next())).toBe("ran");
		expect(registered).toEqual([
			expect.objectContaining({ name: "bedrock" }),
			expect.objectContaining({ name: "adopt" }),
		]);
	});

	test("answerBedrock passes the typed command through", async () => {
		const calls: string[][] = [];
		await answerBedrock(engine({ calls }), { args: "clean" }, next());
		expect(calls[0]?.slice(5)).toEqual(["bun", MAIN, "clean"]);
	});

	test("answerAdopt previews adopt with the typed arguments", async () => {
		const calls: string[][] = [];
		await answerAdopt(engine({ calls }), { args: "rust" }, next());
		expect(calls[0]?.slice(7)).toEqual(["adopt", "rust", "--dry-run"]);
	});

	test("blockForeign denies npm in a Bun repo only", async () => {
		const bun = engine({ files: ["/work/bun.lock"] });
		expect(await blockForeign(bun, { command: "npm ci" }, next())).toEqual({
			deny: "bedrock: this repo uses Bun. Run bun in place of npm, or bunx for a one-off package.",
		});
		expect(await blockForeign(bun, { command: "bun ci" }, next())).toBe("ran");
		expect(await blockForeign(engine(), { command: "npm ci" }, next())).toBe(
			"ran",
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
			["tool.call", { tool: "Bash" }, blockForeign],
		]);
	});
});
