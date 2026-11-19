// Package plan computes what one adopt, init, or update changes: the base
// templates, each selected language's templates and own steps, the blocks of
// the shared files, and the removal of what dropped languages and replaced
// tools left. A plan lists only the steps the repository does not match yet,
// so a second adopt of the same selection plans none.
package plan

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// `replacedByBase` are the hook managers and task runners the `justfile` and
// `lefthook.yml` replace, with the `.githooks` of releases before 0.5 and the
// lefthook config names lefthook would read in place of `lefthook.yml`.
func replacedByBase() []string {
	return []string{
		".githooks",
		".husky",
		".lefthook.yaml",
		".lefthook.yml",
		".pre-commit-config.yaml",
		".simple-git-hooks.json",
		"lefthook.yaml",
		"Makefile",
	}
}

// ReplacedByBase returns the files an adopt removes because the base
// replaces what they did.
func ReplacedByBase() []string { return replacedByBase() }

// Engine plans with one build's languages and templates.
type Engine struct {
	Registry  *language.Registry
	Templates fs.FS
	Runner    execx.Runner
	Stdout    io.Writer
	Stderr    io.Writer
}

// Selected is a language picked for a run, with its settings.
type Selected struct {
	Language language.Language
	Options  config.Options
}

// Options says how Build plans.
type Options struct {
	// Version is the bedrock release .editorconfig records.
	Version string
	// Dropped are languages whose blocks and managed files the plan removes.
	Dropped []language.Language
	// Init starts a project, which adds each language's kinds/<kind> starters.
	Init bool
}

// Plan is everything one adopt, init, or update changes.
type Plan struct {
	// ProjectFiles are written paths whose content stays the project's own.
	ProjectFiles map[string]bool
	// Steps are what the repository does not match yet, ordered by path.
	Steps  []step.Step
	Notes  []string
	Config config.Config
}

// Context returns what lang plans and checks with in a run of selected.
func (e *Engine) Context(root string, selected []Selected, lang Selected, init bool) *language.Context {
	ids := make([]string, 0, len(selected))
	for _, entry := range selected {
		ids = append(ids, entry.Language.Spec().ID)
	}

	return &language.Context{
		Runner:    e.Runner,
		Templates: e.Templates,
		Stdout:    e.Stdout,
		Stderr:    e.Stderr,
		Options:   lang.Options,
		Root:      root,
		Languages: ids,
		Init:      init,
	}
}

// Build plans an adopt of selected into root, which must exist.
func (e *Engine) Build(ctx context.Context, root string, selected []Selected, options Options) (Plan, error) {
	languages := make(map[string]config.Options, len(selected))
	for _, entry := range selected {
		languages[entry.Language.Spec().ID] = entry.Options
	}

	cfg := config.Config{Version: options.Version, Languages: languages}
	base := template.Variables{
		"name":           filepath.Base(root),
		"bedrockVersion": options.Version,
		"bedrockConfig":  cfg.Pairs(),
	}

	var parts []contribution

	for _, entry := range selected {
		part, err := e.languageSteps(ctx, e.Context(root, selected, entry, options.Init), entry.Language, base)
		if err != nil {
			return Plan{}, fmt.Errorf("%s: %w", entry.Language.Spec().ID, err)
		}

		parts = append(parts, part)
	}

	steps, notes, err := e.assemble(root, selected, options.Dropped, base, parts)
	if err != nil {
		return Plan{}, err
	}

	pending, err := step.Pending(root, steps)
	if err != nil {
		return Plan{}, fmt.Errorf("compare the plan with `%s`: %w", root, err)
	}

	projectFiles := map[string]bool{}

	for _, part := range parts {
		for _, path := range part.ProjectFiles {
			projectFiles[path] = true
		}

		for _, starter := range part.Starters {
			projectFiles[starter.Target()] = true
		}
	}

	return Plan{Steps: pending, Notes: notes, ProjectFiles: projectFiles, Config: cfg}, nil
}

