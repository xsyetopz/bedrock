package plan_test

import (
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/catalog"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
)

// zig is a language registered outside the catalog, as a new one starts.
type zig struct{}

func (zig) Spec() language.Spec { return language.Spec{ID: "zig", Block: "zig-bedrock"} }

func TestTheJustfileRunsTasksForEveryRegisteredLanguage(t *testing.T) {
	t.Parallel()

	// Arrange
	registry, err := language.NewRegistry(append(catalog.Languages(), zig{})...)
	if err != nil {
		t.Fatal(err)
	}

	engine := bedrocktest.Engine(t)
	engine.Registry = registry
	root := bedrocktest.Repo(t, nil)

	// Act
	built, err := engine.Build(t.Context(), root, nil, plan.Options{Version: bedrocktest.Version})
	if err != nil {
		t.Fatal(err)
	}

	justfile := bedrocktest.Written(built.Steps, "justfile")

	// Assert
	for _, id := range registry.IDs() {
		module := `mod? ` + id + ` ".just/` + id + `.just"`
		if !strings.Contains(justfile, module) {
			t.Errorf("the `justfile` lacks %s", module)
		}
	}

	if !strings.Contains(justfile, `languages := "typescript rust swift cpp jvm go haxe dotnet zig"`) {
		t.Errorf("the `justfile` does not list the languages in block order:\n%s", justfile)
	}
}
