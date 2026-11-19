package jvm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/step"
)

// jvm selects the JVM language with the kind and group options.
func jvm(t *testing.T, kind, group string) plan.Selected {
	t.Helper()

	options := config.Options{}
	if kind != "" {
		options["kind"] = kind
	}

	if group != "" {
		options["group"] = group
	}

	return bedrocktest.Select(t, "jvm", options)
}

// planError plans an adopt of selected into root and returns its error.
func planError(t *testing.T, root string, selected plan.Selected) error {
	t.Helper()

	_, err := bedrocktest.Engine(t).Build(t.Context(), root, []plan.Selected{selected},
		plan.Options{Version: bedrocktest.Version, Dropped: nil, Init: false})

	return err
}

// removed returns the sorted paths the steps remove.
func removed(steps []step.Step) []string {
	var paths []string

	for _, each := range steps {
		if gone, ok := each.(step.Remove); ok {
			paths = append(paths, gone.Path)
		}
	}

	slices.Sort(paths)

	return paths
}

// hasNote reports whether a note contains text.
func hasNote(notes []string, text string) bool {
	return slices.ContainsFunc(notes, func(note string) bool { return strings.Contains(note, text) })
}

// guard runs the JVM guard in root.
func guard(t *testing.T, root string) []string {
	t.Helper()

	selected := jvm(t, "", "")

	guarder, ok := selected.Language.(language.Guarder)
	if !ok {
		t.Fatal("jvm has no guard")
	}

	found, err := guarder.Guard(t.Context(), bedrocktest.Context(t, root, selected), false)
	if err != nil {
		t.Fatal(err)
	}

	return found
}
