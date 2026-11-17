import { expect, test } from "bun:test";

import { $ } from "bun";

import { inOrder } from "../../core/order.ts";
import { tempRepo } from "../../core/testing.ts";
import { guardRust } from "./guard.ts";
import { adoptRust, contextIn } from "./harness.ts";
import { KINDS } from "./kinds.ts";

const hasCargo = Bun.which("cargo") !== null;

/** Runs a command in `root`, returning its exit code and output for diagnosis. */
const run = async (
	root: string,
	command: readonly string[],
): Promise<{ command: string; code: number; output: string }> => {
	const result = await $`${command}`.cwd(root).nothrow().quiet();
	return {
		command: command.join(" "),
		code: result.exitCode,
		output: result.stdout.toString() + result.stderr.toString(),
	};
};

const CHECKS = [
	["cargo", "fmt", "--all", "--check"],
	["cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"],
	["cargo", "test", "--workspace"],
] as const;

for (const kind of KINDS) {
	const skip = !hasCargo || (kind === "bun-ffi" && Bun.which("bun") === null);
	test.skipIf(skip)(
		`${kind}: init passes fmt, clippy, tests, and the guard`,
		async () => {
			// Arrange
			const root = await tempRepo();
			await adoptRust(root, true, { kind });
			await $`git add -A`.cwd(root).quiet();

			// Act
			// Sequential: the checks share one target directory.
			const results: { command: string; code: number; output: string }[] = [];
			await inOrder(CHECKS, async (command) => {
				results.push(await run(root, command));
			});
			const findings = await guardRust(contextIn(root, true, { kind }), false);

			// Assert
			expect(results.filter(({ code }) => code !== 0)).toEqual([]);
			expect(findings).toEqual([]);
		},
		300_000,
	);
}
