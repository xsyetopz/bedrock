import Foundation
import Testing

@testable import SwiftBedrock

private func converted(_ source: String) throws -> (text: String, notes: [String]) {
  try #require(XCTestConverter.convert(source))
}

/// Converts a statement and returns the statement that replaced it.
private func statement(_ code: String) throws -> String {
  let text = try converted("import XCTest\n\(code)\n").text
  return String(text.dropFirst("import Testing\n".count).dropLast())
}

/// Converts members placed in an XCTestCase and returns the converted class body.
private func members(_ code: String) throws -> (body: String, notes: [String]) {
  let result = try converted("import XCTest\nfinal class T: XCTestCase {\n\(code)\n}\n")
  let prefix = "import Testing\nfinal class T {\n"
  #expect(result.text.hasPrefix(prefix))
  return (String(result.text.dropFirst(prefix.count).dropLast(3)), result.notes)
}

func packageFile(_ path: String) throws -> String {
  let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
    .deletingLastPathComponent().deletingLastPathComponent()
  return try String(contentsOf: root.appendingPathComponent(path), encoding: .utf8)
}

@Suite
struct Assertions {
  @Test(
    arguments: [
      ("XCTAssert(x)", "#expect(x)"),
      ("XCTAssertTrue(x)", "#expect(x)"),
      ("XCTAssertTrue(x, \"why\")", "#expect(x, \"why\")"),
      ("XCTAssertTrue(a == b)", "#expect(a == b)"),
      ("XCTAssertFalse(ok)", "#expect(!ok)"),
      ("XCTAssertFalse(a.isEmpty)", "#expect(!a.isEmpty)"),
      ("XCTAssertFalse(a && b)", "#expect(!(a && b))"),
      ("XCTAssertEqual(a, b)", "#expect(a == b)"),
      ("XCTAssertNotEqual(a, b)", "#expect(a != b)"),
      ("XCTAssertGreaterThan(a, b)", "#expect(a > b)"),
      ("XCTAssertGreaterThanOrEqual(a, b)", "#expect(a >= b)"),
      ("XCTAssertLessThan(a, b)", "#expect(a < b)"),
      ("XCTAssertLessThanOrEqual(a, b)", "#expect(a <= b)"),
      ("XCTAssertIdentical(a, b)", "#expect(a === b)"),
      ("XCTAssertNotIdentical(a, b)", "#expect(a !== b)"),
      ("XCTAssertNil(a)", "#expect(a == nil)"),
      ("XCTAssertNotNil(a)", "#expect(a != nil)"),
      ("XCTAssertNil(a ?? b)", "#expect((a ?? b) == nil)"),
      ("XCTAssertEqual(a, b, accuracy: 0.1)", "#expect(abs(a - b) <= 0.1)"),
      ("XCTAssertNotEqual(a, b, accuracy: e)", "#expect(abs(a - b) > e)"),
      ("XCTAssertEqual(a + 1, b, accuracy: 0.1)", "#expect(abs((a + 1) - b) <= 0.1)"),
      ("XCTAssertEqual(a + 1, b ?? 2)", "#expect((a + 1) == (b ?? 2))"),
      ("XCTAssertEqual(x?.name, \"a b\")", "#expect(x?.name == \"a b\")"),
      ("XCTAssertEqual(x, -1)", "#expect(x == -1)"),
      ("XCTAssertEqual(a, b, \"m\", file: f, line: l)", "#expect(a == b, \"m\")"),
      ("XCTAssertEqual(a, b, file: f, line: l)", "#expect(a == b)"),
      ("XCTAssertTrue(x, message)", "#expect(x, Comment(rawValue: message))"),
      ("XCTAssertEqual(1, try f())", "#expect(try 1 == f())"),
      ("XCTAssertTrue(try await f())", "#expect(try await f())"),
      ("XCTAssertEqual(await a, await b)", "#expect(await a == b)"),
      ("let v = try XCTUnwrap(x)", "let v = try #require(x)"),
      ("let v = try XCTUnwrap(x, \"missing\")", "let v = try #require(x, \"missing\")"),
      ("XCTFail(\"boom\")", "Issue.record(\"boom\")"),
      ("XCTFail()", "Issue.record()"),
      ("XCTFail(\"boom \\(x)\", file: f, line: l)", "Issue.record(\"boom \\(x)\")"),
      ("XCTAssertThrowsError(try f())", "#expect(throws: (any Error).self) { try f() }"),
      (
        "XCTAssertThrowsError(try f(), \"m\")",
        "#expect(throws: (any Error).self, \"m\") { try f() }"
      ),
      ("XCTAssertNoThrow(try f())", "#expect(throws: Never.self) { try f() }"),
      ("XCTAssertNoThrow(try await f())", "#expect(throws: Never.self) { try await f() }"),
    ]
  )
  func converts(input: String, expected: String) throws {
    #expect(try statement(input) == expected)
  }

