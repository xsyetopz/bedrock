import { describe, expect, test } from "bun:test";

import { parseNamed, select } from "./select.ts";

const ids = (selected: ReturnType<typeof select>): string[] =>
	selected.map((entry) => entry.language.id);

describe("parseNamed", () => {
	test("rejects an unknown language with the known names", () => {
		expect(() => parseNamed("cobol")).toThrow(/Unknown language cobol/u);
	});
});

describe("select", () => {
	test("merges configured and named languages in block order", () => {
		const config = { version: "0.2.0", languages: { jvm: { kind: "x" } } };
		const selected = select(config, [parseNamed("rust")]);
		expect(ids(selected)).toEqual(["rust", "jvm"]);
		expect(selected[1]?.options).toEqual({ kind: "x" });
	});

	test("adds cpp for --cxx", () => {
		expect(ids(select(undefined, [], { cxx: true }))).toContain("cpp");
	});
});
