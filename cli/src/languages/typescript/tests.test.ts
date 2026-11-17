import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { migrateTestFile, testSteps } from "./tests.ts";

// Built at runtime so adopt does not migrate this file.
const VITEST = ["vi", "test"].join("");
const JEST = ["@jest", "globals"].join("/");

describe("migrateTestFile", () => {
	test("moves a framework import to bun:test", () => {
		expect(
			migrateTestFile(`import { describe, it, vi } from "${VITEST}";\n`),
		).toBe('import { describe, it, vi } from "bun:test";\n');
		expect(migrateTestFile(`const { jest } = require('${JEST}');\n`)).toBe(
			"const { jest } = require('bun:test');\n",
		);
	});

	test("ignores the name outside an import", () => {
		const text = `const name = "${VITEST}";\ntest("x", () => {});\n`;
		expect(migrateTestFile(text)).toStartWith(
			'import { test } from "bun:test";\n\n',
		);
	});

	test("imports the globals a file calls", () => {
		expect(
			migrateTestFile(
				"describe.each([1])('x', () => {\n\tit('y', () => expect(/a/.test('a')).toBe(true));\n});\n",
			),
		).toStartWith('import { describe, expect, it } from "bun:test";\n\n');
	});

	test("adds only the globals the file does not already import or declare", () => {
		// Arrange
		const partial =
			'import { expect, it, mockHost } from "./runtime.ts";\n\nit("x", () => expect(mockHost()).toBe(1));\ntest("y", () => {});\n';
		const bound =
			'import { expect as it } from "./runtime.ts";\nimport test from "./t.ts";\nit("x", () => {});\ntest("y", () => {});\n';
		const local = "const expect = () => 1;\nexpect(2);\n";

		// Act
		const migrated = migrateTestFile(partial);

		// Assert
		expect(migrated).toStartWith(
			'import { test } from "bun:test";\n\nimport { expect, it, mockHost }',
		);
		expect(migrateTestFile(bound)).toBe(bound);
		expect(migrateTestFile(local)).toBe(local);
	});

	test("leaves a bun:test file and a file without tests alone", () => {
		const bun = 'import { test } from "bun:test";\ntest("x", () => {});\n';
		expect(migrateTestFile(bun)).toBe(bun);
		expect(migrateTestFile("export const submit = () => it;\n")).toBe(
			"export const submit = () => it;\n",
		);
	});
});

describe("testSteps", () => {
	test("rewrites the test files outside node_modules", async () => {
		const jest = 'test("x", () => {});\n';
		const root = await tempRepo({
			"src/a.test.ts": jest,
			"src/b.test.ts": `import "bun:test";\n${jest}`,
			"node_modules/dep/c.test.ts": jest,
		});
		expect(await testSteps(root)).toEqual([
			{
				kind: "write",
				path: "src/a.test.ts",
				content: `import { test } from "bun:test";\n\n${jest}`,
			},
		]);
	});

	test("leaves the test files git ignores alone", async () => {
		const jest = 'test("x", () => {});\n';
		const root = await tempRepo({
			".gitignore": "captures/\n",
			"src/a.test.ts": jest,
			"captures/dep/c.test.ts": jest,
		});
		const steps = await testSteps(root);
		expect(steps.map((step) => step.path)).toEqual(["src/a.test.ts"]);
	});

	test("leaves the test files bunfig.toml says bun test skips", async () => {
		const jest = 'test("x", () => {});\n';
		const root = await tempRepo({
			"bunfig.toml": '[test]\npathIgnorePatterns = ["plugins/**"]\n',
			"src/a.test.ts": jest,
			"plugins/lab/c.test.ts": jest,
		});
		const steps = await testSteps(root);
		expect(steps.map((step) => step.path)).toEqual(["src/a.test.ts"]);
	});
});
