package haxe_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

const buildHxml = "-cp src\n-js out.js\n"

func TestAdoptWritesRecipesWorkflowAndStarterCheckstyleConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"build.hxml": buildHxml})

	// Act
	result := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "haxe", nil))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	for _, want := range []string{".just/haxe.just", ".github/workflows/haxe.yml", "checkstyle.json"} {
		if !slices.Contains(paths, want) {
			t.Errorf("plan misses %s: %v", want, paths)
		}
	}
}

func TestSecondAdoptPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"build.hxml": buildHxml})
	bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "haxe", nil))

	// Act
	again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "haxe", nil))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestAdoptNotesAHandWrittenWorkflowItReplacesAndNotItsOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"build.hxml":                 buildHxml,
		".github/workflows/haxe.yml": "name: Haxe\n",
	})

	// Act
	first := bedrocktest.Adopt(t, root, false, bedrocktest.Select(t, "haxe", nil))
	again := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "haxe", nil))

	// Assert
	if !strings.Contains(strings.Join(first.Notes, "\n"), "`.github/workflows/haxe.yml`: replaced with bedrock's") {
		t.Errorf("notes: %v", first.Notes)
	}

	if len(again.Notes) != 0 {
		t.Errorf("second adopt notes %v", again.Notes)
	}
}

func TestAdoptNotesATestHxmlThatNamesNoWayToRunItsTests(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"test.hxml":        "-cp test\n--main TestMain\n-lib utest\n",
		"test-interp.hxml": "-cp test\n--main TestMain\n--interp\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, bedrocktest.Select(t, "haxe", nil))

	// Assert
	want := "`test.hxml`: names no `--interp`, `--run`, or `--cmd`, so `just haxe::test` does not run it; " +
		"a `project-test` recipe in `project.just` runs it on its target."
	if len(result.Notes) != 1 || result.Notes[0] != want {
		t.Errorf("notes = %q, want [%q]", result.Notes, want)
	}
}

func TestAdoptHaxeBesideGoInOneRepo(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"go.mod":     "module m\n\ngo 1.27\n",
		"build.hxml": buildHxml,
	})

	// Act
	result := bedrocktest.Plan(t, root, false,
		bedrocktest.Select(t, "go", nil), bedrocktest.Select(t, "haxe", nil))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	for _, want := range []string{".just/haxe.just", ".just/go.just"} {
		if !slices.Contains(paths, want) {
			t.Errorf("plan misses %s: %v", want, paths)
		}
	}
}
