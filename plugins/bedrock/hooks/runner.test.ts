import { describe, expect, test } from "bun:test";

import cli from "../../../package.json" with { type: "json" };
import manifest from "../.claude-plugin/plugin.json" with { type: "json" };
import {
	BUILD,
	BUILT,
	engine,
	INSTALLED,
	machine,
	offline
} from "./fake-engine.ts";
import { assetName, findBedrock, VERSION } from "./runner.ts";

describe("version", () => {
	test("the plugin runs the release the CLI builds", () => {
		expect(VERSION).toBe(cli.version);
	});

	test("the plugin manifest names the release the plugin runs", () => {
		expect(manifest.version).toBe(VERSION);
	});
});
describe("assetName", () => {
	test.each([
		["Darwin", "arm64", "bedrock-darwin-arm64"],
		["Linux", "aarch64", "bedrock-linux-arm64"],
		["Linux", "x86_64", "bedrock-linux-x64"]
	])("names the %s %s asset", (system, arch, asset) => {
		expect(assetName(system, arch)).toBe(asset);
	});
});

describe("findBedrock", () => {
	test("builds and runs the source tree when the plugin sits in a bedrock checkout", async () => {
		const calls: string[][] = [];

		const command = await findBedrock(engine({ calls }));

		expect(command).toEqual([BUILT]);
		expect(calls).toEqual([BUILD]);
	});

	test("reports a source tree that does not build", async () => {
		const $ = engine({
			answer: () => ({ exitCode: 1, stdout: "", stderr: "syntax error\n" })
		});

		const failure = await findBedrock($).catch((error: unknown) => error);

		expect(String(failure)).toContain("syntax error");
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
			answer: machine("0.0.1")
		});
		const binary = `${INSTALLED}/.bedrock/${VERSION}/bedrock-darwin-arm64`;

		expect(await findBedrock($)).toEqual([binary]);
		expect(calls.at(-1)?.slice(3)).toEqual([
			"sh",
			binary,
			`https://github.com/xsyetopz/bedrock/releases/download/v${VERSION}`,
			"bedrock-darwin-arm64"
		]);
	});

	test("reuses a downloaded release", async () => {
		const calls: string[][] = [];
		const binary = `${INSTALLED}/.bedrock/${VERSION}/bedrock-darwin-arm64`;
		const $ = engine({
			root: INSTALLED,
			files: [binary],
			calls,
			answer: machine("")
		});

		expect(await findBedrock($)).toEqual([binary]);
		expect(calls.map((argv) => argv[0])).toEqual(["bedrock", "uname", "uname"]);
	});

	test("reports a failed download", async () => {
		const $ = engine({ root: INSTALLED, files: [], answer: offline("404\n") });

		const failure = await findBedrock($).catch((error: unknown) => error);

		expect(String(failure)).toContain("404");
	});
});
