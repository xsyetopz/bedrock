package typescript_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func tsconfigOf(t *testing.T, input string) (map[string]any, []string, string) {
	t.Helper()

	root := bedrocktest.Repo(t, map[string]string{packageJSON: "{}\n", "tsconfig.json": input})
	result := bedrocktest.Adopt(t, root, false, selected(t))

	var parsed map[string]any
	if err := json.Unmarshal([]byte(bedrocktest.Read(t, root, "tsconfig.json")), &parsed); err != nil {
		t.Fatal(err)
	}

	return parsed, result.Notes, root
}

func optionsOf(config map[string]any) map[string]any {
	options, _ := config["compilerOptions"].(map[string]any)

	return options
}

func TestTsconfigDropsOptionsTypeScriptSevenRemovedAndNotesThem(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"compilerOptions":{"esModuleInterop":true,"outFile":"a.js","paths":{"@/*":["src/*"]}}}`

	// Act
	config, notes, _ := tsconfigOf(t, input)

	// Assert
	options := optionsOf(config)
	if _, ok := options["esModuleInterop"]; ok {
		t.Errorf("options: %v", options)
	}

	if _, ok := options["paths"]; !ok {
		t.Errorf("the project's paths were lost: %v", options)
	}

	want := "`tsconfig.json`: removed `\"esModuleInterop\"`; TypeScript 7 rejects it or always behaves as if it were on."
	if !slices.Contains(notes, want) {
		t.Errorf("notes: %q", notes)
	}
}

func TestTsconfigReplacesBaseURLWithPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"compilerOptions":{"baseUrl":"src","paths":{"@/*":["lib/*"]}}}`

	// Act
	config, _, _ := tsconfigOf(t, input)

	// Assert
	options := optionsOf(config)
	if _, ok := options["baseUrl"]; ok {
		t.Errorf("baseUrl stayed: %v", options)
	}

	paths, _ := options["paths"].(map[string]any)
	if got, _ := paths["@/*"].([]any); !slices.Equal(got, []any{"./src/lib/*"}) {
		t.Errorf("paths: %v", paths)
	}
}

func TestTsconfigTurnsEmitOnForAProjectThatEmits(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"compilerOptions":{"outDir":"dist","declaration":true}}`

	// Act
	config, notes, _ := tsconfigOf(t, input)

	// Assert
	if emit, ok := optionsOf(config)["noEmit"].(bool); !ok || emit {
		t.Errorf("options: %v", optionsOf(config))
	}

	if !slices.ContainsFunc(notes, func(note string) bool { return strings.Contains(note, "emits with `tsc`") }) {
		t.Errorf("notes: %q", notes)
	}
}

func TestTsconfigKeepsTheProjectsTypesAndAddsBun(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"compilerOptions":{"types":["vitest"]}}`

	// Act
	config, _, _ := tsconfigOf(t, input)

	// Assert
	types, _ := optionsOf(config)["types"].([]any)
	if !slices.Contains(types, any("vitest")) || !slices.Contains(types, any("bun")) {
		t.Errorf("types: %v", types)
	}
}

func TestTsconfigKeepsAnExtendsThatIsNotTheLegacyBase(t *testing.T) {
	t.Parallel()

	// Arrange
	input := `{"extends":"@acme/tsconfig/base.json"}`

	// Act
	config, notes, root := tsconfigOf(t, input)

	// Assert
	if got, _ := config["extends"].([]any); !slices.Equal(got, []any{"@acme/tsconfig/base.json"}) {
		t.Errorf("extends: %v", config["extends"])
	}

	if !slices.ContainsFunc(
		notes,
		func(note string) bool { return strings.Contains(note, "still extends `@acme/tsconfig/base.json`") },
	) {
		t.Errorf("notes: %q", notes)
	}

	if again := bedrocktest.Plan(
		t,
		root,
		false,
		selected(t),
	); slices.Contains(
		bedrocktest.Paths(again.Steps),
		"tsconfig.json",
	) {
		t.Error("second adopt rewrites `tsconfig.json`")
	}
}

func TestAdoptLeavesACommentedTsconfigAloneOnceMigrated(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(
		t,
		map[string]string{
			packageJSON:     "{}\n",
			"tsconfig.json": "{\n\t// the project's\n\t\"compilerOptions\": {}\n}\n",
		},
	)
	bedrocktest.Adopt(t, root, false, selected(t))
	migrated := bedrocktest.Read(t, root, "tsconfig.json")
	bedrocktest.Write(t, root, "tsconfig.json", "// kept\n"+migrated)

	// Act
	again := bedrocktest.Plan(t, root, false, selected(t))

	// Assert
	if slices.Contains(bedrocktest.Paths(again.Steps), "tsconfig.json") {
		t.Error("adopt rewrote a migrated `tsconfig.json`, dropping its comment")
	}
}
