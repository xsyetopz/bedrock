import { describe, expect, test } from "bun:test";

import { $ } from "bun";

import { tempRepo } from "../../core/testing.ts";
import { guardTypescript } from "./guard/index.ts";
import { adoptTypescript, commitAll, contextIn, textOf } from "./harness.ts";

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
				await check(root, "bun", "fmt", "--check"),
			];
			expect(results.map((result) => result.exitCode)).toEqual([0, 0, 0, 0, 0]);
			expect(await guardTypescript(contextIn(root, kind))).toEqual([]);
		},
		180_000,
	);

	test("the library kind exports its entry point and the cli kind its bin", async () => {
		const library = await tempRepo();
		const cli = await tempRepo();
		await adoptTypescript(library, "library", true);
		await adoptTypescript(cli, "cli", true);
		expect(await textOf(library, "package.json")).toContain('"exports"');
		expect(await textOf(cli, "package.json")).toContain('"./src/cli.ts"');
	});
});
