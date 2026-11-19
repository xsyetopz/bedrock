import { describe, expect, test } from "bun:test";

import { boundsOf, crossed, invalidBounds } from "./limits.ts";

describe("boundsOf", () => {
	test("reads each metric's warning and error from bedrock_* properties", () => {
		// Arrange
		const properties = new Map([
			["indent_size", "4"],
			["bedrock_function_code_lines_warning", "40"],
			["bedrock_function_code_lines_error", "80"],
			["bedrock_file_code_lines_warning", "603"]
		]);

		// Act
		const bounds = boundsOf(properties);

		// Assert
		expect(bounds).toEqual({
			function_code_lines: { warning: 40, error: 80 },
			file_code_lines: { warning: 603 }
		});
	});
});

describe("invalidBounds", () => {
	test.each([
		["bedrock_file_code_lines_warning", "603"],
		["bedrock_function_complexity_error", "unset"],
		["bedrock_version", "0.5.0"],
		["bedrock_jvm_group", "com.example.app"],
		["indent_size", "2"]
	])("accepts %s = %s", (key, value) => {
		expect(invalidBounds([[key, value]])).toEqual([]);
	});

	test.each([
		["bedrock_file_code_lines_warning", "lots"],
		["bedrock_function_parameters_error", "0"],
		["bedrock_function_lines_error", "56"]
	])("rejects %s = %s", (key, value) => {
		expect(invalidBounds([[key, value]])).toHaveLength(1);
	});
});

describe("crossed", () => {
	test.each([
		[56, { error: 56 }, undefined],
		[57, { error: 56 }, { level: "error" as const, limit: 56 }],
		[700, { warning: 603 }, { level: "warning" as const, limit: 603 }],
		[90, { warning: 40, error: 80 }, { level: "error" as const, limit: 80 }],
		[50, { warning: 40, error: 80 }, { level: "warning" as const, limit: 40 }],
		[1000, {}, undefined]
	])("%d against %j", (value, bound, expected) => {
		expect(crossed(value, bound)).toEqual(expected);
	});
});
