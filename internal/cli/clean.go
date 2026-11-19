package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/xsyetopz/bedrock/internal/git"
)

// cleanup is what a backup removal deleted and what it left for a person to review.
type cleanup struct {
	removed []string
	kept    []string
}

// backupSkipped are directories that hold dependencies or build output,
// never a backup to review.
func backupSkipped(name string) bool {
	switch name {
	case ".build", ".git", ".gradle", "build", "node_modules", "target":
		return true
	default:
		return false
	}
}

// findBackups returns the *.bak files under root, including those git
// ignores, as sorted slash paths.
func findBackups(root string) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}

			return err
		}

		if entry.IsDir() {
			if path != root && backupSkipped(entry.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if entry.Type().IsRegular() && filepath.Ext(entry.Name()) == ".bak" {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}

			found = append(found, filepath.ToSlash(rel))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find backups under %s: %w", root, err)
	}

	slices.Sort(found)

	return found, nil
}

// removeBackups deletes each backup git can restore, and each other one with force.
func (a *App) removeBackups(ctx context.Context, root string, force bool) (cleanup, error) {
	backups, err := findBackups(root)
	if err != nil {
		return cleanup{}, err
	}

	held := map[string]bool{}

	repo := git.Git{Runner: a.Runner, Root: root}
	if len(backups) > 0 && repo.IsRepository(ctx) {
		held, err = repo.HeldByGit(ctx, backups)
		if err != nil {
			return cleanup{}, fmt.Errorf("check which backups git holds: %w", err)
		}
	}

	var result cleanup

	for _, path := range backups {
		if !force && !held[path] {
			result.kept = append(result.kept, path)

			continue
		}

		removeErr := os.Remove(filepath.Join(root, filepath.FromSlash(path)))
		if removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
			return cleanup{}, fmt.Errorf("delete %s: %w", path, removeErr)
		}

		result.removed = append(result.removed, path)
	}

	return result, nil
}

func (a *App) reportCleanup(result cleanup) {
	for _, path := range result.removed {
		a.print("  delete  " + path)
	}

	for _, path := range result.kept {
		a.print("  keep    " + path + ": git does not hold this content. Review it, then run `bedrock clean --force`.")
	}
}

const cleanUsage = `Usage: bedrock clean [<dir>] [--force]

Deletes the ` + "`*.bak`" + ` files that bedrock releases before 0.2 left beside each
file they replaced. A backup whose content git already holds goes without
asking. One git does not hold is listed, and goes only with ` + "`--force`" + `.`

func (a *App) clean(ctx context.Context, args []string) (int, error) {
	flags, help := flagSet("clean")
	force := flags.Bool("force", false, "")

	if err := parse(flags, args); err != nil {
		return ExitUsage, err
	}

	if *help {
		a.print(cleanUsage)

		return ExitOK, nil
	}

	result, err := a.removeBackups(ctx, a.root(flags.Arg(0)), *force)
	if err != nil {
		return ExitFailed, err
	}

	if len(result.removed) == 0 && len(result.kept) == 0 {
		a.print("No `*.bak` files.")
	}

	a.reportCleanup(result)

	if len(result.kept) > 0 {
		return ExitFailed, nil
	}

	return ExitOK, nil
}
