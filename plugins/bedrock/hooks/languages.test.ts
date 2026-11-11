import { describe, expect, test } from "bun:test";

import { MARKERS, NEEDS, RUNNERS, TOOLS } from "./languages.ts";
import { adoptCommand, parseArgs } from "./register.ts";

describe("go", () => {
	test("reads go and golang as one language, last in bedrock order", () => {
		expect(parseArgs("golang rust go app")).toMatchObject({
			languages: ["rust", "go"],
			target: "app",
		});
	});

	test("is found by go.mod and needs the go tool", () => {
		expect(MARKERS.go).toEqual(["go.mod"]);
		expect(NEEDS.go).toBe("go");
		expect(TOOLS["go"]).toContain("go.dev");
	});

	test("runs go-bedrock from the checkout", () => {
		const request = {
			languages: ["go"],
			target: "/work/app",
			cxx: false,
			apply: false,
		} as const;
		expect(RUNNERS.go("/src/bedrock")).toEqual([
			"go",
			"run",
			"-C",
			"/src/bedrock/packages/go-bedrock",
			"./cmd/go-bedrock",
		]);
		expect(adoptCommand("/src/bedrock", "go", request).slice(5)).toEqual([
			"adopt",
			"/work/app",
			"--dry-run",
		]);
	});
});
