import { describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

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
		const root = await mkdtemp(join(tmpdir(), "tests-test-"));
		const jest = 'test("x", () => {});\n';
		await Promise.all([
			Bun.write(join(root, "src/a.test.ts"), jest),
			Bun.write(join(root, "src/b.test.ts"), `import "bun:test";\n${jest}`),
			Bun.write(join(root, "node_modules/dep/c.test.ts"), jest),
		]);
		expect(await testSteps(root)).toEqual([
			{
				kind: "write",
				path: "src/a.test.ts",
				content: `import { test } from "bun:test";\n\n${jest}`,
			},
		]);
		await rm(root, { recursive: true });
	});
});