  @Test
  func leavesStringsAndCommentsAlone() throws {
    let lines = [
      "let a = \"XCTAssertEqual(a, b)\"",
      "let b = #\"XCTAssertTrue(\"x\")\"#",
      "// XCTAssertEqual(a, b)",
      "/* XCTAssertTrue(x) /* nested */ XCTAssertFalse(y) */",
      "let c = \"\"\"",
      "XCTAssertEqual(c, d)",
      "\"\"\"",
    ]
    let source = lines.joined(separator: "\n")
    #expect(try statement(source) == source)
  }

  @Test
  func readsPastInterpolationAndEscapes() throws {
    let code = "XCTAssertEqual(\"a\\\")), \\(f(\"x,)\"))\", g(1, 2))"
    let expected = "#expect(\"a\\\")), \\(f(\"x,)\"))\" == g(1, 2))"
    #expect(try statement(code) == expected)
  }

  @Test
  func keepsCommasInNestedArguments() throws {
    #expect(try statement("XCTAssertEqual(max(1, 2), 2)") == "#expect(max(1, 2) == 2)")
    #expect(
      try statement("XCTAssertEqual([1, 2], [a: 1, b: 2][\"a\", default: 0])")
        == "#expect([1, 2] == [a: 1, b: 2][\"a\", default: 0])"
    )
    #expect(
      try statement("XCTAssertEqual(xs.map { $0 + 1 }, [2, 3])")
        == "#expect((xs.map { $0 + 1 }) == [2, 3])"
    )
    #expect(
      try statement("XCTAssertEqual(xs.reduce(0) { a, b in a + b }, 3)")
        == "#expect((xs.reduce(0) { a, b in a + b }) == 3)"
    )
    #expect(
      try statement("XCTAssertEqual(Dictionary<String, Int>(), [:])")
        == "#expect(Dictionary<String, Int>() == [:])"
    )
  }

  @Test
  func rewritesMultiLineAssertions() throws {
    let code = "XCTAssertEqual(\n  foo(1,\n      2),\n  bar,\n  \"message\"\n)"
    #expect(try statement(code) == "#expect(foo(1,\n      2) == bar, \"message\")")
  }

  @Test
  func rewritesAssertionsNestedInClosures() throws {
    let code = "items.forEach { XCTAssertNotNil($0) }"
    #expect(try statement(code) == "items.forEach { #expect($0 != nil) }")
    let handler = "XCTAssertThrowsError(try f()) { XCTAssertEqual($0 as? E, .bad) }"
    let result = try converted("import XCTest\n\(handler)\n")
    #expect(result.text.contains("{ #expect(($0 as? E) == .bad) }"))
    #expect(result.text.contains("XCTAssertThrowsError(try f())"))
  }

  @Test
  func leavesAssertionsWithCommentsInArguments() throws {
    let code = "XCTAssertEqual(a, // why\n  b)"
    let result = try converted("import XCTest\n\(code)\n")
    #expect(result.text.contains(code))
    #expect(result.notes.contains { $0.contains("XCTAssertEqual could not be converted") })
  }
}

@Suite
struct Structure {
  @Test
  func importsKeepTestable() throws {
    let source = "import XCTest\n@testable import Lib\n"
    #expect(try converted(source).text == "import Testing\n@testable import Lib\n")
  }

  @Test(
    arguments: [
      ("class FooTests: XCTestCase {", "final class FooTests {"),
      ("final class FooTests: XCTestCase {", "final class FooTests {"),
      ("@MainActor final class FooTests: XCTestCase {", "@MainActor final class FooTests {"),
      ("@MainActor\nclass FooTests: XCTestCase {", "@MainActor\nfinal class FooTests {"),
      ("final class FooTests: XCTestCase, Sendable {", "final class FooTests: Sendable {"),
      ("class FooTests: Base, XCTestCase, P {", "final class FooTests: Base, P {"),
      ("public class FooTests : XCTestCase{", "public final class FooTests {"),
      ("class Helper: NSObject {", "class Helper: NSObject {"),
    ]
  )
  func convertsClassDeclarations(input: String, expected: String) throws {
    let result = try converted("import XCTest\n\(input)\n}\n")
    #expect(result.text == "import Testing\n\(expected)\n}\n")
  }

