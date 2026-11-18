import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { commitMsg, isConventional } from "./commit-msg.ts";

const roots: string[] = [];

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

describe("isConventional", () => {
	test.each([
		["a typed subject", "feat: add --dry-run\n"],
		["a scope", "fix(cli): read the config once\n"],
		["a breaking marker", "feat(cli)!: drop the old flags\n"],
		["a body after a blank line", "docs: say why\n\nBecause it matters.\n"],
		["an upper-case breaking footer", "feat: x\n\nBREAKING CHANGE: y\n"],
		["a fixup", "fixup! anything at all\n"],
		["a merge", "Merge branch 'main' into topic\n"],
		["comments git strips", "chore: tidy\n# Please enter the message\n"],
		[
			"text below the scissors line",
			`ci: pin\n# ------------------------ >8 ------------------------\n${"x".repeat(200)}\n`,
		],
		["100 characters, not bytes", `test: ${"é".repeat(94)}\n`],
	])("accepts %s", (_, message) => {
		// Arrange, Act
		const accepted = isConventional(message);

		// Assert
		expect(accepted).toBe(true);
	});

	test.each([
		["an unknown type", "update: things\n"],
		["no space after the colon", "feat:add\n"],
		["an empty description", "feat: \n"],
		["a body without a blank line", "feat: x\nmore\n"],
		["a line over 100 characters", `feat: x\n\n${"y".repeat(101)}\n`],
		["a lower-case breaking footer", "feat: x\n\nbreaking change: y\n"],
		["a conventional line below the subject", "wip\n\nfeat: x\n"],
		["an empty message", ""],
	])("rejects %s", (_, message) => {
		// Arrange, Act
		const accepted = isConventional(message);

		// Assert
		expect(accepted).toBe(false);
	});
});

describe("bedrock commit-msg", () => {
	test("exits 0 for a conventional message file", async () => {
		// Arrange
		const root = await mkdtemp(join(tmpdir(), "commit-msg-"));
		roots.push(root);
		const file = join(root, "COMMIT_EDITMSG");
		await Bun.write(file, "feat: add lefthook\n");

		// Act
		const code = await commitMsg([file]);

		// Assert
		expect(code).toBe(0);
	});

	test("exits 1 when the file is missing", async () => {
		// Arrange
		const file = join(tmpdir(), "commit-msg-missing", "COMMIT_EDITMSG");

		// Act
		const code = await commitMsg([file]);

		// Assert
		expect(code).toBe(1);
	});
});
