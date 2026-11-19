package selection_test

import (
	"io"
	"testing"

	"github.com/xsyetopz/bedrock"
	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/catalog"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/selection"
)

func selector(t *testing.T) selection.Selector {
	t.Helper()

	registry, err := catalog.Registry()
	if err != nil {
		t.Fatal(err)
	}

	return selection.Selector{
		Registry:  registry,
		Templates: bedrock.Templates(),
		Runner:    execx.System{Stdout: io.Discard, Stderr: io.Discard},
	}
}

func ids(languages []language.Language) []string {
	found := make([]string, 0, len(languages))
	for _, lang := range languages {
		found = append(found, lang.Spec().ID)
	}

	return found
}

func TestDetectFindsAGlobMarkerInASubdirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"tools/hxml/compile.hxml": "-main Main\n"})

	// Act
	found, err := selector(t).Detect(t.Context(), root)

	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if got := ids(found); len(got) != 1 || got[0] != "haxe" {
		t.Errorf("detected %v, want [haxe]", got)
	}
}

func TestDetectSkipsFilesGitIgnores(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		".gitignore":                        "vendor\nnode_modules\n",
		"vendor/lib/src/Lib.hx":             "class Lib {}\n",
		"node_modules/pkg/Package.csproj":   "<Project />\n",
		"node_modules/pkg/tools/build.hxml": "-main Main\n",
	})

	// Act
	found, err := selector(t).Detect(t.Context(), root)

	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if len(found) != 0 {
		t.Errorf("detected %v from ignored or dependency files", ids(found))
	}
}
