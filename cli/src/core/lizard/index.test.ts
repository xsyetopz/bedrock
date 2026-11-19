import { describe, expect, test } from "bun:test";

import {
	editorconfigTree,
	parseEditorconfig,
	propertiesFor
} from "../editorconfig/index.ts";
import { tempRepo } from "../testing.ts";
import { checkSize, judge } from "./index.ts";
import { findLizard } from "./measure.ts";

const hasLizard = (await findLizard()) !== undefined;

const BOUNDS_TEXT = `root = true

[*.{java,kt}]
bedrock_file_code_lines_warning = 603
bedrock_function_code_lines_error = 56
bedrock_function_complexity_error = 15
bedrock_function_parameters_error = 7

[*.rs]
bedrock_file_code_lines_warning = 603
bedrock_test_file_code_lines_warning = 1000
`;

const BOUNDS = parseEditorconfig(BOUNDS_TEXT);

const boundsOf = (path: string): Map<string, string> =>
	propertiesFor(BOUNDS, path);

const MEASURES = {
	files: [
		{ path: "src/Big.kt", nloc: 700 },
		{ path: "src/Small.kt", nloc: 20 }
	],
	functions: [
		{ path: "src/Big.kt", line: 9, name: "run", nloc: 80, ccn: 3, params: 8 },
		{ path: "src/Small.kt", line: 1, name: "ok", nloc: 5, ccn: 1, params: 1 }
	]
};

describe("judge", () => {
	test("fails over-long functions and only warns about long files", () => {
		// Act
		const report = judge(MEASURES, boundsOf);

		// Assert
		expect(report.errors).toEqual([
			"src/Big.kt:9: run code lines is 80, over 56",
			"src/Big.kt:9: run parameters is 8, over 7"
		]);
		expect(report.warnings).toEqual([
			"src/Big.kt: file code lines is 700, over 603"
		]);
	});

	test("bounds a test file by the test file bound in place of the file one", () => {
		// Arrange
		const rust = {
			files: [
				{ path: "src/lib.rs", nloc: 700 },
				{ path: "src/a/checks.rs", nloc: 700 },
				{ path: "src/b/tests.rs", nloc: 1200 }
			],
			functions: []
		};
		const tests = new Set(["src/a/checks.rs", "src/b/tests.rs"]);

		// Act
		const report = judge(rust, boundsOf, (path) => tests.has(path));

		// Assert
		expect(report.warnings).toEqual([
			"src/lib.rs: file code lines is 700, over 603",
			"src/b/tests.rs: test file code lines is 1200, over 1000"
		]);
	});

	test("bounds only the metrics a path's sections set", () => {
		// Arrange
		const rust = {
			files: [{ path: "src/lib.rs", nloc: 700 }],
			functions: [
				{
					path: "src/lib.rs",
					line: 3,
					name: "run",
					nloc: 90,
					ccn: 30,
					params: 9
				}
			]
		};

		// Act
		const report = judge(rust, boundsOf);

		// Assert
		expect(report.errors).toEqual([]);
		expect(report.warnings).toEqual([
			"src/lib.rs: file code lines is 700, over 603"
		]);
	});
});

const longMethod = (name: string): string => {
	const body = Array.from(
		{ length: 60 },
		(_, index) => `    int v${String(index)} = ${String(index)};`
	);
	return `class ${name} {\n  void run() {\n${body.join("\n")}\n  }\n}\n`;
};

const rustFile = (lines: number, attribute: string): string =>
	`${attribute}\nfn run() {\n${Array.from({ length: lines }, (_, index) => `    let v${String(index)} = ${String(index)};`).join("\n")}\n}\n`;

/** Picks `checks.rs` as the one test file, in place of a language's picker. */
const checksOnly = (
	_root: string,
	files: readonly string[]
): Promise<Set<string>> =>
	Promise.resolve(new Set(files.filter((file) => file.endsWith("checks.rs"))));

describe("checkSize", () => {
	test.skipIf(!hasLizard)(
		"bounds the files the language picks as tests by the test file bound",
		async () => {
			// Arrange
			const root = await tempRepo({
				".editorconfig":
					"root = true\n\n[*.rs]\nbedrock_file_code_lines_warning = 20\nbedrock_test_file_code_lines_warning = 50\n",
				"src/live.rs": rustFile(30, "#[inline]"),
				"src/checks.rs": rustFile(30, "#[test]")
			});
			// Act
			const report = await checkSize(
				root,
				["rust"],
				editorconfigTree(root),
				checksOnly
			);

			// Assert
			expect(report.warnings).toHaveLength(1);
			expect(report.warnings[0]).toStartWith("src/live.rs: file code lines");
		},
		120_000
	);

	test.skipIf(!hasLizard)(
		"reports a long Java method, except where a nested .editorconfig unsets the bound",
		async () => {
			// Arrange
			const root = await tempRepo({
				".editorconfig": BOUNDS_TEXT,
				"src/A.java": longMethod("A"),
				"legacy/.editorconfig":
					"[*.java]\nbedrock_function_code_lines_error = unset\n",
				"legacy/B.java": longMethod("B")
			});

			// Act
			const report = await checkSize(root, ["java"], editorconfigTree(root));

			// Assert
			expect(report.errors).toHaveLength(1);
			expect(report.errors[0]).toStartWith("src/A.java:2: ");
		},
		120_000
	);
});
