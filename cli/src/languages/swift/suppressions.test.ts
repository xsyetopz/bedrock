import { describe, expect, test } from "bun:test";

import { importsXCTest, isUITest, suppressions } from "./suppressions.ts";

describe("suppressions", () => {
	test("flags swift-format-ignore-file", () => {
		expect(suppressions("// swift-format-ignore-file\nlet a = 1\n")).toEqual([
			"1: swift-format-ignore-file; ignore the one declaration instead",
		]);
	});

	test("flags a disable without an enable", () => {
		const found = suppressions("// swiftlint:disable force_cast\nlet a = 1\n");

		expect(found).toEqual([
			"1: swiftlint:disable without a matching swiftlint:enable; use swiftlint:disable:next",
		]);
	});

	test("accepts a disable the file enables again", () => {
		const text =
			"// swiftlint:disable force_cast\nlet a = 1\n// swiftlint:enable force_cast\n";

		expect(suppressions(text)).toEqual([]);
	});

	test("an enable of other rules does not cover the disable", () => {
		const text =
			"// swiftlint:disable force_cast\n// swiftlint:enable line_length\n";

		expect(suppressions(text)).toHaveLength(1);
	});

	test.each(["next", "this", "previous"])(
		"accepts swiftlint:disable:%s",
		(scope) => {
			expect(
				suppressions(`let a = 1 // swiftlint:disable:${scope} x\n`),
			).toEqual([]);
		},
	);

	test("ignores the text in a string", () => {
		expect(suppressions('let a = "swiftlint:disable x"\n')).toEqual([]);
	});
});

describe("isUITest", () => {
	test.each([
		["AppUITests/Login.swift", true],
		["Tests/AppUITest/Login.swift", true],
		["Tests/AppTests/Login.swift", false],
		["UITests.swift", false],
	])("%s is %s", (path, expected) => {
		expect(isUITest(path)).toBe(expected);
	});
});

describe("importsXCTest", () => {
	test.each([
		"import XCTest",
		"  import XCTest // comment",
		"@testable import XCTest",
		"@preconcurrency import XCTest",
		"import struct XCTest",
	])("matches %s", (line) => {
		expect(importsXCTest(`${line}\n`)).toBe(true);
	});

	test.each(["import XCTestSupport", "import Testing", "// import XCTest"])(
		"does not match %s",
		(line) => {
			expect(importsXCTest(`${line}\n`)).toBe(false);
		},
	);
});
