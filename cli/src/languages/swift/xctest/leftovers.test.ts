import { describe, expect, test } from "bun:test";

import { converted, fixture } from "./convert-helpers.ts";
import { convertXCTest } from "./convert.ts";

describe("leftovers", () => {
	test.each([
		"try XCTSkipIf(flag)",
		"try XCTSkipUnless(flag)",
		'throw XCTSkip("no")',
		'let e = expectation(description: "x")',
		"wait(for: [e], timeout: 1)",
		"e.fulfill()",
		'let e = XCTestExpectation(description: "x")',
		"measure { work() }",
		"self.measure(metrics: m) { work() }",
		"let app = XCUIApplication()",
		"addTeardownBlock { cleanup() }",
		"continueAfterFailure = false",
		"XCTAssertThrowsError(try f()) { _ in }"
	])("notes %s and keeps XCTest", (input) => {
		const source = `import XCTest\nfinal class T: XCTestCase {\n  func testA() {\n    ${input}\n  }\n}\n`;

		const result = converted(source);

		expect(
			result.text.startsWith("import Testing\nimport XCTest\nfinal class T {\n")
		).toBe(true);
		expect(result.text).toContain(`@Test\n  func a() {\n    ${input}\n`);
		expect(result.notes.some((note) => note.startsWith("line 6: "))).toBe(true);
		expect(result.notes.some((note) => note.startsWith("line 2: "))).toBe(true);
	});

	test("does not flag unrelated methods", () => {
		const rest =
			"let t = ruler.measure(x) + clock.wait(1)\nobj.expectation(1)\n";

		const result = converted(`import XCTest\n${rest}`);

		expect(result.text).toBe(`import Testing\n${rest}`);
		expect(result.notes).toEqual([]);
	});

	test("drops the XCTest import when nothing remains", () => {
		const result = converted("import XCTest\nXCTAssertTrue(a)\n");

		expect(result.notes).toEqual([]);
		expect(result.text).not.toContain("import XCTest");
	});

	test("reuses an existing Testing import", () => {
		expect(
			converted("import Testing\nimport XCTest\nXCTAssertTrue(a)\n").text
		).toBe("import Testing\n#expect(a)\n");
	});
});

describe("whole files", () => {
	test("returns nothing without an XCTest import", () => {
		for (const source of [
			"import Testing\n@Test func a() {}\n",
			'let s = "import XCTest"\n',
			"// import XCTest\n",
			""
		]) {
			expect(convertXCTest(source)).toBeUndefined();
		}
	});

	test("is idempotent", async () => {
		const once = converted(await fixture("legacy-tests.txt"));

		expect(convertXCTest(once.text)).toBeUndefined();
	});

	test("is idempotent when XCTest is kept", () => {
		const source =
			"import XCTest\nclass T: XCTestCase {\n  func testA() {\n    measure {}\n  }\n}\n";
		const once = converted(source);

		const twice = converted(once.text);

		expect(twice.text).toBe(once.text);
		expect(twice.notes).toEqual(once.notes);
	});

	test("keeps Windows line endings", () => {
		const source =
			"import XCTest\r\nclass T: XCTestCase {\r\n  func testA() {}\r\n}\r\n";

		expect(converted(source).text).toBe(
			"import Testing\r\nfinal class T {\r\n  @Test\r\n  func a() {}\r\n}\r\n"
		);
	});

	test("converts the legacy fixture to the committed file", async () => {
		const result = converted(await fixture("legacy-tests.txt"));

		expect(result.text).toBe(await fixture("converted-legacy-tests.txt"));
		expect(result.notes).toEqual([]);
	});
});
