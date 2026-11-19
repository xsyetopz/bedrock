package cli

import (
	"context"
	"fmt"
	"slices"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/git"
	"github.com/xsyetopz/bedrock/internal/language"
)

const languagesUsage = `Usage: bedrock languages [--staged | --unpushed]

Prints the id of each language ` + "`.editorconfig`" + ` lists, one per line. With
` + "`--staged`" + `, only those whose files the commit changes, so the pre-commit hook
skips the toolchains a commit does not touch. With ` + "`--unpushed`" + `, only those whose
files HEAD's commits change that no remote has yet, for the pre-push hook.`

// touches reports whether a changed path concerns lang: a base file every
// language's checks read, its own recipes and CI, or one of its sources.
func touches(lang language.Language, path string) bool {
	id := lang.Spec().ID
	if path == config.Path || path == "justfile" || path == ".just/"+id+".just" ||
		path == ".github/workflows/"+id+".yml" {
		return true
	}

	return slices.ContainsFunc(lang.Spec().Sources, func(glob string) bool {
		matched, err := doublestar.Match(glob, path)

		return err == nil && matched
	})
}

func (a *App) languages(ctx context.Context, args []string) (int, error) {
	flags, help := flagSet("languages")
	staged := flags.Bool("staged", false, "")
	unpushed := flags.Bool("unpushed", false, "")

	if err := parse(flags, args); err != nil {
		return ExitUsage, err
	}

	if *help {
		a.print(languagesUsage)

		return ExitOK, nil
	}

	cfg, _, err := config.Read(a.Dir)
	if err != nil {
		return ExitFailed, fmt.Errorf("read `%s`: %w", config.Path, err)
	}

	selected, err := a.selector().Select(cfg, nil, nil)
	if err != nil {
		return ExitFailed, err
	}

	paths, narrowed, err := a.changedPaths(ctx, *staged, *unpushed)
	if err != nil {
		return ExitFailed, err
	}

	for _, entry := range selected {
		if !narrowed || slices.ContainsFunc(paths, func(path string) bool { return touches(entry.Language, path) }) {
			a.print(entry.Language.Spec().ID)
		}
	}

	return ExitOK, nil
}

// changedPaths returns the paths the flags narrow the languages to; narrowed
// is false when every language counts.
func (a *App) changedPaths(ctx context.Context, staged, unpushed bool) ([]string, bool, error) {
	repo := git.Git{Runner: a.Runner, Root: a.Dir}

	switch {
	case staged:
		paths, err := repo.Staged(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("list staged paths: %w", err)
		}

		return paths, true, nil
	case unpushed:
		paths, ok, err := repo.Unpushed(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("list unpushed paths: %w", err)
		}

		// At a root commit every path is new, so every language counts.
		return paths, ok, nil
	default:
		return nil, false, nil
	}
}