  @Test
  func convertsTests() throws {
    let body = try members(
      [
        "  func testAddsNumbers() {}",
        "  func testLoads() async throws {}",
        "  func test_snake_case() throws {}",
        "  func testURLParsing() {}",
      ].joined(separator: "\n")
    ).body
    let expected = [
      "  @Test", "  func addsNumbers() {}", "  @Test", "  func loads() async throws {}",
      "  @Test", "  func snake_case() throws {}", "  @Test", "  func uRLParsing() {}",
    ]
    #expect(body == expected.joined(separator: "\n"))
  }

  @Test
  func putsTestOnItsOwnLineAtTheSameIndentation() throws {
    let result = try members("    @MainActor func testIt() {}")
    #expect(result.body == "    @Test\n    @MainActor func it() {}")
  }

  @Test
  func keepsPrefixOnCollision() throws {
    let body = try members(
      [
        "  func testFoo() {}", "  func foo() {}", "  func testBar() {}", "  var bar = 1",
        "  func testBaz() { baz(1) }", "  func testQux() {}", "  func test_qux() {}",
        "  func testFree() { _ = free }",
      ].joined(separator: "\n")
    ).body
    #expect(body.contains("func testFoo() {}"))
    #expect(body.contains("func testBar() {}"))
    #expect(body.contains("func testBaz() { baz(1) }"))
    #expect(body.contains("func testQux() {}"))
    #expect(body.contains("func test_qux() {}"))
    #expect(body.contains("func free() { _ = free }"))
  }

  @Test
  func keepsPrefixWhenTheNameWouldBeInvalid() throws {
    let names = ["test", "test1", "testDefault", "testInit", "test_"]
    let body = try members(names.map { "  func \($0)() {}" }.joined(separator: "\n")).body
    for name in names {
      #expect(body.contains("@Test\n  func \(name)() {}"))
    }
  }

  @Test
  func ignoresFunctionsXCTestWouldNotRun() throws {
    let body = try members(
      [
        "  func testWithArgument(_ x: Int) {}", "  private func testHelper() {}",
        "  static func testStatic() {}",
      ].joined(separator: "\n")
    ).body
    #expect(!body.contains("@Test"))
  }

  @Test
  func convertsTestsInExtensionsOfConvertedClasses() throws {
    let source = "import XCTest\nclass A: XCTestCase {}\nextension A {\n  func testB() {}\n}\n"
    let text = try converted(source).text
    #expect(text.contains("extension A {\n  @Test\n  func b() {}\n}"))
  }

  @Test
  func leavesFunctionsOfOtherClassesAlone() throws {
    let source = "import XCTest\nclass A {\n  func testB() {}\n}\n"
    #expect(try converted(source).text == "import Testing\nclass A {\n  func testB() {}\n}\n")
  }

  @Test(
    arguments: [
      ("override func setUp()", "init()"),
      ("override func setUp() async", "init() async"),
      ("override func setUpWithError() throws", "init() throws"),
      ("override func setUp() async throws", "init() async throws"),
      ("override func tearDown()", "deinit"),
      ("override func tearDownWithError()", "deinit"),
    ]
  )
  func convertsLifecycle(input: String, expected: String) throws {
    let call = input.contains("tear") ? "super.tearDown()" : "super.setUp()"
    let result = try members("  \(input) {\n    \(call)\n    work()\n  }")
    #expect(result.body == "  \(expected) {\n    work()\n  }")
    #expect(result.notes.isEmpty)
  }

  @Test
  func removesEverySuperCallForm() throws {
    let lines = [
      "  override func setUp() async throws {", "    try await super.setUp()",
      "    try super.setUpWithError()", "    await super.setUp()", "    super.setUp();",
      "    keep()", "  }",
    ]
    let body = try members(lines.joined(separator: "\n")).body
    #expect(body == "  init() async throws {\n    keep()\n  }")
  }

  @Test
  func leavesTearDownThatCannotBecomeDeinit() throws {
    let lines = [
      "  override func tearDownWithError() throws {", "    try super.tearDownWithError()", "  }",
      "  override func tearDown() async {", "    await cleanup()", "  }",
    ]
    let result = try members(lines.joined(separator: "\n"))
    #expect(result.body.contains("override func tearDownWithError() throws {"))
    #expect(result.body.contains("override func tearDown() async {"))
    #expect(result.notes.count == 2)
    #expect(result.notes.allSatisfy { $0.contains("deinit cannot throw or be async") })
    #expect(result.notes.first?.hasPrefix("line 3: ") == true)
  }

  @Test
  func convertsOnlyOneInitAndDeinit() throws {
    let lines = [
      "  override func setUp() {}", "  override func setUpWithError() throws {}",
      "  override func tearDown() {}", "  override func tearDownWithError() {}",
    ]
    let result = try members(lines.joined(separator: "\n"))
    #expect(result.body.contains("  init() {}\n  override func setUpWithError() throws {}"))
    #expect(result.body.contains("  deinit {}\n  override func tearDownWithError() {}"))
    #expect(result.notes.count == 2)
  }
}

