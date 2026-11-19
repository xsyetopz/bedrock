import { describe, expect, test } from "bun:test";
import { join } from "node:path";

import { $ } from "bun";

import { tempRepo } from "../../core/testing.ts";
import { guardTypescript } from "./guard/index.ts";
import { adoptTypescript, commitAll, contextIn } from "./harness.ts";
import { asRecord, readJsonc } from "./json.ts";

const HAS_BUN = Bun.which("bun") !== null;

const check = (root: string, ...command: string[]) =>
	$`${command}`.cwd(root).nothrow().quiet();

describe("bedrock init typescript:<kind>", () => {
	test.skipIf(!HAS_BUN).each(["library", "cli", "plugin"])(
		"the %s kind installs, type checks, lints, tests, and passes the guard",
		async (kind) => {
			const root = await tempRepo();
			await adoptTypescript(root, kind, true);
			const install = await check(root, "bun", "install");
			await commitAll(root);

			const results = [
				install,
				await check(root, "bun", "check"),
				await check(root, "bun", "run", "oxlint"),
				await check(root, "bun", "test"),
				await check(root, "bun", "fmt", "--check")
			];
			expect(results.map((result) => result.exitCode)).toEqual([0, 0, 0, 0, 0]);
			expect(await guardTypescript(contextIn(root, kind))).toEqual([]);
		},
		180_000
	);

	test("the library kind exports its entry point and the cli kind its bin", async () => {
		const library = await tempRepo();
		const cli = await tempRepo();
		await adoptTypescript(library, "library", true);
		await adoptTypescript(cli, "cli", true);
		const libraryManifest = asRecord(
			await readJsonc(join(library, "package.json"))
		);
		const cliManifest = asRecord(await readJsonc(join(cli, "package.json")));
		expect(libraryManifest["exports"]).toBeDefined();
		expect(Object.values(asRecord(cliManifest["bin"]))).toContain(
			"./src/cli.ts"
		);
	});
});
