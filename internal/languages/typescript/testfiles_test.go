package typescript_test

import (
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

// Built at runtime so adopt does not migrate this file.
func frameworkName() string { return strings.Join([]string{"vi", "test"}, "") }

const bareTest = "test(\"x\", () => {});\n"

func adoptTestFile(t *testing.T, files map[string]string) string {
	t.Helper()

	files[packageJSON] = "{}\n"
	root := bedrocktest.Repo(t, files)
	bedrocktest.Adopt(t, root, false, selected(t))

	return bedrocktest.Read(t, root, typesTest)
}

func TestAdoptMovesAFrameworkImportToBunTest(t *testing.T) {
	t.Parallel()

	// Arrange
	source := `import { describe, it, vi } from "` + frameworkName() + "\";\n"

	// Act
	got := adoptTestFile(t, map[string]string{typesTest: source})

	// Assert
	if want := "import { describe, it, vi } from \"bun:test\";\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdoptImportsTheGlobalsATestFileCalls(t *testing.T) {
	t.Parallel()

	// Arrange
	source := "describe.each([1])('x', () => {\n\tit('y', () => expect(/a/.test('a')).toBe(true));\n});\n"

	// Act
	got := adoptTestFile(t, map[string]string{typesTest: source})

	// Assert
	if want := "import { describe, expect, it } from \"bun:test\";\n\n"; !strings.HasPrefix(got, want) {
		t.Errorf("got %q", got)
	}
}

func TestAdoptIgnoresTheFrameworkNameOutsideAnImport(t *testing.T) {
	t.Parallel()

	// Arrange
	source := "const name = \"" + frameworkName() + "\";\n" + bareTest

	// Act
	got := adoptTestFile(t, map[string]string{typesTest: source})

	// Assert
	if want := "import { test } from \"bun:test\";\n\n"; !strings.HasPrefix(got, want) {
		t.Errorf("got %q", got)
	}
}

func TestAdoptLeavesBoundAndDeclaredGlobalsAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	bound := "import { expect as it } from \"./runtime.ts\";\nimport test from \"./t.ts\";\nit(\"x\", () => {});\ntest(\"y\", () => {});\n"
	local := "const expect = () => 1;\nexpect(2);\n"

	// Act
	files := map[string]string{typesTest: bound, "src/b.test.ts": local}
	gotBound := adoptTestFile(t, files)

	// Assert
	if gotBound != bound {
		t.Errorf("bound globals rewritten: %q", gotBound)
	}
}

func TestAdoptLeavesABunTestFileAndANonTestFileAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	bun := "import { test } from \"bun:test\";\n" + bareTest
	plain := "export const submit = () => it;\n"

	// Act
	root := bedrocktest.Repo(t, map[string]string{packageJSON: "{}\n", typesTest: bun, "src/plain.test.ts": plain})
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	if got := bedrocktest.Read(t, root, typesTest); got != bun {
		t.Errorf("bun:test file rewritten: %q", got)
	}

	if got := bedrocktest.Read(t, root, "src/plain.test.ts"); got != plain {
		t.Errorf("file without tests rewritten: %q", got)
	}
}

func TestAdoptSkipsTestFilesGitIgnoresAndBunfigSkips(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		packageJSON:                  "{}\n",
		".gitignore":                 "captures/\n",
		bunfig:                       "[test]\npathIgnorePatterns = [\"plugins/**\"]\n",
		typesTest:                    bareTest,
		"captures/dep/c.test.ts":     bareTest,
		"plugins/lab/c.test.ts":      bareTest,
		"node_modules/dep/c.test.ts": bareTest,
	})

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	if got := bedrocktest.Read(t, root, typesTest); !strings.HasPrefix(got, "import { test }") {
		t.Errorf("source test not migrated: %q", got)
	}

	for _, path := range []string{"captures/dep/c.test.ts", "plugins/lab/c.test.ts", "node_modules/dep/c.test.ts"} {
		if got := bedrocktest.Read(t, root, path); got != bareTest {
			t.Errorf("%s was rewritten: %q", path, got)
		}
	}
}
