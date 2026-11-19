// Package selection decides which languages a run covers: those the
// repository records, those the command line names, and those its marker
// files show. Each language stands alone, so a repository mixes any of them.
package selection

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/template"
)

// ErrUsage reports a command line that names languages or flags wrongly.
var ErrUsage = errors.New("usage")

// ErrUnknownRecorded reports a repository that records a language this build
// lacks, such as one a newer bedrock added.
var ErrUnknownRecorded = errors.New("unknown recorded language")

// Named is a language the command line names, as <name> or
// <name>:<kind>[,<kind>][@<platform>[,<platform>]].
type Named struct {
	Language language.Language
	// Kind is the comma list after the colon; empty when there is none.
	Kind string
	// Platform is the comma list after the at sign; empty when there is none.
	Platform string
}

// Selector selects among one registry's languages.
type Selector struct {
	Registry  *language.Registry
	Templates fs.FS
	// Runner lists the repository's files with git for glob markers.
	Runner execx.Runner
}

// Parse reads <name>[:<kind>[@<platform>]], checking the name, each kind,
// and that every kind named builds for each platform. The at sign splits
// first, so `swift:library,app@ios,macos` gives both kinds both platforms.
func (s Selector) Parse(arg string) (Named, error) {
	name, kind, hasKind := strings.Cut(arg, ":")

	lang, ok := s.Registry.Find(name)
	if !ok {
		return Named{}, fmt.Errorf("%w: unknown language `%s`. Known: %s", ErrUsage, name, s.Registry.Known())
	}

	if !hasKind {
		return Named{Language: lang, Kind: "", Platform: ""}, nil
	}

	kind, platform, hasPlatform := strings.Cut(kind, "@")

	kinds, err := parseKinds(lang.Spec(), arg, kind)
	if err != nil {
		return Named{}, err
	}

	var platforms []string

	if hasPlatform {
		if platforms, err = parsePlatforms(lang.Spec(), arg, kinds, platform); err != nil {
			return Named{}, err
		}
	}

	return Named{Language: lang, Kind: strings.Join(kinds, ","), Platform: strings.Join(platforms, ",")}, nil
}

// parseKinds checks the comma list of kinds after the colon.
func parseKinds(spec language.Spec, arg, list string) ([]string, error) {
	entries, err := config.SplitList(list)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrUsage, arg, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: `%s` needs a kind after the colon, as in `%s:<kind>[,<kind>]`",
			ErrUsage, arg, spec.ID)
	}

	var unknown []string

	for _, entry := range entries {
		if !slices.Contains(spec.Kinds, entry) {
			unknown = append(unknown, entry)
		}
	}

	if len(unknown) > 0 {
		known := strings.Join(spec.Kinds, ", ")
		if known == "" {
			known = "none"
		}

		return nil, fmt.Errorf("%w: %s has no kind `%s`. Kinds: %s",
			ErrUsage, spec.ID, strings.Join(unknown, ", "), known)
	}

	return entries, nil
}

// parsePlatforms checks the comma list after the at sign against every kind.
func parsePlatforms(spec language.Spec, arg string, kinds []string, list string) ([]string, error) {
	entries, err := config.SplitList(list)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrUsage, arg, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: `%s` needs a platform after the at sign, as in `%s:<kind>@<platform>[,<platform>]`",
			ErrUsage, arg, spec.ID)
	}

	for _, kind := range kinds {
		offered := spec.KindPlatforms(kind)

		for _, entry := range entries {
			if slices.Contains(offered, entry) {
				continue
			}

			known := strings.Join(offered, ", ")
			if known == "" {
				known = "none; it builds for the host"
			}

			return nil, fmt.Errorf("%w: %s kind `%s` has no platform `%s`. Platforms: %s",
				ErrUsage, spec.ID, kind, entry, known)
		}
	}

	return entries, nil
}

// Detect returns the languages whose markers sit at root.
func (s Selector) Detect(ctx context.Context, root string) ([]language.Language, error) {
	var found []language.Language

	search := s.markers(ctx, root)

	for _, lang := range s.Registry.All() {
		has, err := search.has(lang.Spec().Markers, nil)
		if err != nil {
			return nil, err
		}

		if has {
			found = append(found, lang)
		}
	}

	return found, nil
}

// Dropped returns the languages a run leaves out that no marker at root
// points to. A repository that dropped one keeps no block or file of it; one
// whose markers remain, such as a repository adopted before bedrock recorded
// its languages, keeps it. Markers bedrock writes itself, such as gradlew,
// do not count: they outlive the code they were written for.
func (s Selector) Dropped(ctx context.Context, root string, selected []plan.Selected) ([]language.Language, error) {
	var dropped []language.Language

	search := s.markers(ctx, root)

	for _, lang := range s.Registry.All() {
		if slices.ContainsFunc(selected, func(entry plan.Selected) bool { return entry.Language == lang }) {
			continue
		}

		written, err := template.NewDir(s.Templates, lang.Spec().ID+"/files").Paths()
		if err != nil {
			return nil, fmt.Errorf("list %s files: %w", lang.Spec().ID, err)
		}

		has, err := search.has(lang.Spec().Markers, written)
		if err != nil {
			return nil, err
		}

		if !has {
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

	for _, id := range slices.Sorted(maps.Keys(cfg.Languages)) {
		// A release that predates a language would otherwise rewrite the repo
		// without it, deleting its block and dropping it from the config.
		if _, ok := s.Registry.Find(id); !ok {
			return nil, fmt.Errorf("%w: `%s` lists `%s`, which this bedrock does not know. Known: %s; "+
				"run the release `bedrock_version` pins, or a newer one", ErrUnknownRecorded, config.Path, id,
				s.Registry.Known())
		}

		options[id] = maps.Clone(cfg.Languages[id])
	}

	add := func(id string, extra config.Options) {
		options[id] = options[id].Merge(extra)
	}

	for _, entry := range named {
		extra := config.Options{}
		if entry.Kind != "" {
			extra["kind"] = entry.Kind
		}

		if entry.Platform != "" {
			extra["platform"] = entry.Platform
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

// markerSearch finds markers under one root. It lists the files for glob
// markers at most once, through git, so a search skips what git ignores, such
// as node_modules or other worktrees, as the language checks do.
type markerSearch struct {
	list  func() ([]string, error)
	root  string
	files []string
	ready bool
}

func (s Selector) markers(ctx context.Context, root string) *markerSearch {
	return &markerSearch{
		root: root,
		list: func() ([]string, error) {
			return fsutil.ListFiles(ctx, s.Runner, root, func(string) bool { return true })
		},
	}
}

// has reports whether a marker other than those ignored, a path or a glob,
// matches a file under the root.
func (m *markerSearch) has(markers, ignored []string) (bool, error) {
	var globs []string

	for _, marker := range markers {
		switch {
		case slices.Contains(ignored, marker):
		case strings.ContainsAny(marker, "*?{["):
			globs = append(globs, marker)
		case fsutil.Exists(filepath.Join(m.root, marker)):
			return true, nil
		}
	}

	if len(globs) == 0 {
		return false, nil
	}

	if !m.ready {
		files, err := m.list()
		if err != nil {
			return false, fmt.Errorf("list the files under `%s`: %w", m.root, err)
		}

		m.files, m.ready = files, true
	}

	for _, file := range m.files {
		if slices.ContainsFunc(globs, func(glob string) bool { return doublestar.MatchUnvalidated(glob, file) }) {
			return true, nil
		}
	}

	return false, nil
}
