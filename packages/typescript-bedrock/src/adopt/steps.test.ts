import { expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe as describeStep } from "./steps.ts";

test("describe names each change and the backup it keeps", async () => {
	const root = await mkdtemp(join(tmpdir(), "typescript-bedrock-steps-"));
	await Bun.write(join(root, "old.txt"), "old");
	expect(
		await Promise.all([
			describeStep(root, {
				kind: "link",
				path: "CLAUDE.md",
				target: "AGENTS.md",
			}),
			describeStep(root, { kind: "remove", path: "old.txt" }),
			describeStep(root, { kind: "write", path: "old.txt", content: "new" }),
			describeStep(root, { kind: "write", path: "new.txt", content: "new" }),
		]),
	).toEqual([
		"link    CLAUDE.md -> AGENTS.md",
		"remove  old.txt (kept as old.txt.bak)",
		"update  old.txt (kept as old.txt.bak)",
		"create  new.txt",
	]);
	await rm(root, { recursive: true });
});
