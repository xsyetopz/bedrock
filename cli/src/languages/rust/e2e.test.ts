import { expect, test } from "bun:test";
import { appendFile } from "node:fs/promises";
import { basename, join } from "node:path";

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

test.skipIf(!hasCargo)(
	"a dependency the crate never uses fails cargo check",
	async () => {
		// Arrange
		const root = await tempRepo();
		await adoptRust(root, true, { kind: "lib" });
		await Bun.write(
			join(root, "crates/unused/Cargo.toml"),
			'[package]\nname = "unused"\nversion = "0.1.0"\nedition = "2024"\npublish = false\n',
		);
		await Bun.write(join(root, "crates/unused/src/lib.rs"), "");
		await appendFile(
			join(root, "crates", basename(root), "Cargo.toml"),
			'\n[dependencies]\nunused = { path = "../unused" }\n',
		);

		// Act
		const result = await run(root, ["cargo", "check", "--workspace"]);

		// Assert
		expect(result.code).not.toBe(0);
		expect(result.output).toContain("cargo::unused_dependencies");
	},
	300_000,
);