@Suite
struct Leftovers {
  @Test(
    arguments: [
      ("try XCTSkipIf(flag)", "XCTSkipIf has no direct Swift Testing form"),
      ("try XCTSkipUnless(flag)", "XCTSkipUnless has no direct Swift Testing form"),
      ("throw XCTSkip(\"no\")", "XCTSkip has no direct Swift Testing form"),
      (
        "let e = expectation(description: \"x\")",
        "expectation(description:) has no direct Swift Testing form; use confirmation(...)"
      ),
      ("wait(for: [e], timeout: 1)", "wait(for:) has no direct Swift Testing form"),
      ("e.fulfill()", "fulfill() has no direct Swift Testing form"),
      ("let e = XCTestExpectation(description: \"x\")", "XCTestExpectation has no direct"),
      ("measure { work() }", "measure has no direct Swift Testing form"),
      ("self.measure(metrics: m) { work() }", "measure has no direct Swift Testing form"),
      ("let app = XCUIApplication()", "XCUIApplication has no direct Swift Testing form"),
      ("addTeardownBlock { cleanup() }", "addTeardownBlock has no direct Swift Testing form"),
      ("continueAfterFailure = false", "continueAfterFailure has no direct Swift Testing form"),
      ("XCTAssertThrowsError(try f()) { _ in }", "XCTAssertThrowsError with an error handler"),
    ]
  )
  func notesAndKeepsXCTest(input: String, note: String) throws {
    let source =
      "import XCTest\nfinal class T: XCTestCase {\n  func testA() {\n    \(input)\n  }\n}\n"
    let result = try converted(source)
    #expect(result.text.hasPrefix("import Testing\nimport XCTest\nfinal class T {\n"))
    #expect(result.text.contains("@Test\n  func a() {\n    \(input)\n"))
    #expect(result.notes.contains { $0.hasPrefix("line 6: ") && $0.contains(note) })
    #expect(result.notes.contains("line 2: import XCTest kept for the constructs noted here"))
  }

  @Test
  func doesNotFlagUnrelatedMethods() throws {
    let rest = "let t = ruler.measure(x) + clock.wait(1)\nobj.expectation(1)\n"
    let result = try converted("import XCTest\n\(rest)")
    #expect(result.text == "import Testing\n\(rest)")
    #expect(result.notes.isEmpty)
  }

  @Test
  func dropsXCTestImportWhenNothingRemains() throws {
    let result = try converted("import XCTest\nXCTAssertTrue(a)\n")
    #expect(result.notes.isEmpty)
    #expect(!result.text.contains("import XCTest"))
  }

  @Test
  func reusesAnExistingTestingImport() throws {
    let result = try converted("import Testing\nimport XCTest\nXCTAssertTrue(a)\n")
    #expect(result.text == "import Testing\n#expect(a)\n")
  }
}

@Suite
struct Whole {
  @Test
  func returnsNilWithoutXCTestImport() {
    #expect(XCTestConverter.convert("import Testing\n@Test func a() {}\n") == nil)
    #expect(XCTestConverter.convert("let s = \"import XCTest\"\n") == nil)
    #expect(XCTestConverter.convert("// import XCTest\n") == nil)
    #expect(XCTestConverter.convert("") == nil)
  }

  @Test
  func isIdempotent() throws {
    let source = try packageFile("Tests/Fixtures/LegacyTests.xctest.swift.txt")
    let once = try converted(source)
    #expect(XCTestConverter.convert(once.text) == nil)
  }

  @Test
  func isIdempotentWhenXCTestIsKept() throws {
    let source = "import XCTest\nclass T: XCTestCase {\n  func testA() {\n    measure {}\n  }\n}\n"
    let once = try converted(source)
    let twice = try converted(once.text)
    #expect(twice.text == once.text)
    #expect(twice.notes == once.notes)
  }

  @Test
  func keepsWindowsLineEndings() throws {
    let source = "import XCTest\r\nclass T: XCTestCase {\r\n  func testA() {}\r\n}\r\n"
    let expected = "import Testing\r\nfinal class T {\r\n  @Test\r\n  func a() {}\r\n}\r\n"
    #expect(try converted(source).text == expected)
  }

  @Test
  func convertsTheLegacyFixtureToTheCompiledFile() throws {
    let source = try packageFile("Tests/Fixtures/LegacyTests.xctest.swift.txt")
    let committed = try packageFile("Tests/Fixtures/ConvertedLegacyTests.swift.txt")
    let result = try converted(source)
    #expect(result.text == committed)
    #expect(result.notes.isEmpty)
  }
}
