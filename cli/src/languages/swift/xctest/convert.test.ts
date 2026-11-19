import { describe, expect, test } from "bun:test";

import { converted, members } from "./convert-helpers.ts";

describe("structure", () => {
	test("keeps @testable imports", () => {
		expect(converted("import XCTest\n@testable import Lib\n").text).toBe(
			"import Testing\n@testable import Lib\n",
		);
	});

	test.each([
		["class FooTests: XCTestCase {", "final class FooTests {"],
		["final class FooTests: XCTestCase {", "final class FooTests {"],
		[
			"@MainActor final class FooTests: XCTestCase {",
			"@MainActor final class FooTests {",
		],
		[
			"@MainActor\nclass FooTests: XCTestCase {",
			"@MainActor\nfinal class FooTests {",
		],
		[
			"final class FooTests: XCTestCase, Sendable {",
			"final class FooTests: Sendable {",
		],
		[
			"class FooTests: Base, XCTestCase, P {",
			"final class FooTests: Base, P {",
		],
		["public class FooTests : XCTestCase{", "public final class FooTests {"],
		["class Helper: NSObject {", "class Helper: NSObject {"],
	])("class declaration %s", (input, expected) => {
		expect(converted(`import XCTest\n${input}\n}\n`).text).toBe(
			`import Testing\n${expected}\n}\n`,
		);
	});

	test("converts tests", () => {
		const body = members(
			[
				"  func testAddsNumbers() {}",
				"  func testLoads() async throws {}",
				"  func test_snake_case() throws {}",
				"  func testURLParsing() {}",
			].join("\n"),
		).body;

		expect(body).toBe(
			[
				"  @Test",
				"  func addsNumbers() {}",
				"  @Test",
				"  func loads() async throws {}",
				"  @Test",
				"  func snake_case() throws {}",
				"  @Test",
				"  func uRLParsing() {}",
			].join("\n"),
		);
	});

	test("puts @Test on its own line at the same indentation", () => {
		expect(members("    @MainActor func testIt() {}").body).toBe(
			"    @Test\n    @MainActor func it() {}",
		);
	});

	test("keeps the prefix on a collision", () => {
		const { body } = members(
			[
				"  func testFoo() {}",
				"  func foo() {}",
				"  func testBar() {}",
				"  var bar = 1",
				"  func testBaz() { baz(1) }",
				"  func testQux() {}",
				"  func test_qux() {}",
				"  func testFree() { _ = free }",
			].join("\n"),
		);

		for (const kept of [
			"func testFoo() {}",
			"func testBar() {}",
			"func testBaz() { baz(1) }",
			"func testQux() {}",
			"func test_qux() {}",
			"func free() { _ = free }",
		]) {
			expect(body).toContain(kept);
		}
	});

	test("keeps the prefix when the name would be invalid", () => {
		const names = ["test", "test1", "testDefault", "testInit", "test_"];

		const { body } = members(
			names.map((name) => `  func ${name}() {}`).join("\n"),
		);

		for (const name of names) {
			expect(body).toContain(`@Test\n  func ${name}() {}`);
		}
	});

	test("ignores functions XCTest would not run", () => {
		const { body } = members(
			[
				"  func testWithArgument(_ x: Int) {}",
				"  private func testHelper() {}",
				"  static func testStatic() {}",
			].join("\n"),
		);

		expect(body).not.toContain("@Test");
	});

	test("converts tests in extensions of converted classes", () => {
		const source =
			"import XCTest\nclass A: XCTestCase {}\nextension A {\n  func testB() {}\n}\n";

		expect(converted(source).text).toContain(
			"extension A {\n  @Test\n  func b() {}\n}",
		);
	});

	test("leaves functions of other classes alone", () => {
		const source = "import XCTest\nclass A {\n  func testB() {}\n}\n";

		expect(converted(source).text).toBe(
			"import Testing\nclass A {\n  func testB() {}\n}\n",
		);
	});

	test.each([
		["override func setUp()", "init()"],
		["override func setUp() async", "init() async"],
		["override func setUpWithError() throws", "init() throws"],
		["override func setUp() async throws", "init() async throws"],
		["override func tearDown()", "deinit"],
		["override func tearDownWithError()", "deinit"],
	])("lifecycle %s", (input, expected) => {
		const call = `super.${input.slice(0, input.indexOf("(")).split(" ").pop()}()`;

		const result = members(`  ${input} {\n    ${call}\n    work()\n  }`);

		expect(result.body).toBe(`  ${expected} {\n    work()\n  }`);
		expect(result.notes).toEqual([]);
	});

	test("removes every super call form", () => {
		const lines = [
			"  override func setUp() async throws {",
			"    try await super.setUp()",
			"    try super.setUpWithError()",
			"    await super.setUp()",
			"    super.setUp();",
			"    keep()",
			"  }",
		];

		expect(members(lines.join("\n")).body).toBe(
			"  init() async throws {\n    keep()\n  }",
		);
	});

	test("leaves a tearDown that cannot become deinit", () => {
		const lines = [
			"  override func tearDownWithError() throws {",
			"    try super.tearDownWithError()",
			"  }",
			"  override func tearDown() async {",
			"    await cleanup()",
			"  }",
		];

		const result = members(lines.join("\n"));

		expect(result.body).toContain("override func tearDownWithError() throws {");
		expect(result.body).toContain("override func tearDown() async {");
		expect(result.notes).toHaveLength(2);
		expect(result.notes[0]?.startsWith("line 3: ")).toBe(true);
	});

	test("converts only one init and one deinit", () => {
		const lines = [
			"  override func setUp() {}",
			"  override func setUpWithError() throws {}",
			"  override func tearDown() {}",
			"  override func tearDownWithError() {}",
		];

		const result = members(lines.join("\n"));

		expect(result.body).toContain(
			"  init() {}\n  override func setUpWithError() throws {}",
		);
		expect(result.body).toContain(
			"  deinit {}\n  override func tearDownWithError() {}",
		);
		expect(result.notes).toHaveLength(2);
	});
});
