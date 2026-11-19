package xctest_test

import (
	"slices"
	"strings"
	"testing"
)

func TestStructureKeepsTestableImports(t *testing.T) {
	t.Parallel()

	got := converted(t, "import XCTest\n@testable import Lib\n").Text

	if want := "import Testing\n@testable import Lib\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStructureClassDeclarations(t *testing.T) {
	t.Parallel()

	cases := [][2]string{
		{"class FooTests: XCTestCase {", "final class FooTests {"},
		{"final class FooTests: XCTestCase {", "final class FooTests {"},
		{"@MainActor final class FooTests: XCTestCase {", "@MainActor final class FooTests {"},
		{"@MainActor\nclass FooTests: XCTestCase {", "@MainActor\nfinal class FooTests {"},
		{"final class FooTests: XCTestCase, Sendable {", "final class FooTests: Sendable {"},
		{"class FooTests: Base, XCTestCase, P {", "final class FooTests: Base, P {"},
		{"public class FooTests : XCTestCase{", "public final class FooTests {"},
		{"class Helper: NSObject {", "class Helper: NSObject {"},
	}

	for _, tc := range cases {
		t.Run(tc[0], func(t *testing.T) {
			t.Parallel()

			got := converted(t, "import XCTest\n"+tc[0]+"\n}\n").Text

			if want := "import Testing\n" + tc[1] + "\n}\n"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestStructureConvertsTests(t *testing.T) {
	t.Parallel()

	// Arrange
	code := strings.Join([]string{
		"  func testAddsNumbers() {}",
		"  func testLoads() async throws {}",
		"  func test_snake_case() throws {}",
		"  func testURLParsing() {}",
	}, "\n")

	// Act
	body, _ := members(t, code)

	// Assert
	want := strings.Join([]string{
		"  @Test",
		"  func addsNumbers() {}",
		"  @Test",
		"  func loads() async throws {}",
		"  @Test",
		"  func snake_case() throws {}",
		"  @Test",
		"  func uRLParsing() {}",
	}, "\n")
	if body != want {
		t.Errorf("got %q, want %q", body, want)
	}
}

func TestStructurePutsTestOnItsOwnLine(t *testing.T) {
	t.Parallel()

	body, _ := members(t, "    @MainActor func testIt() {}")

	if want := "    @Test\n    @MainActor func it() {}"; body != want {
		t.Errorf("got %q, want %q", body, want)
	}
}

func TestStructureKeepsThePrefixOnACollision(t *testing.T) {
	t.Parallel()

	// Arrange
	code := strings.Join([]string{
		"  func testFoo() {}",
		"  func foo() {}",
		"  func testBar() {}",
		"  var bar = 1",
		"  func testBaz() { baz(1) }",
		"  func testQux() {}",
		"  func test_qux() {}",
		"  func testFree() { _ = free }",
	}, "\n")

	// Act
	body, _ := members(t, code)

	// Assert
	for _, kept := range []string{
		"func testFoo() {}",
		"func testBar() {}",
		"func testBaz() { baz(1) }",
		"func testQux() {}",
		"func test_qux() {}",
		"func free() { _ = free }",
	} {
		if !strings.Contains(body, kept) {
			t.Errorf("missing %q in %q", kept, body)
		}
	}
}

func TestStructureKeepsThePrefixWhenTheNameWouldBeInvalid(t *testing.T) {
	t.Parallel()

	names := []string{"test", "test1", "testDefault", "testInit", "test_"}

	var lines []string
	for _, name := range names {
		lines = append(lines, "  func "+name+"() {}")
	}

	body, _ := members(t, strings.Join(lines, "\n"))

	for _, name := range names {
		if want := "@Test\n  func " + name + "() {}"; !strings.Contains(body, want) {
			t.Errorf("missing %q in %q", want, body)
		}
	}
}

func TestStructureIgnoresFunctionsXCTestWouldNotRun(t *testing.T) {
	t.Parallel()

	body, _ := members(t, strings.Join([]string{
		"  func testWithArgument(_ x: Int) {}",
		"  private func testHelper() {}",
		"  static func testStatic() {}",
	}, "\n"))

	if strings.Contains(body, "@Test") {
		t.Errorf("unexpected @Test in %q", body)
	}
}

func TestStructureConvertsExtensionsOfConvertedClasses(t *testing.T) {
	t.Parallel()

	source := "import XCTest\nclass A: XCTestCase {}\nextension A {\n  func testB() {}\n}\n"

	got := converted(t, source).Text

	if want := "extension A {\n  @Test\n  func b() {}\n}"; !strings.Contains(got, want) {
		t.Errorf("missing %q in %q", want, got)
	}
}

func TestStructureLeavesOtherClassesAlone(t *testing.T) {
	t.Parallel()

	got := converted(t, "import XCTest\nclass A {\n  func testB() {}\n}\n").Text

	if want := "import Testing\nclass A {\n  func testB() {}\n}\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLifecycleConversions(t *testing.T) {
	t.Parallel()

	cases := [][3]string{
		{"override func setUp()", "init()", "super.setUp()"},
		{"override func setUp() async", "init() async", "super.setUp()"},
		{"override func setUpWithError() throws", "init() throws", "super.setUpWithError()"},
		{"override func setUp() async throws", "init() async throws", "super.setUp()"},
		{"override func tearDown()", "deinit", "super.tearDown()"},
		{"override func tearDownWithError()", "deinit", "super.tearDownWithError()"},
	}

	for _, tc := range cases {
		t.Run(tc[0], func(t *testing.T) {
			t.Parallel()

			// Arrange
			code := "  " + tc[0] + " {\n    " + tc[2] + "\n    work()\n  }"

			// Act
			body, notes := members(t, code)

			// Assert
			if want := "  " + tc[1] + " {\n    work()\n  }"; body != want {
				t.Errorf("got %q, want %q", body, want)
			}

			if len(notes) != 0 {
				t.Errorf("unexpected notes %v", notes)
			}
		})
	}
}

func TestLifecycleRemovesEverySuperCallForm(t *testing.T) {
	t.Parallel()

	body, _ := members(t, strings.Join([]string{
		"  override func setUp() async throws {",
		"    try await super.setUp()",
		"    try super.setUpWithError()",
		"    await super.setUp()",
		"    super.setUp();",
		"    keep()",
		"  }",
	}, "\n"))

	if want := "  init() async throws {\n    keep()\n  }"; body != want {
		t.Errorf("got %q, want %q", body, want)
	}
}

func TestLifecycleLeavesATearDownThatCannotBecomeDeinit(t *testing.T) {
	t.Parallel()

	body, notes := members(t, strings.Join([]string{
		"  override func tearDownWithError() throws {",
		"    try super.tearDownWithError()",
		"  }",
		"  override func tearDown() async {",
		"    await cleanup()",
		"  }",
	}, "\n"))

	for _, want := range []string{
		"override func tearDownWithError() throws {",
		"override func tearDown() async {",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in %q", want, body)
		}
	}

	if len(notes) != 2 || !strings.HasPrefix(notes[0], "line 3: ") {
		t.Errorf("unexpected notes %v", notes)
	}
}

func TestLifecycleConvertsOnlyOneInitAndOneDeinit(t *testing.T) {
	t.Parallel()

	body, notes := members(t, strings.Join([]string{
		"  override func setUp() {}",
		"  override func setUpWithError() throws {}",
		"  override func tearDown() {}",
		"  override func tearDownWithError() {}",
	}, "\n"))

	for _, want := range []string{
		"  init() {}\n  override func setUpWithError() throws {}",
		"  deinit {}\n  override func tearDownWithError() {}",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in %q", want, body)
		}
	}

	if len(notes) != 2 {
		t.Errorf("want 2 notes, got %v", notes)
	}
}

func TestConvertKeepsWindowsLineEndings(t *testing.T) {
	t.Parallel()

	source := "import XCTest\r\nclass T: XCTestCase {\r\n  func testA() {}\r\n}\r\n"

	got := converted(t, source).Text

	if want := "import Testing\r\nfinal class T {\r\n  @Test\r\n  func a() {}\r\n}\r\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertsTheLegacyFixtureToTheCommittedFile(t *testing.T) {
	t.Parallel()

	result := converted(t, fixture(t, "legacy-tests.txt"))

	if want := fixture(t, "converted-legacy-tests.txt"); result.Text != want {
		t.Errorf("converted text differs from the committed file:\n%s", result.Text)
	}

	if len(result.Notes) != 0 {
		t.Errorf("unexpected notes %v", result.Notes)
	}
}

func TestConvertIsIdempotent(t *testing.T) {
	t.Parallel()

	once := converted(t, fixture(t, "legacy-tests.txt"))

	if _, ok := xctestConvert(once.Text); ok {
		t.Error("a converted file converted again")
	}
}

func TestConvertIsIdempotentWhenXCTestIsKept(t *testing.T) {
	t.Parallel()

	source := "import XCTest\nclass T: XCTestCase {\n  func testA() {\n    measure {}\n  }\n}\n"
	once := converted(t, source)

	twice := converted(t, once.Text)

	if twice.Text != once.Text || !slices.Equal(twice.Notes, once.Notes) {
		t.Errorf("second pass changed the result:\n%q\n%v", twice.Text, twice.Notes)
	}
}
