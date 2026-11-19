package typescript_test

import (
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/language"
)

func guardFindings(t *testing.T, files map[string]string) []language.Finding {
	t.Helper()

	root := bedrocktest.Repo(t, files)
	chosen := bedrocktest.Select(t, "typescript", nil)

	guarder, ok := bedrocktest.Language(t, "typescript").(language.Guarder)
	if !ok {
		t.Fatal("the language has no guard")
	}

	findings, err := guarder.Guard(t.Context(), bedrocktest.Context(t, root, chosen), false)
	if err != nil {
		t.Fatal(err)
	}

	return findings
}

func TestGuardReportsSuppressionCommentsWithTheirLine(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		packageJSON:   "{}\n",
		"src/a.ts":    "export const a = 1;\n// @ts-" + "ignore\nexport const b: number = 'x';\n",
		"src/ok.ts":   "export const ok = 1;\n",
		"vendor/x.ts": "// @ts-" + "ignore\n",
		oxlintrc:      `{"ignorePatterns":["vendor/**"]}`,
	}

	// Act
	findings := guardFindings(t, files)

	// Assert
	if !slices.Contains(findings, "src/a.ts:2: suppression comment") {
		t.Errorf("findings: %q", findings)
	}

	if slices.Contains(findings, "vendor/x.ts:1: suppression comment") {
		t.Errorf("an ignored file was checked: %q", findings)
	}
}

func TestGuardReportsFilesAndScriptsOfOtherPackageManagers(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		packageJSON:         `{"scripts":{"build":"npm run x","ok":"bun run x"}}`,
		"yarn.lock":         "",
		"package-lock.json": "{}",
	}

	// Act
	findings := guardFindings(t, files)

	// Assert
	for _, want := range []string{
		"yarn.lock: not used in a Bun-only repo",
		"package-lock.json: not used in a Bun-only repo",
		"`package.json`: script \"build\" calls another package manager",
	} {
		if !slices.Contains(findings, want) {
			t.Errorf("misses %q in %q", want, findings)
		}
	}

	if slices.Contains(findings, "`package.json`: script \"ok\" calls another package manager") {
		t.Errorf("a Bun script was reported: %q", findings)
	}
}

func TestGuardReportsAnUnsortedCatalog(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{packageJSON: `{"workspaces":{"catalog":{"zod":"1","arktype":"2"}}}`}

	// Act
	findings := guardFindings(t, files)

	// Assert
	if !slices.Contains(findings, "`package.json`: sort `workspaces.catalog` by package name") {
		t.Errorf("findings: %q", findings)
	}
}

func TestGuardPassesACleanRepo(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		packageJSON: `{"workspaces":{"catalog":{"arktype":"2","zod":"1"}}}`,
		"src/a.ts":  "export const a = 1;\n",
	}

	// Act
	findings := guardFindings(t, files)

	// Assert
	if len(findings) != 0 {
		t.Errorf("findings: %q", findings)
	}
}
