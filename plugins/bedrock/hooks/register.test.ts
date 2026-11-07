import { describe, expect, test } from "bun:test";

import type { Engine, Next, On, ProcessResult } from "./engine.ts";
import {
	adoptCommand,
	announce,
	answerAdopt,
	blockForeign,
	foreignPackageManager,
	parseArgs,
	register,
	runAdopt,
} from "./register.ts";

const ROOT = "/src/bedrock/plugins/bedrock";
const SOURCE = "/src/bedrock";

interface Fake {
	readonly results?: readonly ProcessResult[];
	readonly files?: readonly string[];
	readonly calls?: string[][];
	readonly root?: string;
	/** The tools `command -v` does not find. */
	readonly missing?: readonly string[];
}

const engine = ({
	results = [OK],
	files = [`${SOURCE}/packages/typescript-bedrock/package.json`],
	calls = [],
	root = ROOT,
	missing = [],
}: Fake = {}): Engine => {
	let run = 0;
	return {
		plugin: { name: "bedrock", root },
		command: { register: () => Promise.resolve({}) },
		fs: { exists: (path) => Promise.resolve(files.includes(path)) },
		process: {
			run: (argv) => {
				const tool = argv[0] === "sh" ? argv.at(-1) : undefined;
				if (tool !== undefined) {
					const exitCode = missing.includes(tool) ? 1 : 0;
					return Promise.resolve({ exitCode, stdout: "", stderr: "" });
				}
				calls.push([...argv]);
				const result = results[Math.min(run, results.length - 1)] ?? OK;
				run += 1;
				return Promise.resolve(result);
			},
		},
		session: { cwd: () => Promise.resolve("/work") },
	};
};

const next =
	<E>(): Next<E> =>
	() =>
		Promise.resolve("ran");

const OK: ProcessResult = { exitCode: 0, stdout: "plan\n", stderr: "" };

describe("parseArgs", () => {
	test("reads languages case-insensitively, in bedrock order", () => {
		expect(parseArgs("Swift TS rust app --yes")).toEqual({
			languages: ["typescript", "rust", "swift"],
			target: "app",
			cxx: false,
			apply: true,
		});
	});

	test.each(["cpp", "C++", "cxx", "ObjC"])(
		"%j means Swift with C++",
		(name) => {
			expect(parseArgs(name)).toMatchObject({
				languages: ["swift"],
				target: ".",
				cxx: true,
			});
		},
	);

	test("rejects unknown options and a second target", () => {
		expect(parseArgs("bun --force")).toStartWith("Unknown option --force.");
		expect(parseArgs("python app")).toStartWith(
			"Name one target, not python, app.",
		);
	});
});

describe("adoptCommand", () => {
	const request = parseArgs("/work/app --cxx");
	if (typeof request === "string") {
		throw new TypeError(request);
	}

	test("runs each bedrock from the checkout", () => {
		expect(adoptCommand(SOURCE, "typescript", request)).toEqual([
			"bun",
			`${SOURCE}/packages/typescript-bedrock/src/cli.ts`,
			"adopt",
			"/work/app",
			"--package",
			`file:${SOURCE}/packages/typescript-bedrock`,
			"--dry-run",
		]);
		expect(adoptCommand(SOURCE, "rust", request)).toEqual([
			"cargo",
			"run",
			"--quiet",
			"--manifest-path",
			`${SOURCE}/packages/rust-bedrock/Cargo.toml`,
			"--",
			"adopt",
			"/work/app",
			"--dry-run",
		]);
		expect(adoptCommand(SOURCE, "swift", { ...request, apply: true })).toEqual([
			"swift",
			"run",
			"--quiet",
			"--package-path",
			SOURCE,
			"swift-bedrock",
			"adopt",
			"/work/app",
			"--cxx",
			"--yes",
		]);
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

describe("runAdopt", () => {
	test("detects the languages and says how to apply the plan", async () => {
		const calls: string[][] = [];
		const files = [
			`${SOURCE}/packages/typescript-bedrock/package.json`,
			"/work/app/Package.swift",
			"/work/app/package.json",
		];
		const result = await runAdopt(engine({ calls, files }), "app");
		expect(calls.map(([tool]) => tool)).toEqual(["bun", "swift"]);
		expect(calls.map((argv) => argv.includes("/work/app"))).toEqual([
			true,
			true,
		]);
		expect(result.text).toBe(
			"typescript-bedrock:\nplan\n\nswift-bedrock:\nplan\n\nTo apply this plan, run /adopt typescript swift app --yes",
		);
	});

	test("asks for languages when it finds none", async () => {
		const calls: string[][] = [];
		const result = await runAdopt(engine({ calls }), "");
		expect(result.text).toStartWith(". has no package.json");
		expect(calls).toEqual([]);
	});

	test("prints the parse error", async () => {
		const result = await runAdopt(engine(), "--force");
		expect(result.text).toStartWith("Unknown option --force.");
		expect(result.context).toEqual([]);
	});

	test("names the missing tools and runs nothing", async () => {
		const calls: string[][] = [];
		const result = await runAdopt(
			engine({ calls, missing: ["just", "cargo"] }),
			"rust",
		);
		expect(result.text).toBe(
			"Install these, then run /adopt again:\n- cargo: rustup from https://rustup.rs\n- just: brew install just, or cargo install --locked just",
		);
		expect(result.context).toEqual([`/adopt rust:\n${result.text}`]);
		expect(calls).toEqual([]);
	});

	test("stops at the first failure", async () => {
		const calls: string[][] = [];
		const failed = { exitCode: 1, stdout: "", stderr: "no Cargo.toml\n" };
		const result = await runAdopt(
			engine({ calls, results: [OK, failed] }),
			"rust bun /x --yes",
		);
		expect(calls.map(([tool]) => tool)).toEqual(["bun", "cargo"]);
		expect(result.text).toBe(
			"typescript-bedrock:\nplan\n\nrust-bedrock (exited 1):\nno Cargo.toml",
		);
	});

	test("clones the repository when the plugin was installed alone", async () => {
		const calls: string[][] = [];
		const root = "/cache/bedrock";
		await runAdopt(engine({ calls, files: [], root }), "rust");
		expect(calls[0]).toEqual([
			"git",
			"clone",
			"--quiet",
			"--depth",
			"1",
			"https://github.com/xsyetopz/bedrock",
			`${root}/.bedrock`,
		]);
		expect(calls[1]).toContain(
			`${root}/.bedrock/packages/rust-bedrock/Cargo.toml`,
		);
	});

	test("reports a failed clone", async () => {
		const failed = { exitCode: 128, stdout: "", stderr: "offline\n" };
		const error = await runAdopt(
			engine({ files: [], results: [failed], root: "/c" }),
			"rust",
		).catch((error_: unknown) => error_);
		expect(error).toBeInstanceOf(Error);
		expect(String(error)).toContain("failed: offline");
	});
});

describe("hooks", () => {
	test("announce registers /adopt, then continues", async () => {
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
		expect(registered).toEqual([expect.objectContaining({ name: "adopt" })]);
	});

	test("answerAdopt runs adopt with the typed arguments", async () => {
		const calls: string[][] = [];
		await answerAdopt(engine({ calls }), { args: "rust" }, next());
		expect(calls.map(([tool]) => tool)).toEqual(["cargo"]);
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
			["command.run", { command: "adopt" }, answerAdopt],
			["tool.call", { tool: "Bash" }, blockForeign],
		]);
	});
});
