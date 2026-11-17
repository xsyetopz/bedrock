import { describe, expect, test } from "bun:test";
import { lstat, readlink } from "node:fs/promises";
import { join } from "node:path";

import { applySteps, dedupe, pending, type Step } from "./steps.ts";
import { tempRepo } from "./testing.ts";

describe("dedupe", () => {
	test("keeps the last step per path and sorts by path", () => {
		const steps: Step[] = [
			{ kind: "write", path: "b", content: "base" },
			{ kind: "write", path: "a", content: "a" },
			{ kind: "write", path: "b", content: "language" },
		];
		expect(dedupe(steps)).toEqual([
			{ kind: "write", path: "a", content: "a" },
			{ kind: "write", path: "b", content: "language" },
		]);
	});
});

describe("applySteps", () => {
	test("writes, links, and removes without leaving a backup", async () => {
		const root = await tempRepo({ "old.txt": "old\n", "dir/x": "x\n" });
		const steps: Step[] = [
			{ kind: "write", path: "old.txt", content: "new\n" },
			{
				kind: "write",
				path: "hooks/run",
				content: "#!/bin/sh\n",
				executable: true,
			},
			{ kind: "link", path: "CLAUDE.md", target: "AGENTS.md" },
			{ kind: "remove", path: "dir" },
		];
		await applySteps(root, steps);
		expect(await Bun.file(join(root, "old.txt")).text()).toBe("new\n");
		const hook = await lstat(join(root, "hooks/run"));
		expect(hook.mode & 0o111).not.toBe(0);
		expect(await readlink(join(root, "CLAUDE.md"))).toBe("AGENTS.md");
		expect(await Bun.file(join(root, "old.txt.bak")).exists()).toBe(false);
		expect(await pending(root, steps)).toEqual([]);
	});

	test("replaces a directory that sits where a file goes", async () => {
		const root = await tempRepo({ "x/inner": "i\n" });
		await applySteps(root, [{ kind: "write", path: "x", content: "file\n" }]);
		expect(await Bun.file(join(root, "x")).text()).toBe("file\n");
	});
});
