package selection_test

import (
	"errors"
	"testing"

	"github.com/xsyetopz/bedrock/internal/catalog"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/selection"
)

// apple offers an app on two platforms and a library on the host alone.
type apple struct{}

func (apple) Spec() language.Spec {
	return language.Spec{
		ID:        "apple",
		Block:     "apple-bedrock",
		Kinds:     []string{"app", "library"},
		Platforms: map[string][]string{"app": {"ios", "macos"}},
	}
}

func appleSelector(t *testing.T) selection.Selector {
	t.Helper()

	registry, err := language.NewRegistry(append(catalog.Languages(), apple{})...)
	if err != nil {
		t.Fatal(err)
	}

	sel := selector(t)
	sel.Registry = registry

	return sel
}

func TestParseSplitsPlatformsFromKinds(t *testing.T) {
	t.Parallel()

	// Act
	named, err := appleSelector(t).Parse("apple:app@ios,macos")
	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if named.Kind != "app" || named.Platform != "ios,macos" {
		t.Errorf("parsed kind %q platform %q, want app and ios,macos", named.Kind, named.Platform)
	}
}

func TestParseRejectsAPlatformAKindLacks(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{"apple:app@android", "apple:app,library@ios", "apple:app@"} {
		// Act
		_, err := appleSelector(t).Parse(arg)

		// Assert
		if !errors.Is(err, selection.ErrUsage) {
			t.Errorf("Parse(%q) = %v, want a usage error", arg, err)
		}
	}
}

func TestSelectRecordsThePlatforms(t *testing.T) {
	t.Parallel()

	// Arrange
	sel := appleSelector(t)

	named, err := sel.Parse("apple:app@macos")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	selected, err := sel.Select(
		config.Config{Version: "", Languages: map[string]config.Options{}},
		[]selection.Named{named},
		nil,
	)
	// Assert
	if err != nil {
		t.Fatal(err)
	}

	if len(selected) != 1 || selected[0].Options["platform"] != "macos" {
		t.Errorf("selected %+v, want apple with platform macos", selected)
	}
}

// unlisted names platforms for a kind it lacks.
type unlisted struct{}

func (unlisted) Spec() language.Spec {
	return language.Spec{ID: "unlisted", Block: "unlisted-bedrock", Platforms: map[string][]string{"app": {"ios"}}}
}

// misspelled names a platform outside the shared vocabulary.
type misspelled struct{}

func (misspelled) Spec() language.Spec {
	return language.Spec{
		ID: "misspelled", Block: "misspelled-bedrock",
		Kinds: []string{"app"}, Platforms: map[string][]string{"app": {"iphone"}},
	}
}

func TestRegistryRejectsUnknownPlatformsAndKinds(t *testing.T) {
	t.Parallel()

	for _, lang := range []language.Language{unlisted{}, misspelled{}} {
		// Act
		_, err := language.NewRegistry(lang)

		// Assert
		if !errors.Is(err, language.ErrInvalidSpec) {
			t.Errorf("NewRegistry(%s) = %v, want an invalid spec error", lang.Spec().ID, err)
		}
	}
}
