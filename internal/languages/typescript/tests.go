package typescript

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

const testFiles = "**/*.{spec,test}.{cjs,cts,js,jsx,mjs,mts,ts,tsx}"

// frameworkImport finds a module specifier that names Jest's or Vitest's
// package; RE2 has no backreference, so each quote style has its own branch.
func frameworkImport() *regexp.Regexp {
	return regexp.MustCompile(
		`(\bfrom\s*|\b(?:import|require)\s*\(\s*)(?:"(?:@jest/globals|vitest)"|'(?:@jest/globals|vitest)')`,
	)
}

func testGlobals() []string {
	return []string{
		"afterAll",
		"afterEach",
		"beforeAll",
		"beforeEach",
		"describe",
		"expect",
		"it",
		nameJest,
		nameTest,
		"vi",
	}
}

// usesGlobal reports a call or member access that is not itself a member,
// so `regex.test(…)` is no use.
func usesGlobal(text, name string) bool {
	return regexp.MustCompile(`(?:^|[^\w$.])` + name + `\s*[(.]`).MatchString(text)
}

func importClause() *regexp.Regexp {
	return regexp.MustCompile(`\bimport\s+(?:type\s+)?([^"';]*?)\s*from\s*["']`)
}

// binds reports whether the file already binds name, by an import or a
// declaration.
func binds(text, name string) bool {
	declared := regexp.MustCompile(`\b(?:const|let|var|function|class)\s+` + name + `\b`)
	if declared.MatchString(text) {
		return true
	}

	for _, match := range importClause().FindAllStringSubmatch(text, -1) {
		clause := strings.NewReplacer("{", ",", "}", ",").Replace(match[1])

		for part := range strings.SplitSeq(clause, ",") {
			names := regexp.MustCompile(`\s+as\s+`).Split(strings.TrimSpace(part), -1)
			if strings.TrimSpace(names[len(names)-1]) == name {
				return true
			}
		}
	}

	return false
}

// migrateTestFile points a Jest or Vitest test file at bun:test, which has
// the same API: its framework import changes module, and the globals it
// calls get imported, since the types declare them only in bun:test.
func migrateTestFile(text string) string {
	framework := frameworkImport()
	imported := framework.ReplaceAllStringFunc(text, func(match string) string {
		head := framework.FindStringSubmatch(match)[1]
		quoteMark := match[len(match)-1:]

		return head + quoteMark + "bun:test" + quoteMark
	})

	if regexp.MustCompile(`["']bun:test["']`).MatchString(imported) {
		return imported
	}

	var used []string

	for _, name := range testGlobals() {
		if usesGlobal(text, name) && !binds(imported, name) {
			used = append(used, name)
		}
	}

	if len(used) == 0 {
		return text
	}

	return "import { " + strings.Join(used, ", ") + " } from \"bun:test\";\n\n" + text
}

// testSteps rewrites each test file that is not on bun:test yet, except
// those `bun test` never runs: in node_modules or a hidden directory, ignored
// by git, or skipped by `bunfig.toml`.
func testSteps(ctx context.Context, c *language.Context) ([]step.Step, error) {
	ignore, err := readBunfigTestIgnores(c.Root)
	if err != nil {
		return nil, err
	}

	paths, err := listSources(ctx, c.Runner, c.Root, testFiles, ignore)
	if err != nil {
		return nil, err
	}

	var steps []step.Step

	for _, path := range paths {
		if inHiddenDirectory(path) {
			continue
		}

		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(path)))
		if readErr != nil {
			return nil, readErr
		}

		if content := migrateTestFile(text); content != text {
			steps = append(steps, step.Write{Path: path, Content: []byte(content), Executable: false})
		}
	}

	return steps, nil
}
