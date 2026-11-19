package typescript_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const bunfig = "bunfig.toml"

func parsedBunfig(t *testing.T, text string) map[string]any {
	t.Helper()

	var parsed map[string]any
	if err := toml.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("not TOML: %v\n%s", err, text)
	}

	return parsed
}

func adoptBunfig(t *testing.T, current string) (string, []string, string) {
	t.Helper()

	root := bedrocktest.Repo(t, map[string]string{packageJSON: "{}\n", bunfig: current})
	result := bedrocktest.Adopt(t, root, false, selected(t))

	return bedrocktest.Read(t, root, bunfig), result.Notes, root
}

func secondAdoptKeepsBunfig(t *testing.T, root, want string) {
	t.Helper()

	bedrocktest.Adopt(t, root, false, selected(t))

	if got := bedrocktest.Read(t, root, bunfig); got != want {
		t.Errorf("second adopt changed `bunfig.toml`:\n%s\nwant:\n%s", got, want)
	}
}

func TestBunfigKeepsKeysAndArrayEntriesAProjectAddsWithoutATrailingComma(t *testing.T) {
	t.Parallel()

	// Arrange
	current := "[test]\n# Cached packages carry their own tests.\nmyOption = true\n\n" +
		"[fmt]\nignorePatterns = [\n\t\"**/dist/\",\n\t# Built and committed.\n\t\"cli/bin/\",\n]\n"

	// Act
	merged, notes, root := adoptBunfig(t, current)

	// Assert
	fmtTable, _ := parsedBunfig(t, merged)["fmt"].(map[string]any)
	if patterns, _ := fmtTable["ignorePatterns"].([]any); !slices.Contains(patterns, any("cli/bin/")) {
		t.Errorf("project entry lost: %v", patterns)
	}

	for _, want := range []string{"# Cached packages carry their own tests.\nmyOption = true", "# Built and committed.\n\t\"cli/bin/\"\n]"} {
		if !strings.Contains(merged, want) {
			t.Errorf("merged misses %q:\n%s", want, merged)
		}
	}

	if len(notes) != 0 {
		t.Errorf("notes: %q", notes)
	}

	secondAdoptKeepsBunfig(t, root, merged)
}

func TestBunfigAddsAProjectEntryToAOneLineTemplateArray(t *testing.T) {
	t.Parallel()

	// Arrange
	current := "[test]\ncoverageReporter = [\"text\", \"junit\"]\n"

	// Act
	merged, _, root := adoptBunfig(t, current)

	// Assert
	if want := "coverageReporter = [\"text\", \"lcov\", \"junit\"]\n"; !strings.Contains(merged, want) {
		t.Errorf("merged misses %q:\n%s", want, merged)
	}

	secondAdoptKeepsBunfig(t, root, merged)
}

func TestBunfigKeepsAnEntryOnTheLineThatOpensAProjectArray(t *testing.T) {
	t.Parallel()

	// Arrange
	current := "[fmt]\nignorePatterns = [\"out/\",\n\t\"**/dist/\",\n\t\"gen/\", # generated\n]\n"

	// Act
	merged, _, _ := adoptBunfig(t, current)

	// Assert
	fmtTable, _ := parsedBunfig(t, merged)["fmt"].(map[string]any)
	patterns, _ := fmtTable["ignorePatterns"].([]any)

	for _, want := range []string{"out/", "gen/", "**/dist/"} {
		if !slices.Contains(patterns, any(want)) {
			t.Errorf("%q lost: %v", want, patterns)
		}
	}

	if !strings.Contains(merged, "\"gen/\", # generated") {
		t.Errorf("comment lost:\n%s", merged)
	}
}

func TestBunfigAppendsATableAndAnOverrideTheTemplateLacks(t *testing.T) {
	t.Parallel()

	// Arrange
	current := "[install.scopes]\n# The company registry.\ncorp = \"https://npm.example.com\"\n\n" +
		"[[fmt.overrides]]\nfiles = [\"*.json\"]\noptions = { printWidth = 120 }\n"

	// Act
	merged, _, root := adoptBunfig(t, current)

	// Assert
	suffix := "\n[install.scopes]\n# The company registry.\ncorp = \"https://npm.example.com\"\n\n" +
		"[[fmt.overrides]]\nfiles = [\"*.json\"]\noptions = { printWidth = 120 }\n"
	if !strings.HasSuffix(merged, suffix) {
		t.Errorf("merged:\n%s", merged)
	}

	secondAdoptKeepsBunfig(t, root, merged)
}

func TestBunfigKeepsTheTemplatesValueAndNotesTheProjectsOne(t *testing.T) {
	t.Parallel()

	// Arrange
	current := "env = true\n"

	// Act
	merged, notes, _ := adoptBunfig(t, current)

	// Assert
	if !strings.Contains(merged, "env = false") || strings.Contains(merged, "env = true") {
		t.Errorf("merged:\n%s", merged)
	}

	want := "`bunfig.toml`: kept the template's `env = false`; the repo had `env = true`."
	if !slices.Contains(notes, want) {
		t.Errorf("notes: %q", notes)
	}
}

func TestBunfigWritesTheTemplateOverACopyThatIsNotTOML(t *testing.T) {
	t.Parallel()

	// Arrange
	current := "[test\n"

	// Act
	merged, notes, _ := adoptBunfig(t, current)

	// Assert
	parsedBunfig(t, merged)

	if len(notes) == 0 || !strings.Contains(strings.Join(notes, "\n"), "not valid TOML") {
		t.Errorf("notes: %q", notes)
	}
}
