import { expect, test } from "bun:test";
import { join } from "node:path";

import { $ } from "bun";

import { tempRepo } from "./core/testing.ts";

const MAIN = join(import.meta.dir, "main.ts");

test("the usage aligns every command's summary, language subcommands included", async () => {
	// Arrange
	const command = $`bun ${MAIN} help`.quiet();

	// Act
	const { stdout } = await command;

	// Assert
	// A command row is indented and lowercase; its summary is the first capital after two spaces.
	const rows = stdout
		.toString()
		.split("\n")
		.filter((line) => /^ {2}[a-z]/u.test(line));
	const columns = new Set(rows.map((row) => row.search(/(?<= {2})[A-Z]/u)));
	expect(rows.join("\n")).toContain("jvm j2k <path>...");
	expect(columns.size).toBe(1);
});

test("a language subcommand exits 0 for --help instead of running", async () => {
	// Arrange
	const root = await tempRepo();

	// Act
	const result = await $`bun ${MAIN} jvm j2k --help`
		.cwd(root)
		.nothrow()
		.quiet();

	// Assert
	expect(result.exitCode).toBe(0);
});
