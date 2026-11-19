package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/git"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/selection"
	"github.com/xsyetopz/bedrock/internal/step"
)

// errNotAdopted reports a repository whose .editorconfig records no bedrock_version.
var errNotAdopted = errors.New("records no `bedrock_version` in `.editorconfig`. Run `bedrock adopt` first")

// errDeclined reports an adopt a person, or the lack of one, did not approve.
var errDeclined = errors.New("declined")

// adoptFlags are the options adopt, init, and update share.
type adoptFlags struct {
	// languages maps each set language flag to its value, "true" for a boolean one.
	languages  map[string]string
	help       bool
	dryRun     bool
	yes        bool
	noRun      bool
	allowDirty bool
}

func (a *App) parseAdopt(mode mode, args []string) (adoptFlags, []string, error) {
	flags, help := flagSet(string(mode))
	dryRun := flags.Bool("dry-run", false, "")
	yes := flags.Bool("yes", false, "")
	noRun := flags.Bool("no-run", false, "")
	allowDirty := flags.Bool("allow-dirty", false, "")

	owned := a.Registry.Flags()
	values := make([]*string, len(owned))
	switches := make([]*bool, len(owned))

	for index, entry := range owned {
		if entry.Flag.Value == "" {
			switches[index] = flags.Bool(entry.Flag.Name, false, entry.Flag.Usage)
		} else {
			values[index] = flags.String(entry.Flag.Name, "", entry.Flag.Usage)
		}
	}

	if err := parse(flags, args); err != nil {
		return adoptFlags{}, nil, err
	}

	set := map[string]string{}

	for index, entry := range owned {
		name := entry.Flag.Name
		if !flags.Changed(name) {
			continue
		}

		if switches[index] != nil {
			if *switches[index] {
				set[name] = "true"
			}

			continue
		}

		set[name] = *values[index]
	}

	return adoptFlags{
		languages:  set,
		help:       *help,
		dryRun:     *dryRun,
		yes:        *yes,
		noRun:      *noRun,
		allowDirty: *allowDirty,
	}, flags.Args(), nil
}

func (a *App) adoptCommand(mode mode) func(context.Context, []string) (int, error) {
	return func(ctx context.Context, args []string) (int, error) {
		return a.adopt(ctx, mode, args)
	}
}

func (a *App) adopt(ctx context.Context, mode mode, args []string) (int, error) {
	flags, positionals, err := a.parseAdopt(mode, args)
	if err != nil {
		return ExitUsage, err
	}

	if flags.help || (mode == modeInit && len(positionals) < 2) {
		a.print(a.adoptUsage(mode))

		if flags.help {
			return ExitOK, nil
		}

		return ExitUsage, nil
	}

	root, names, err := a.target(mode, positionals)
	if err != nil {
		return ExitUsage, err
	}

	if prepareErr := a.prepare(ctx, mode, root, flags.dryRun); prepareErr != nil {
		return ExitFailed, prepareErr
	}

	selected, err := a.selection(ctx, mode, root, names, flags.languages)
	if err != nil {
		return ExitFailed, err
	}

	init := mode == modeInit

	built, err := a.planFor(ctx, root, selected, init)
	if err != nil {
		return ExitFailed, err
	}

	a.printPlan(root, built)

	if flags.dryRun {
		return ExitOK, nil
	}

	return a.apply(ctx, root, selected, built, init, flags)
}

// change applies the plan and runs the after-runs once a person approves;
// errDeclined means no one did.
func (a *App) change(
	ctx context.Context,
	root string,
	selected []plan.Selected,
	built plan.Plan,
	runs []plan.Selected,
	init bool,
	flags adoptFlags,
) error {
	if len(built.Steps) > 0 && !init && !flags.allowDirty {
		if err := a.checkClean(ctx, root, built); err != nil {
			return err
		}
	}

	if !a.confirm(flags.yes) {
		return errDeclined
	}

	if err := step.Apply(root, built.Steps); err != nil {
		return fmt.Errorf("apply the plan: %w", err)
	}

	if flags.noRun {
		return nil
	}

	return a.runAfterApply(ctx, root, selected, runs, init)
}

func (a *App) apply(
	ctx context.Context,
	root string,
	selected []plan.Selected,
	built plan.Plan,
	init bool,
	flags adoptFlags,
) (int, error) {
	runs, err := a.toRun(ctx, root, selected, built, init)
	if err != nil {
		return ExitFailed, err
	}

	a.printRuns(runs, flags.noRun)

	if len(built.Steps) > 0 || (len(runs) > 0 && !flags.noRun) {
		changeErr := a.change(ctx, root, selected, built, runs, init, flags)
		if errors.Is(changeErr, errDeclined) {
			return ExitFailed, nil
		}

		if changeErr != nil {
			return ExitFailed, changeErr
		}
	}

	cleanup, err := a.removeBackups(ctx, root, false)
	if err != nil {
		return ExitFailed, err
	}

	a.reportCleanup(cleanup)
	a.print(
		"Done. Review the change with `git diff`, then run `just setup` and `just verify`, and fix what they report.",
	)

	return ExitOK, nil
}

