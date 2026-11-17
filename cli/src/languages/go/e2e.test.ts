import { expect, test } from "bun:test";

import { $ } from "bun";

import { tempRepo } from "../../core/testing.ts";
import { guardGo } from "./guard.ts";
import { adoptGo, commitAll, contextIn } from "./harness.ts";

const hasGo = Bun.which("go") !== null;
const hasLint = Bun.which("golangci-lint") !== null;

/** Runs `command` in `root` and returns its exit code with its output. */
const run = async (command: readonly string[], root: string) => {
	const child = await $`${command}`.cwd(root).nothrow().quiet();
	return {
		code: child.exitCode,
		output: child.stdout.toString() + child.stderr.toString(),
	};
};

test.skipIf(!hasGo).each(["command", "library"])(
	"init %s builds, tests, and keeps go.mod tidy",
	async (kind) => {
		// Arrange
		const root = await tempRepo();
		await adoptGo(root, { kind, init: true });
		await commitAll(root);

		// Act
		const build = await run(["go", "build", "./..."], root);
		const tests = await run(["go", "test", "./..."], root);
		const tidy = await run(["go", "mod", "tidy", "-diff"], root);
		const found = await guardGo(contextIn(root, kind));

		// Assert
		expect(build.output).toBe("");
		expect(tests.code).toBe(0);
		expect(tidy.code).toBe(0);
		expect(found).toEqual([]);
	},
	300_000,
);

test.skipIf(!hasGo || !hasLint).each(["command", "library"])(
	"init %s passes the format and lint recipes' commands",
	async (kind) => {
		// Arrange
		const root = await tempRepo();
		await adoptGo(root, { kind, init: true });
		await commitAll(root);

		// Act
		const format = await run(["golangci-lint", "fmt", "--diff"], root);
		const lint = await run(["golangci-lint", "run"], root);

		// Assert
		expect(format.output).toBe("");
		expect(lint.output).toContain("0 issues");
		expect(lint.code).toBe(0);
	},
	600_000,
);
