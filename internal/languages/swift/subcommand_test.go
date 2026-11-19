package swift_test

import (
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func TestConvertTestsConvertsEveryUnitTestFileWithoutPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"Tests/A.swift":      xctestSource,
		"AppUITests/U.swift": xctestSource,
	})

	// Act
	got := runSubcommand(t, root)
	status, stdout := got.status, got.stdout

	// Assert
	if status != 0 || !strings.Contains(stdout, "`Tests/A.swift`: converted") {
		t.Errorf("status = %d, stdout = %q", status, stdout)
	}

	if !strings.Contains(bedrocktest.Read(t, root, "Tests/A.swift"), "import Testing") {
		t.Error("the unit test is not converted")
	}

	if bedrocktest.Read(t, root, "AppUITests/U.swift") != xctestSource {
		t.Error("the UI test is rewritten")
	}
}

func TestConvertTestsLeavesAFileWithoutXCTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"Sources/B.swift": "let b = 1\n"})

	// Act
	got := runSubcommand(t, root, "Sources/B.swift")
	status, stdout := got.status, got.stdout

	// Assert
	if status != 0 || !strings.Contains(stdout, "`Sources/B.swift`: no XCTest import; left as is") {
		t.Errorf("status = %d, stdout = %q", status, stdout)
	}
}

func TestConvertTestsFailsOnAMissingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	got := runSubcommand(t, root, "Tests/Nope.swift")
	status, stderr := got.status, got.stderr

	// Assert
	if status != 1 || !strings.Contains(stderr, "`Tests/Nope.swift`: no such file") {
		t.Errorf("status = %d, stderr = %q", status, stderr)
	}
}

func TestConvertTestsPrintsWhatConversionLeavesForAPerson(t *testing.T) {
	t.Parallel()

	// Arrange
	source := "import XCTest\nfinal class A: XCTestCase {\n  func testB() {\n    expectation(description: \"x\")\n  }\n}\n"
	root := bedrocktest.Repo(t, map[string]string{"Tests/A.swift": source})

	// Act
	got := runSubcommand(t, root, "Tests/A.swift")
	status, stderr := got.status, got.stderr

	// Assert
	if status != 0 || !strings.Contains(stderr, "`Tests/A.swift`: line ") {
		t.Errorf("status = %d, stderr = %q", status, stderr)
	}
}
