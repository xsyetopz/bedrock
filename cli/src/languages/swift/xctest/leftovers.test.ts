import { describe, expect, test } from "bun:test";

import { converted, fixture } from "./convert-helpers.ts";
import { convertXCTest } from "./convert.ts";

describe("leftovers", () => {
	test.each([
		["try XCTSkipIf(flag)", "XCTSkipIf has no direct Swift Testing form"],
		[
			"try XCTSkipUnless(flag)",
			"XCTSkipUnless has no direct Swift Testing form",
		],
		['throw XCTSkip("no")', "XCTSkip has no direct Swift Testing form"],
		[
			'let e = expectation(description: "x")',
			"expectation(description:) has no direct Swift Testing form; use confirmation(...)",
		],
		[
			"wait(for: [e], timeout: 1)",
			"wait(for:) has no direct Swift Testing form",
		],
		["e.fulfill()", "fulfill() has no direct Swift Testing form"],
		[
			'let e = XCTestExpectation(description: "x")',
			"XCTestExpectation has no direct",
		],
		["measure { work() }", "measure has no direct Swift Testing form"],
		[
			"self.measure(metrics: m) { work() }",
			"measure has no direct Swift Testing form",
		],
		[
			"let app = XCUIApplication()",
			"XCUIApplication has no direct Swift Testing form",
		],
		[
			"addTeardownBlock { cleanup() }",
			"addTeardownBlock has no direct Swift Testing form",
		],
		[
			"continueAfterFailure = false",
			"continueAfterFailure has no direct Swift Testing form",
		],
		[
			"XCTAssertThrowsError(try f()) { _ in }",
			"XCTAssertThrowsError with an error handler",
		],
	])("notes %s and keeps XCTest", (input, note) => {
		const source = `import XCTest\nfinal class T: XCTestCase {\n  func testA() {\n    ${input}\n  }\n}\n`;

		const result = converted(source);

		expect(
			result.text.startsWith(
				"import Testing\nimport XCTest\nfinal class T {\n",
			),
		).toBe(true);
		expect(result.text).toContain(`@Test\n  func a() {\n    ${input}\n`);
		expect(result.notes.join("\n")).toContain(`line 6: ${note}`);
		expect(result.notes).toContain(
			"line 2: import XCTest kept for the constructs noted here",
		);
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
			converted("import Testing\nimport XCTest\nXCTAssertTrue(a)\n").text,
		).toBe("import Testing\n#expect(a)\n");
	});
});

describe("whole files", () => {
	test("returns nothing without an XCTest import", () => {
		for (const source of [
			"import Testing\n@Test func a() {}\n",
			'let s = "import XCTest"\n',
			"// import XCTest\n",
			"",
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
			"import Testing\r\nfinal class T {\r\n  @Test\r\n  func a() {}\r\n}\r\n",
		);
	});

	test("converts the legacy fixture to the committed file", async () => {
		const result = converted(await fixture("legacy-tests.txt"));

		expect(result.text).toBe(await fixture("converted-legacy-tests.txt"));
		expect(result.notes).toEqual([]);
	});
});
