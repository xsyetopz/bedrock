package typescript_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/plan"
)

const (
	packageJSON = "package.json"
	oxlintrc    = ".oxlintrc.json"
	markdownrc  = ".markdownlint-cli2.jsonc"
	typesTest   = "src/a.test.ts"
)

func legacyRepo(t *testing.T) string {
	t.Helper()

	return bedrocktest.Repo(t, map[string]string{
		packageJSON: `{"name":"old","scripts":{"guard":"typescript-bedrock guard","lint":"oxlint","test":"jest"},` +
			`"devDependencies":{"typescript-bedrock":"catalog:","eslint":"^9.0.0"},` +
			`"workspaces":{"catalog":{"typescript-bedrock":"1.2.3"}}}`,
		"tsconfig.json": `{"extends":"typescript-bedrock/tsconfig.base.json","compilerOptions":{"rootDir":"."}}`,
		oxlintrc: `{"extends":["./node_modules/typescript-bedrock/oxlintrc.json"],` +
			`"ignorePatterns":["vendor/**"],"rules":{"no-console":"off"}}`,
		markdownrc: `{"ignores":["notes/**"],` +
			`"config":{"extends":"./node_modules/typescript-bedrock/markdownlint.jsonc"}}`,
		"eslint.config.js":         "export default [];\n",
		"yarn.lock":                "",
		".github/workflows/ci.yml": "steps:\n  - run: npm test\n",
		"project.just":             "guard:\n    bunx typescript-bedrock guard\n",
		typesTest:                  "test(\"a\", () => {});\n",
	})
}

func selected(t *testing.T) plan.Selected {
	t.Helper()
	return bedrocktest.Select(t, "typescript", nil)
}

func TestAdoptDropsTheLegacyPackageItsExtendsAndTheToolsItReplaced(t *testing.T) {
	t.Parallel()

	// Arrange
	root := legacyRepo(t)

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	for _, path := range []string{packageJSON, "tsconfig.json", oxlintrc, markdownrc} {
		if text := bedrocktest.Read(t, root, path); strings.Contains(text, "typescript-bedrock") {
			t.Errorf("%s still names the legacy package:\n%s", path, text)
		}
	}

	manifest := bedrocktest.Read(t, root, packageJSON)
	for _, want := range []string{`"guard": "just typescript::guard"`, `"lint": "just lint"`} {
		if !strings.Contains(manifest, want) {
			t.Errorf("package.json misses %s:\n%s", want, manifest)
		}
	}

	for _, gone := range []string{"eslint.config.js", ".github/workflows/ci.yml"} {
		if _, err := os.Stat(filepath.Join(root, gone)); err == nil {
			t.Errorf("%s was not removed", gone)
		}
	}
}

func TestAdoptKeepsTheReposOwnIgnores(t *testing.T) {
	t.Parallel()

	// Arrange
	root := legacyRepo(t)

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	if text := bedrocktest.Read(t, root, oxlintrc); !strings.Contains(text, `"vendor/**"`) {
		t.Errorf("%s:\n%s", oxlintrc, text)
	}

	if text := bedrocktest.Read(t, root, markdownrc); !strings.Contains(text, `"notes/**"`) {
		t.Errorf("%s:\n%s", markdownrc, text)
	}
}

func TestAdoptMovesTestFilesOntoBunTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := legacyRepo(t)

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	if text := bedrocktest.Read(t, root, typesTest); !strings.HasPrefix(text, `import { test } from "bun:test";`) {
		t.Errorf("test file:\n%s", text)
	}
}

func TestAdoptNotesTheLegacyMentionInProjectJust(t *testing.T) {
	t.Parallel()

	// Arrange
	root := legacyRepo(t)

	// Act
	result := bedrocktest.Plan(t, root, false, selected(t))

	// Assert
	want := "`project.just` still mentions `typescript-bedrock`, which bedrock no longer installs. " +
		"Run `just typescript::guard` and `just lint` instead."
	if !slices.Contains(result.Notes, want) {
		t.Errorf("notes: %q", result.Notes)
	}
}

func TestSecondAdoptOfALegacyRepoPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := legacyRepo(t)
	bedrocktest.Adopt(t, root, false, selected(t))

	// Act
	again := bedrocktest.Plan(t, root, false, selected(t))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestAdoptOfAFreshRepoWritesConfigsThatExtendNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	for _, path := range []string{"tsconfig.json", oxlintrc, markdownrc} {
		if text := bedrocktest.Read(t, root, path); strings.Contains(text, `"extends"`) {
			t.Errorf("%s extends another config:\n%s", path, text)
		}
	}
}

func TestAdoptLeavesPackageTsconfigAndBunfigToTheProject(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, false, selected(t))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	for _, want := range []string{packageJSON, "tsconfig.json", "bunfig.toml", oxlintrc} {
		if !slices.Contains(paths, want) {
			t.Errorf("plan misses %s: %v", want, paths)
		}
	}
}

func driftAfter(t *testing.T, edit func(string) string) []string {
	t.Helper()

	root := bedrocktest.Repo(t, map[string]string{packageJSON: `{"name":"x"}`})
	bedrocktest.Adopt(t, root, false, selected(t))
	bedrocktest.Write(t, root, oxlintrc, edit(bedrocktest.Read(t, root, oxlintrc)))

	return bedrocktest.Paths(bedrocktest.Plan(t, root, false, selected(t)).Steps)
}

func TestAChangedRuleInTheOxlintConfigIsDrift(t *testing.T) {
	t.Parallel()

	// Arrange
	edit := func(text string) string {
		return strings.Replace(text, `"correctness": "error"`, `"correctness": "off"`, 1)
	}

	// Act
	paths := driftAfter(t, edit)

	// Assert
	if !slices.Contains(paths, oxlintrc) {
		t.Errorf("no drift reported: %v", paths)
	}
}

func TestAnAddedIgnoreInTheOxlintConfigIsTheReposOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	edit := func(text string) string {
		return strings.Replace(text, `"openspec/**"`, "\"openspec/**\",\n\t\t\"vendor/**\"", 1)
	}

	// Act
	paths := driftAfter(t, edit)

	// Assert
	if slices.Contains(paths, oxlintrc) {
		t.Errorf("the added ignore reads as drift: %v", paths)
	}
}

func TestALintConfigThatDroppedABedrockIgnoreGetsItBack(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{packageJSON: "{}\n", oxlintrc: `{"ignorePatterns":["vendor/**"]}`})

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	if text := bedrocktest.Read(t, root, oxlintrc); !strings.Contains(text, `"openspec/**"`) {
		t.Errorf("%s:\n%s", oxlintrc, text)
	}
}

func TestMergedIgnoreListsHaveNoTrailingComma(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{packageJSON: "{}\n", oxlintrc: `{"ignorePatterns":["vendor/**"]}`})

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	text := bedrocktest.Read(t, root, oxlintrc)
	if strings.Contains(text, "\"vendor/**\",\n\t]") || strings.Contains(text, ",\n\t]") {
		t.Errorf("trailing comma in:\n%s", text)
	}

	if !strings.Contains(text, `"vendor/**"`) {
		t.Errorf("repo ignore lost:\n%s", text)
	}
}

func TestInitWritesTheKindsEntryPointIntoThePackage(t *testing.T) {
	t.Parallel()

	for kind, want := range map[string]string{
		"library": `"exports": {`,
		"cli":     `"bin": {`,
	} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, nil)

			// Act
			result := bedrocktest.Plan(t, root, true,
				bedrocktest.Select(t, "typescript", map[string]string{"kind": kind}))

			// Assert
			if text := bedrocktest.Written(result.Steps, packageJSON); !strings.Contains(text, want) {
				t.Errorf("package.json misses %s:\n%s", want, text)
			}
		})
	}
}
