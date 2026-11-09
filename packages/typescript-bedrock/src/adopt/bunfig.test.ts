import { describe, expect, test } from "bun:test";

import { mergeBunfig } from "./bunfig.ts";

const template = `# Header comment.
env = false

[test]
coverage = true
coverageReporter = ["text", "lcov"]

[fmt]
useTabs = true
ignorePatterns = [
	"**/dist/",
]

[[fmt.overrides]]
files = ["*.yml"]
options = { useTabs = false }
`;

describe("mergeBunfig", () => {
	test("writes the template when the repo has no bunfig.toml", () => {
		expect(mergeBunfig(template)).toEqual({ content: template, notes: [] });
	});

	test("keeps the keys and array entries a project adds, with their comments", () => {
		const current = `env = false

[test]
coverage = true
coverageReporter = ["text", "lcov"]
# Cached packages carry their own tests.
pathIgnorePatterns = ["captures/**"]

[fmt]
useTabs = true
ignorePatterns = [
	"**/dist/",
	# Built and committed.
	"cli/bin/",
]
`;
		const { content, notes } = mergeBunfig(template, current);
		expect(content).toBe(`# Header comment.
env = false

[test]
coverage = true
coverageReporter = ["text", "lcov"]
# Cached packages carry their own tests.
pathIgnorePatterns = ["captures/**"]

[fmt]
useTabs = true
ignorePatterns = [
	"**/dist/",
	# Built and committed.
	"cli/bin/",
]

[[fmt.overrides]]
files = ["*.yml"]
options = { useTabs = false }
`);
		expect(notes).toEqual([]);
		expect(mergeBunfig(template, content).content).toBe(content);
	});

	test("adds a project entry to a one-line template array", () => {
		const current = `[test]\ncoverageReporter = ["text", "junit"]\n`;
		const { content } = mergeBunfig(template, current);
		expect(content).toContain('coverageReporter = ["text", "lcov", "junit"]\n');
		expect(mergeBunfig(template, content).content).toBe(content);
	});

	test("keeps an entry on the line that opens a project array", () => {
		const current = `[fmt]\nignorePatterns = ["out/",\n\t"**/dist/",\n\t"gen/", # generated\n]\n`;
		const { content } = mergeBunfig(template, current);
		expect(content).toContain('\t"gen/", # generated\n\t"out/",\n]');
		expect(Bun.TOML.parse(content)).toMatchObject({
			fmt: { ignorePatterns: ["**/dist/", "gen/", "out/"] },
		});
	});

	test("keeps an entry split over lines", () => {
		const current = `[fmt]\nignorePatterns = [\n\t["a",\n\t"b"],\n]\n`;
		const { content } = mergeBunfig(template, current);
		expect(Bun.TOML.parse(content)).toMatchObject({
			fmt: { ignorePatterns: ["**/dist/", ["a", "b"]] },
		});
	});

	test("writes the template, with a note, over a copy that is not TOML", () => {
		expect(mergeBunfig(template, "[test\n")).toEqual({
			content: template,
			notes: [
				"bunfig.toml: the repo's copy is not valid TOML, so adopt wrote the template. Move its settings over from bunfig.toml.bak.",
			],
		});
	});

	test("appends a table and an override the template lacks", () => {
		const current = `[install.scopes]
# The company registry.
corp = "https://npm.example.com"

[[fmt.overrides]]
files = ["*.json"]
options = { printWidth = 120 }
`;
		const { content } = mergeBunfig(template, current);
		expect(content).toEndWith(`options = { useTabs = false }

[install.scopes]
# The company registry.
corp = "https://npm.example.com"

[[fmt.overrides]]
files = ["*.json"]
options = { printWidth = 120 }
`);
		expect(mergeBunfig(template, content).content).toBe(content);
	});

	test("keeps the template's value where the project changed one, with a note", () => {
		const current = `env = true\n\n[test]\ncoverage = false\n`;
		const { content, notes } = mergeBunfig(template, current);
		expect(content).toBe(template);
		expect(notes).toEqual([
			"bunfig.toml: kept the template's `env = false`; the repo had `env = true`.",
			"bunfig.toml: kept the template's `test.coverage = true`; the repo had `test.coverage = false`.",
		]);
	});
});
