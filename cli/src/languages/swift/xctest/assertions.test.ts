import { describe, expect, test } from "bun:test";

import { converted, statement } from "./convert-helpers.ts";

const ASSERTION_CASES: readonly [string, string][] = [
	["XCTAssert(x)", "#expect(x)"],
	["XCTAssertTrue(x)", "#expect(x)"],
	['XCTAssertTrue(x, "why")', '#expect(x, "why")'],
	["XCTAssertTrue(a == b)", "#expect(a == b)"],
	["XCTAssertFalse(ok)", "#expect(!ok)"],
	["XCTAssertFalse(a.isEmpty)", "#expect(!a.isEmpty)"],
	["XCTAssertFalse(a && b)", "#expect(!(a && b))"],
	["XCTAssertEqual(a, b)", "#expect(a == b)"],
	["XCTAssertNotEqual(a, b)", "#expect(a != b)"],
	["XCTAssertGreaterThan(a, b)", "#expect(a > b)"],
	["XCTAssertGreaterThanOrEqual(a, b)", "#expect(a >= b)"],
	["XCTAssertLessThan(a, b)", "#expect(a < b)"],
	["XCTAssertLessThanOrEqual(a, b)", "#expect(a <= b)"],
	["XCTAssertIdentical(a, b)", "#expect(a === b)"],
	["XCTAssertNotIdentical(a, b)", "#expect(a !== b)"],
	["XCTAssertNil(a)", "#expect(a == nil)"],
	["XCTAssertNotNil(a)", "#expect(a != nil)"],
	["XCTAssertNil(a ?? b)", "#expect((a ?? b) == nil)"],
	["XCTAssertEqual(a, b, accuracy: 0.1)", "#expect(abs(a - b) <= 0.1)"],
	["XCTAssertNotEqual(a, b, accuracy: e)", "#expect(abs(a - b) > e)"],
	[
		"XCTAssertEqual(a + 1, b, accuracy: 0.1)",
		"#expect(abs((a + 1) - b) <= 0.1)",
	],
	["XCTAssertEqual(a + 1, b ?? 2)", "#expect((a + 1) == (b ?? 2))"],
	['XCTAssertEqual(x?.name, "a b")', '#expect(x?.name == "a b")'],
	["XCTAssertEqual(x, -1)", "#expect(x == -1)"],
	['XCTAssertEqual(a, b, "m", file: f, line: l)', '#expect(a == b, "m")'],
	["XCTAssertEqual(a, b, file: f, line: l)", "#expect(a == b)"],
	["XCTAssertTrue(x, message)", "#expect(x, Comment(rawValue: message))"],
	["XCTAssertEqual(1, try f())", "#expect(try 1 == f())"],
	["XCTAssertTrue(try await f())", "#expect(try await f())"],
	["XCTAssertEqual(await a, await b)", "#expect(await a == b)"],
	["let v = try XCTUnwrap(x)", "let v = try #require(x)"],
	['let v = try XCTUnwrap(x, "missing")', 'let v = try #require(x, "missing")'],
	['XCTFail("boom")', 'Issue.record("boom")'],
	["XCTFail()", "Issue.record()"],
	[
		String.raw`XCTFail("boom \(x)", file: f, line: l)`,
		String.raw`Issue.record("boom \(x)")`,
	],
	[
		"XCTAssertThrowsError(try f())",
		"#expect(throws: (any Error).self) { try f() }",
	],
	[
		'XCTAssertThrowsError(try f(), "m")',
		'#expect(throws: (any Error).self, "m") { try f() }',
	],
	["XCTAssertNoThrow(try f())", "#expect(throws: Never.self) { try f() }"],
	[
		"XCTAssertNoThrow(try await f())",
		"#expect(throws: Never.self) { try await f() }",
	],
];

describe("assertions", () => {
	test.each(ASSERTION_CASES)("%s", (input, expected) => {
		// Arrange / Act
		const result = statement(input);

		// Assert
		expect(result).toBe(expected);
	});

	test("leaves strings and comments alone", () => {
		const source = [
			'let a = "XCTAssertEqual(a, b)"',
			'let b = #"XCTAssertTrue("x")"#',
			"// XCTAssertEqual(a, b)",
			"/* XCTAssertTrue(x) /* nested */ XCTAssertFalse(y) */",
			'let c = """',
			"XCTAssertEqual(c, d)",
			'"""',
		].join("\n");

		expect(statement(source)).toBe(source);
	});

	test("reads past interpolation and escapes", () => {
		const code = String.raw`XCTAssertEqual("a\")), \(f("x,)"))", g(1, 2))`;

		expect(statement(code)).toBe(
			String.raw`#expect("a\")), \(f("x,)"))" == g(1, 2))`,
		);
	});

	test("keeps commas in nested arguments", () => {
		expect(statement("XCTAssertEqual(max(1, 2), 2)")).toBe(
			"#expect(max(1, 2) == 2)",
		);
		expect(
			statement('XCTAssertEqual([1, 2], [a: 1, b: 2]["a", default: 0])'),
		).toBe('#expect([1, 2] == [a: 1, b: 2]["a", default: 0])');
		expect(statement("XCTAssertEqual(xs.map { $0 + 1 }, [2, 3])")).toBe(
			"#expect((xs.map { $0 + 1 }) == [2, 3])",
		);
		expect(statement("XCTAssertEqual(xs.reduce(0) { a, b in a + b }, 3)")).toBe(
			"#expect((xs.reduce(0) { a, b in a + b }) == 3)",
		);
		expect(statement("XCTAssertEqual(Dictionary<String, Int>(), [:])")).toBe(
			"#expect(Dictionary<String, Int>() == [:])",
		);
	});

	test("rewrites multi-line assertions", () => {
		const code = 'XCTAssertEqual(\n  foo(1,\n      2),\n  bar,\n  "message"\n)';

		expect(statement(code)).toBe('#expect(foo(1,\n      2) == bar, "message")');
	});

	test("rewrites assertions nested in closures", () => {
		expect(statement("items.forEach { XCTAssertNotNil($0) }")).toBe(
			"items.forEach { #expect($0 != nil) }",
		);
		const handler =
			"XCTAssertThrowsError(try f()) { XCTAssertEqual($0 as? E, .bad) }";

		const { text } = converted(`import XCTest\n${handler}\n`);

		expect(text).toContain("{ #expect(($0 as? E) == .bad) }");
		expect(text).toContain("XCTAssertThrowsError(try f())");
	});

	test("leaves assertions with comments in their arguments", () => {
		const code = "XCTAssertEqual(a, // why\n  b)";

		const result = converted(`import XCTest\n${code}\n`);

		expect(result.text).toContain(code);
		expect(result.notes.length).toBeGreaterThan(0);
	});
});
