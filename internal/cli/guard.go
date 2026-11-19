package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/git"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/selection"
)

const baseScope = "base"

const guardUsage = `Usage: bedrock guard [base|<language>...] [--fix]

Reports what the linters cannot see: suppressions without a reason, files of
replaced tools, managed files that differ from the templates, and each
language's own rules. Without a scope, it checks base and every language
` + "`.editorconfig`" + ` lists. ` + "`--fix`" + ` repairs what it can first.`

// adopted reads the languages root records; a repository without a
// bedrock_version has none to check.
func (a *App) adopted(root string) (config.Config, []plan.Selected, error) {
	cfg, found, err := config.Read(root)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("read `%s`: %w", config.Path, err)
	}

	if !found {
		return config.Config{}, nil, fmt.Errorf("%s %w", root, errNotAdopted)
	}

	selected, err := a.selector().Select(cfg, nil, nil)
	if err != nil {
		return config.Config{}, nil, err
	}

	return cfg, selected, nil
}

func (a *App) guard(ctx context.Context, args []string) (int, error) {
	flags, help := flagSet("guard")
	fix := flags.Bool("fix", false, "")

	if err := parse(flags, args); err != nil {
		return ExitUsage, err
	}

	if *help {
		a.print(guardUsage)

		return ExitOK, nil
	}

	root := a.Dir

	cfg, selected, err := a.adopted(root)
	if err != nil {
		return ExitFailed, err
	}

	scopes, err := scopesOf(flags.Args(), selected)
	if err != nil {
		return ExitUsage, err
	}

	findings, err := a.findings(ctx, root, cfg, selected, scopes, *fix)
	if err != nil {
		return ExitFailed, err
	}

	for _, finding := range findings {
		_, _ = fmt.Fprintln(a.Stderr, finding)
	}

	if len(findings) > 0 {
		return ExitFailed, nil
	}

	return ExitOK, nil
}

// findings runs base and each scoped language's guard, then the size guard.
// --fix moves files, such as Rust tests out of the file they test, so sizes
// are measured once every fix is done.
func (a *App) findings(
	ctx context.Context,
	root string,
	cfg config.Config,
	selected []plan.Selected,
	scopes []string,
	fix bool,
) ([]string, error) {
	var findings []string

	if slices.Contains(scopes, baseScope) {
		base, err := a.guardBase(ctx, root, cfg, selected)
		if err != nil {
			return nil, err
		}

		findings = append(findings, base...)
	}

	var inScope []plan.Selected

	for _, entry := range selected {
		if !slices.Contains(scopes, entry.Language.Spec().ID) {
			continue
		}

		inScope = append(inScope, entry)

		guarder, ok := entry.Language.(language.Guarder)
		if !ok {
			continue
		}

		found, err := guarder.Guard(ctx, a.engine().Context(root, selected, entry, false), fix)
		if err != nil {
			return nil, fmt.Errorf("%s guard: %w", entry.Language.Spec().ID, err)
		}

		findings = append(findings, found...)
	}

	if a.Sizes == nil {
		return findings, nil
	}

	sizes, err := a.Sizes(ctx, a.engine(), root, inScope)
	if err != nil {
		return nil, fmt.Errorf("size guard: %w", err)
	}

	for _, warning := range sizes.Warnings {
		_, _ = fmt.Fprintln(a.Stderr, "warning: "+warning)
	}

	return append(findings, sizes.Errors...), nil
}

// scopesOf returns the scopes to check: those named, or base and every adopted language.
func scopesOf(named []string, selected []plan.Selected) ([]string, error) {
	ids := make([]string, 0, len(selected))
	for _, entry := range selected {
		ids = append(ids, entry.Language.Spec().ID)
	}

	if len(named) == 0 {
		return append([]string{baseScope}, ids...), nil
	}

	var unknown []string

	for _, scope := range named {
		if scope != baseScope && !slices.Contains(ids, scope) {
			unknown = append(unknown, scope)
		}
	}

	if len(unknown) > 0 {
		return nil, fmt.Errorf("%w: not adopted here: %s. Scopes: %s", selection.ErrUsage,
			strings.Join(unknown, ", "), strings.Join(append([]string{baseScope}, ids...), ", "))
	}

	return named, nil
}

// guardBase checks the rules every bedrock repository follows, whatever its languages.
func (a *App) guardBase(
	ctx context.Context,
	root string,
	cfg config.Config,
	selected []plan.Selected,
) ([]string, error) {
	var findings []string

	for _, link := range []string{"CLAUDE.md", "GEMINI.md"} {
		if dest, ok := fsutil.LinkTarget(filepath.Join(root, link)); !ok || dest != "AGENTS.md" {
			findings = append(findings, link+": make it a symlink to `AGENTS.md`")
		}
	}

	for _, path := range fsutil.Existing(root, plan.ReplacedByBase()) {
		findings = append(findings, path+": replaced by the `justfile` and `lefthook.yml`; "+
			"move what it still does into `project.just`")
	}

	if a.Getenv("CI") == "" && !(git.Git{Runner: a.Runner, Root: root}).HooksOn(ctx) {
		findings = append(findings, "git hooks are off: run `just setup`")
	}

	backups, err := findBackups(root)
	if err != nil {
		return nil, err
	}

	for _, path := range backups {
		findings = append(findings, path+": a backup from an older bedrock; run `bedrock clean`")
	}

	if cfg.Version != config.SourceVersion && cfg.Version != a.Version {
		findings = append(findings, fmt.Sprintf(
			"%s: `bedrock_version` pins %s, but %s runs; run `just setup`", config.Path, cfg.Version, a.Version))
	}

	drifted, err := a.drift(ctx, root, cfg, selected)
	if err != nil {
		return nil, err
	}

	return append(findings, drifted...), nil
}

// drift returns the managed files that differ from the templates root pins.
func (a *App) drift(ctx context.Context, root string, cfg config.Config, selected []plan.Selected) ([]string, error) {
	dropped, err := a.selector().Dropped(ctx, root, selected)
	if err != nil {
		return nil, err
	}

	built, err := a.engine().
		Build(ctx, root, selected, plan.Options{Version: cfg.Version, Dropped: dropped, Init: false})
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}

	var findings []string

	for _, s := range built.Steps {
		if !built.ProjectFiles[s.Target()] {
			findings = append(findings, s.Target()+": differs from what bedrock writes; run `bedrock update`")
		}
	}

	return findings, nil
}
