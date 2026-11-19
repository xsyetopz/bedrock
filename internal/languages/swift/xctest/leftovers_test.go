package xctest_test

import (
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/languages/swift/xctest"
)

func xctestConvert(source string) (xctest.Converted, bool) { return xctest.Convert(source) }

func hasNote(notes []string, prefix string) bool {
	for _, note := range notes {
		if strings.HasPrefix(note, prefix) {
			return true
		}
	}

	return false
}

func TestLeftoversAreNotedAndKeepXCTest(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"try XCTSkipIf(flag)",
		"try XCTSkipUnless(flag)",
		`throw XCTSkip("no")`,
		`let e = expectation(description: "x")`,
		"wait(for: [e], timeout: 1)",
		"e.fulfill()",
		`let e = XCTestExpectation(description: "x")`,
		"measure { work() }",
		"self.measure(metrics: m) { work() }",
		"let app = XCUIApplication()",
		"addTeardownBlock { cleanup() }",
		"continueAfterFailure = false",
		"XCTAssertThrowsError(try f()) { _ in }",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			// Arrange
			source := "import XCTest\nfinal class T: XCTestCase {\n  func testA() {\n    " + input + "\n  }\n}\n"

			// Act
			result := converted(t, source)

			// Assert
			if !strings.HasPrefix(result.Text, "import Testing\nimport XCTest\nfinal class T {\n") {
				t.Errorf("imports or header wrong: %q", result.Text)
			}

			if want := "@Test\n  func a() {\n    " + input + "\n"; !strings.Contains(result.Text, want) {
				t.Errorf("missing %q in %q", want, result.Text)
			}

			if !hasNote(result.Notes, "line 6: ") || !hasNote(result.Notes, "line 2: ") {
				t.Errorf("notes lack the construct or the kept import: %v", result.Notes)
			}
		})
	}
}

func TestLeftoversIgnoreUnrelatedMethods(t *testing.T) {
	t.Parallel()

	rest := "let t = ruler.measure(x) + clock.wait(1)\nobj.expectation(1)\n"

	result := converted(t, "import XCTest\n"+rest)

	if result.Text != "import Testing\n"+rest || len(result.Notes) != 0 {
		t.Errorf("unexpected result %q %v", result.Text, result.Notes)
	}
}

func TestConvertDropsXCTestWhenNothingRemains(t *testing.T) {
	t.Parallel()

	result := converted(t, "import XCTest\nXCTAssertTrue(a)\n")

	if len(result.Notes) != 0 || strings.Contains(result.Text, "import XCTest") {
		t.Errorf("unexpected result %q %v", result.Text, result.Notes)
	}
}

func TestConvertReusesAnExistingTestingImport(t *testing.T) {
	t.Parallel()

	got := converted(t, "import Testing\nimport XCTest\nXCTAssertTrue(a)\n").Text

	if want := "import Testing\n#expect(a)\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConvertReturnsNothingWithoutAnXCTestImport(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		"import Testing\n@Test func a() {}\n",
		"let s = \"import XCTest\"\n",
		"// import XCTest\n",
		"",
	} {
		if _, ok := xctest.Convert(source); ok {
			t.Errorf("converted %q", source)
		}

		if xctest.ImportsXCTest(source) {
			t.Errorf("ImportsXCTest(%q) = true", source)
		}
	}
}
