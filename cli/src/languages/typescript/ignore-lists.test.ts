import { describe, expect, test } from "bun:test";

import { mergeIgnoreList } from "./ignore-lists.ts";

describe("mergeIgnoreList", () => {
	test("keeps a multi-line list's trailing comma, as the formatter writes it", () => {
		// Arrange
		const template = '{\n\t"ignores": [\n\t\t"a/**",\n\t],\n}\n';

		// Act
		const merged = mergeIgnoreList(template, { ignores: ["b/**"] }, "ignores");

		// Assert
		expect(merged).toBe(
			'{\n\t"ignores": [\n\t\t"a/**",\n\t\t"b/**",\n\t],\n}\n',
		);
	});

	test("adds no trailing comma to a list without one, as JSON requires", () => {
		// Arrange
		const template = '{\n\t"ignorePatterns": [\n\t\t"a/**"\n\t]\n}\n';

		// Act
		const merged = mergeIgnoreList(
			template,
			{ ignorePatterns: ["b/**"] },
			"ignorePatterns",
		);

		// Assert
		expect(merged).toBe(
			'{\n\t"ignorePatterns": [\n\t\t"a/**",\n\t\t"b/**"\n\t]\n}\n',
		);
	});
});
