import { describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { asRecord, asStrings, exists, readJsonc, toJson } from "./json.ts";

describe("json helpers", () => {
	test("narrow unknown values", () => {
		expect(asRecord([1])).toEqual({});
		expect(asRecord({ a: 1 })).toEqual({ a: 1 });
		expect(asStrings(["a", 1, "b"])).toEqual(["a", "b"]);
		expect(asStrings("a")).toEqual([]);
		expect(toJson({ a: 1 })).toBe('{\n\t"a": 1\n}\n');
	});

	test("reads JSONC files and reports missing ones", async () => {
		const dir = await mkdtemp(join(tmpdir(), "json-test-"));
		await Bun.write(join(dir, "a.json"), '{ // note\n "a": 1, }');
		expect(await readJsonc(join(dir, "a.json"))).toEqual({ a: 1 });
		expect(await readJsonc(join(dir, "missing.json"))).toBeUndefined();
		expect(await exists(join(dir, "a.json"))).toBe(true);
		expect(await exists(join(dir, "missing.json"))).toBe(false);
		await rm(dir, { recursive: true, force: true });
	});
});
