import { describe, expect, test } from "bun:test";
import { join } from "node:path";

import { tempRepo } from "../core/testing.ts";
import { findBackups, removeBackups } from "./clean.ts";

describe("removeBackups", () => {
	test("deletes backups git can restore and keeps the rest", async () => {
		const root = await tempRepo({ Makefile: "all:\n" });
		await Bun.write(join(root, "Makefile.bak"), "all:\n");
		await Bun.write(join(root, "notes.bak"), "only here\n");
		await Bun.write(join(root, "node_modules/x/y.bak"), "dep\n");
		expect(await removeBackups(root, false)).toEqual({
			removed: ["Makefile.bak"],
			kept: ["notes.bak"],
		});
		expect(await findBackups(root)).toEqual(["notes.bak"]);
	});

	test("deletes every backup with force", async () => {
		const root = await tempRepo();
		await Bun.write(join(root, "notes.bak"), "only here\n");
		expect(await removeBackups(root, true)).toEqual({
			removed: ["notes.bak"],
			kept: [],
		});
	});
});