// selection returns the languages a run covers and fails when it covers
// none or their tools are missing, before anything changes.
func (a *App) selection(
	ctx context.Context,
	mode mode,
	root string,
	names []string,
	flags map[string]string,
) ([]plan.Selected, error) {
	cfg, found, err := config.Read(root)
	if err != nil {
		return nil, fmt.Errorf("read `%s`: %w", config.Path, err)
	}

	if mode == modeUpdate && !found {
		return nil, fmt.Errorf("%s %w", root, errNotAdopted)
	}

	selector := a.selector()

	named := make([]selection.Named, 0, len(names))

	for _, name := range names {
		entry, parseErr := selector.Parse(name)
		if parseErr != nil {
			return nil, parseErr
		}

		named = append(named, entry)
	}

	if len(named) == 0 && !found {
		detected, detectErr := selector.Detect(ctx, root)
		if detectErr != nil {
			return nil, detectErr
		}

		for _, lang := range detected {
			named = append(named, selection.Named{Language: lang, Kind: "", Platform: ""})
		}
	}

	selected, err := selector.Select(cfg, named, flags)
	if err != nil {
		return nil, err
	}

	if len(selected) == 0 {
		dir := ""
		if mode == modeInit {
			dir = "<dir> "
		}

		return nil, fmt.Errorf("found no language in %s. Name one, such as `bedrock %s %sgo`", root, mode, dir)
	}

	if missing := selection.MissingTools(selected, a.Runner.LookPath); len(missing) > 0 {
		return nil, fmt.Errorf("install these, then run bedrock again:\n  %s", strings.Join(missing, "\n  "))
	}

	return selected, nil
}

// planFor plans selected into root. A repository that runs bedrock from its
// own source tree keeps that pin; any other gets this release.
func (a *App) planFor(ctx context.Context, root string, selected []plan.Selected, init bool) (plan.Plan, error) {
	cfg, _, err := config.Read(root)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("read `%s`: %w", config.Path, err)
	}

	version := a.Version
	if cfg.Version == config.SourceVersion {
		version = config.SourceVersion
	}

	dropped, err := a.selector().Dropped(ctx, root, selected)
	if err != nil {
		return plan.Plan{}, err
	}

	built, err := a.engine().Build(ctx, root, selected, plan.Options{Version: version, Dropped: dropped, Init: init})
	if err != nil {
		return plan.Plan{}, fmt.Errorf("plan: %w", err)
	}

	return built, nil
}

func (a *App) printPlan(root string, built plan.Plan) {
	a.print("Plan for " + root + ":")

	for _, s := range built.Steps {
		a.print("  " + step.Describe(root, s))
	}

	for _, note := range built.Notes {
		a.print("  note    " + note)
	}

	if len(built.Steps) == 0 {
		a.print("  nothing to change")
	}
}

// checkClean refuses to overwrite a file whose current content git does not hold.
func (a *App) checkClean(ctx context.Context, root string, built plan.Plan) error {
	var touched []string

	for _, s := range built.Steps {
		if fsutil.Exists(filepath.Join(root, filepath.FromSlash(s.Target()))) {
			touched = append(touched, s.Target())
		}
	}

	dirty, err := git.Git{Runner: a.Runner, Root: root}.Uncommitted(ctx, touched)
	if err != nil {
		return fmt.Errorf("check for uncommitted work: %w", err)
	}

	if len(dirty) > 0 {
		return fmt.Errorf(
			"commit or stash these first, so git can show and undo what bedrock changes (or pass `--allow-dirty`):\n  %s",
			strings.Join(dirty, "\n  "),
		)
	}

	return nil
}

// toRun returns the languages whose after-run goes with this apply: all of
// them when there are steps, else those a skipped or failed run left work for.
func (a *App) toRun(
	ctx context.Context,
	root string,
	selected []plan.Selected,
	built plan.Plan,
	init bool,
) ([]plan.Selected, error) {
	var runs []plan.Selected

	for _, entry := range selected {
		if _, ok := entry.Language.(language.PostApplier); !ok {
			continue
		}

		if len(built.Steps) > 0 {
			runs = append(runs, entry)

			continue
		}

		pending, ok := entry.Language.(language.PendingRunner)
		if !ok {
			continue
		}

		left, err := pending.RunPending(ctx, a.engine().Context(root, selected, entry, init))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Language.Spec().ID, err)
		}

		if left {
			runs = append(runs, entry)
		}
	}

	return runs, nil
}

// printRuns lists each after-run, which `--no-run` turns into a reminder of what it skipped.
func (a *App) printRuns(runs []plan.Selected, skipped bool) {
	for _, entry := range runs {
		applier, ok := entry.Language.(language.PostApplier)
		if !ok {
			continue
		}

		what := applier.Runs()
		id := entry.Language.Spec().ID

		if skipped {
			a.print(fmt.Sprintf("  skip    %s: %s (`--no-run`)", id, what))
		} else {
			a.print(fmt.Sprintf("  run     %s: %s", id, what))
		}
	}
}

// runAfterApply runs each after-run in block order, since one, such as a
// workspace install, may read what an earlier one wrote.
func (a *App) runAfterApply(
	ctx context.Context,
	root string,
	selected, runs []plan.Selected,
	init bool,
) error {
	for _, entry := range runs {
		applier, _ := entry.Language.(language.PostApplier)
		if err := applier.AfterApply(ctx, a.engine().Context(root, selected, entry, init)); err != nil {
			return fmt.Errorf("%s: %w", entry.Language.Spec().ID, err)
		}
	}

	return nil
}
