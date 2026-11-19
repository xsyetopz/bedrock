// Package selection decides which languages a run covers: those the
// repository records, those the command line names, and those its marker
// files show. Each language stands alone, so a repository mixes any of them.
package selection

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/template"
)

// ErrUsage reports a command line that names languages or flags wrongly.
var ErrUsage = errors.New("usage")

// Named is a language the command line names, as <name> or <name>:<kind>[,<kind>].
type Named struct {
	Language language.Language
	// Kind is the comma list after the colon; empty when there is none.
	Kind string
}

// Selector selects among one registry's languages.
type Selector struct {
	Registry  *language.Registry
	Templates fs.FS
}

// Parse reads <name>[:<kind>], checking the name and each kind.
func (s Selector) Parse(arg string) (Named, error) {
	name, kind, hasKind := strings.Cut(arg, ":")

	lang, ok := s.Registry.Find(name)
	if !ok {
		return Named{}, fmt.Errorf("%w: unknown language `%s`. Known: %s", ErrUsage, name, s.Registry.Known())
	}

	if !hasKind {
		return Named{Language: lang, Kind: ""}, nil
	}

	kinds := lang.Spec().Kinds

	entries, err := config.SplitList(kind)
	if err != nil {
		return Named{}, fmt.Errorf("%w: %s: %w", ErrUsage, arg, err)
	}

	if len(entries) == 0 {
		return Named{}, fmt.Errorf("%w: `%s` needs a kind after the colon, as in `%s:<kind>[,<kind>]`",
			ErrUsage, arg, lang.Spec().ID)
	}

	var unknown []string

	for _, entry := range entries {
		if !slices.Contains(kinds, entry) {
			unknown = append(unknown, entry)
		}
	}

	if len(unknown) > 0 {
		known := strings.Join(kinds, ", ")
		if known == "" {
			known = "none"
		}

		return Named{}, fmt.Errorf("%w: %s has no kind `%s`. Kinds: %s",
			ErrUsage, lang.Spec().ID, strings.Join(unknown, ", "), known)
	}

	return Named{Language: lang, Kind: strings.Join(entries, ",")}, nil
}

// Detect returns the languages whose markers sit at root.
func (s Selector) Detect(root string) []language.Language {
	var found []language.Language

	for _, lang := range s.Registry.All() {
		if hasMarker(root, lang.Spec().Markers, nil) {
			found = append(found, lang)
		}
	}

	return found
}

// Dropped returns the languages a run leaves out that no marker at root
// points to. A repository that dropped one keeps no block or file of it; one
// whose markers remain, such as a repository adopted before bedrock recorded
// its languages, keeps it. Markers bedrock writes itself, such as gradlew,
// do not count: they outlive the code they were written for.
func (s Selector) Dropped(root string, selected []plan.Selected) ([]language.Language, error) {
	var dropped []language.Language

	for _, lang := range s.Registry.All() {
		if slices.ContainsFunc(selected, func(entry plan.Selected) bool { return entry.Language == lang }) {
			continue
		}

		written, err := template.NewDir(s.Templates, lang.Spec().ID+"/files").Paths()
		if err != nil {
			return nil, fmt.Errorf("list %s files: %w", lang.Spec().ID, err)
		}

		if !hasMarker(root, lang.Spec().Markers, written) {
			dropped = append(dropped, lang)
		}
	}

	return dropped, nil
}

// Select returns the languages a run covers in block order: those cfg
// records, those named, and those the set flags select. flags maps a
// language flag's name to its value, "true" for a boolean one.
func (s Selector) Select(cfg config.Config, named []Named, flags map[string]string) ([]plan.Selected, error) {
	options := map[string]config.Options{}
	for id, opts := range cfg.Languages {
		options[id] = maps.Clone(opts)
	}

	add := func(id string, extra config.Options) {
		options[id] = options[id].Merge(extra)
	}

	for _, entry := range named {
		extra := config.Options{}
		if entry.Kind != "" {
			extra["kind"] = entry.Kind
		}

		add(entry.Language.Spec().ID, extra)
	}

	for _, owned := range s.Registry.Flags() {
		value, set := flags[owned.Flag.Name]
		if !set {
			continue
		}

		id := owned.Language.Spec().ID
		if err := checkFlag(owned.Flag, id, value, options); err != nil {
			return nil, err
		}

		if owned.Flag.Value == "" {
			add(id, config.Options{})
		} else {
			add(id, config.Options{owned.Flag.Name: value})
		}
	}

	var selected []plan.Selected

	for _, lang := range s.Registry.All() {
		if opts, ok := options[lang.Spec().ID]; ok {
			selected = append(selected, plan.Selected{Language: lang, Options: opts})
		}
	}

	return selected, nil
}

func checkFlag(flag language.Flag, id, value string, options map[string]config.Options) error {
	if _, ok := options[id]; flag.Requires && !ok {
		return fmt.Errorf("%w: `--%s` sets the %s %s; name `%s` too", ErrUsage, flag.Name, id, flag.Name, id)
	}

	if flag.Validate != nil {
		if err := flag.Validate(value); err != nil {
			return fmt.Errorf("%w: `--%s %s`: %w", ErrUsage, flag.Name, value, err)
		}
	}

	return nil
}

// MissingTools returns the tools git, just, and the selected languages need
// that lookPath cannot find, each with how to install it, sorted.
func MissingTools(selected []plan.Selected, lookPath func(string) bool) []string {
	tools := map[string]string{
		"git":  "the system's package manager, such as `brew install git`",
		"just": "`brew install just`, or `cargo install --locked just`",
	}

	for _, entry := range selected {
		for _, tool := range entry.Language.Spec().Tools {
			tools[tool.Name] = tool.Install
		}
	}

	var missing []string

	for name, install := range tools {
		if !lookPath(name) {
			missing = append(missing, name+": "+install)
		}
	}

	slices.Sort(missing)

	return missing
}

// hasMarker reports whether a marker other than those ignored, a path or a
// glob, matches a file under root.
func hasMarker(root string, markers, ignored []string) bool {
	var globs []string

	for _, marker := range markers {
		switch {
		case slices.Contains(ignored, marker):
		case strings.ContainsAny(marker, "*?{["):
			globs = append(globs, marker)
		case fsutil.Exists(filepath.Join(root, marker)):
			return true
		}
	}

	return slices.ContainsFunc(globs, func(glob string) bool { return matchesFile(root, glob) })
}

// errFound stops a glob walk at its first match.
var errFound = errors.New("found")

func matchesFile(root, glob string) bool {
	err := doublestar.GlobWalk(os.DirFS(root), glob, func(string, fs.DirEntry) error {
		return errFound
	}, doublestar.WithFilesOnly())

	return errors.Is(err, errFound)
}