// assemble orders every step so a later one for the same path wins: leftover
// removals, base files, language files, shared files, retired files, then
// starters, which never overwrite.
func (e *Engine) assemble(
	root string,
	selected []Selected,
	dropped []language.Language,
	base template.Variables,
	parts []contribution,
) ([]step.Step, []string, error) {
	left, err := e.leftovers(root, dropped, base)
	if err != nil {
		return nil, nil, err
	}

	baseFiles, err := copySteps(template.NewDir(e.Templates, "base/files"), base, nil)
	if err != nil {
		return nil, nil, err
	}

	kept := make([]language.Language, 0, len(selected))
	for _, entry := range selected {
		kept = append(kept, entry.Language)
	}

	shared, err := e.sharedSteps(root, kept, left.Blocks, base)
	if err != nil {
		return nil, nil, err
	}

	retired := fsutil.Existing(root, append(replacedByBase(), config.LegacyPath))

	var steps []step.Step
	for _, path := range left.Files {
		steps = append(steps, step.Remove{Path: path})
	}

	steps = append(steps, baseFiles...)
	for _, part := range parts {
		steps = append(steps, part.Steps...)
	}

	steps = append(steps, shared...)
	for _, path := range retired {
		steps = append(steps, step.Remove{Path: path})
	}

	for _, part := range parts {
		steps = append(steps, part.Starters...)
	}

	steps = step.Dedupe(steps)

	notes := removalNotes(retired)
	notes = append(notes, left.notes(steps)...)

	for _, part := range parts {
		notes = append(notes, part.Notes...)
	}

	return steps, notes, nil
}

// removalNotes asks the user to move what the replaced hook and task-runner
// files did into project.just. A .bedrock.json needs no move: the bedrock
// block of .editorconfig now records what it held.
func removalNotes(removed []string) []string {
	moved := slices.DeleteFunc(slices.Clone(removed), func(path string) bool { return path == config.LegacyPath })
	if len(moved) == 0 {
		return nil
	}

	return []string{
		"Move what `" + strings.Join(
			moved,
			"`, `",
		) + "`" + " did into `project.just`; git still has the removed files.",
	}
}

// contribution is one language's share of a plan.
type contribution struct {
	language.Contribution

	Starters []step.Step
}

func (e *Engine) languageSteps(
	ctx context.Context,
	c *language.Context,
	lang language.Language,
	base template.Variables,
) (contribution, error) {
	spec := lang.Spec()
	vars := base

	if provider, ok := lang.(language.VariableProvider); ok {
		own, err := provider.Variables(ctx, c)
		if err != nil {
			return contribution{}, fmt.Errorf("template variables: %w", err)
		}

		vars = base.With(own)
	}

	var own language.Contribution

	if planner, ok := lang.(language.Planner); ok {
		var err error
		if own, err = planner.Plan(ctx, c); err != nil {
			return contribution{}, fmt.Errorf("plan: %w", err)
		}
	}

	files, err := copySteps(c.Dir(spec.ID, "files"), vars, spec.Executables)
	if err != nil {
		return contribution{}, err
	}

	dirs := []string{"starter"}
	if c.Init {
		kinds, kindErr := config.SplitList(c.Options["kind"])
		if kindErr != nil {
			return contribution{}, fmt.Errorf("kind: %w", kindErr)
		}

		for _, kind := range kinds {
			dirs = append(dirs, "kinds/"+kind)
		}
	}

	var starters []step.Step

	for _, dir := range dirs {
		steps, starterErr := starterSteps(c.Root, c.Dir(spec.ID, dir), vars, spec.Executables)
		if starterErr != nil {
			return contribution{}, starterErr
		}

		starters = append(starters, steps...)
	}

	own.Steps = append(files, own.Steps...)

	return contribution{Contribution: own, Starters: starters}, nil
}

// copySteps writes every template under dir. A path that matches one of
// executables is written runnable.
func copySteps(dir template.Dir, vars template.Variables, executables []string) ([]step.Step, error) {
	files, err := dir.Files(vars)
	if err != nil {
		return nil, fmt.Errorf("render `%s`: %w", dir.Name(), err)
	}

	steps := make([]step.Step, 0, len(files))

	for _, file := range files {
		steps = append(steps, step.Write{
			Path:       file.Path,
			Content:    file.Content,
			Executable: matchesAny(executables, file.Path),
		})
	}

	return steps, nil
}

// starterSteps writes the templates under dir that root does not have yet:
// a starter is the project's to change once written.
func starterSteps(root string, dir template.Dir, vars template.Variables, executables []string) ([]step.Step, error) {
	steps, err := copySteps(dir, vars, executables)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(steps, func(s step.Step) bool {
		return fsutil.Exists(filepath.Join(root, s.Target()))
	}), nil
}

func matchesAny(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if doublestar.MatchUnvalidated(pattern, path) {
			return true
		}
	}

	return false
}
