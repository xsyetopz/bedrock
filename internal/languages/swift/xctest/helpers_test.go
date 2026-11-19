package xctest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/languages/swift/xctest"
)

func fixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	return string(data)
}

func converted(t *testing.T, source string) xctest.Converted {
	t.Helper()

	result, ok := xctest.Convert(source)
	if !ok {
		t.Fatalf("expected the source to import XCTest: %q", source)
	}

	return result
}

// statement converts one statement and returns what replaced it.
func statement(t *testing.T, code string) string {
	t.Helper()

	text := converted(t, "import XCTest\n"+code+"\n").Text

	return strings.TrimSuffix(strings.TrimPrefix(text, "import Testing\n"), "\n")
}

// members converts members of an XCTestCase and returns the class body.
func members(t *testing.T, code string) (string, []string) {
	t.Helper()

	result := converted(t, "import XCTest\nfinal class T: XCTestCase {\n"+code+"\n}\n")

	const prefix = "import Testing\nfinal class T {\n"
	if !strings.HasPrefix(result.Text, prefix) {
		t.Fatalf("unexpected class header: %q", result.Text)
	}

	return strings.TrimSuffix(strings.TrimPrefix(result.Text, prefix), "\n}\n"), result.Notes
}
