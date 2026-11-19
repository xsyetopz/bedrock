import { describe, expect, test } from "bun:test";

import { tempRepo } from "../testing.ts";
import { editorconfigTree, parseEditorconfig, propertiesFor } from "./index.ts";

const TEXT = `root = true

# >>> jvm-bedrock
[*.{java,kt}]
Indent_Size = 4
bedrock_file_code_lines_warning = 603
# <<< jvm-bedrock

; generated code
[gen/**]
bedrock_file_code_lines_warning = unset

[/src/Big.kt]
bedrock_file_code_lines_warning = 900
`;

describe("parseEditorconfig", () => {
	test("keeps sections in order, lowercases keys, and skips the preamble and comments", () => {
		// Act
		const sections = parseEditorconfig(TEXT);

		// Assert
		expect(sections.map((section) => section.glob)).toEqual([
			"*.{java,kt}",
			"gen/**",
			"/src/Big.kt",
		]);
		expect(sections[0]?.pairs[0]).toEqual(["indent_size", "4"]);
	});
});

describe("propertiesFor", () => {
	const sections = parseEditorconfig(TEXT);

	test.each([
		["app/src/A.kt", "603"],
		["A.java", "603"],
		["gen/B.kt", undefined],
		["src/Big.kt", "900"],
		["sub/src/Big.kt", "603"],
		["README.md", undefined],
	])("%s gets a file bound of %s", (path, expected) => {
		expect(
			propertiesFor(sections, path).get("bedrock_file_code_lines_warning"),
		).toBe(expected);
	});
});

describe("editorconfigTree", () => {
	const KEY = "bedrock_file_code_lines_warning";

	test.each([
		["src/A.kt", "603"],
		["legacy/A.kt", "900"],
		["legacy/gen/A.kt", undefined],
		["vendor/A.kt", undefined],
	])("%s gets a file bound of %s", async (path, expected) => {
		// Arrange
		const root = await tempRepo({
			".editorconfig": `root = true\n\n[*.kt]\n${KEY} = 603\n`,
			"legacy/.editorconfig": `[*.kt]\n${KEY} = 900\n\n[gen/**]\n${KEY} = unset\n`,
			"vendor/.editorconfig": "root = true\n\n[*]\nindent_size = 2\n",
		});

		// Act
		const properties = await editorconfigTree(root).properties(path);

		// Assert
		expect(properties.get(KEY)).toBe(expected);
	});

	test("lists the root file and each nested one a path read", async () => {
		// Arrange
		const root = await tempRepo({
			".editorconfig": "root = true\n",
			"legacy/.editorconfig": "[*]\n",
			"other/.editorconfig": "[*]\n",
		});
		const tree = editorconfigTree(root);
		await tree.properties("legacy/A.kt");

		// Act
		const files = await tree.files();

		// Assert
		expect(files.map((file) => file.path).toSorted()).toEqual([
			".editorconfig",
			"legacy/.editorconfig",
		]);
	});
});
