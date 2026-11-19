// Package fsutil reads a repository's files the way every bedrock check
// needs: a missing file is an answer, not an error, while a file that exists
// but cannot be read is an error the caller reports.
package fsutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// ReadText returns the file's text and true, or false when nothing, a
// directory, or a dangling link sits at path.
func ReadText(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return string(data), true, nil
	}

	if errors.Is(err, fs.ErrNotExist) || isDirectory(path) {
		return "", false, nil
	}

	return "", false, fmt.Errorf("read `%s`: %w", path, err)
}

// Exists reports whether anything, a dangling link included, sits at path.
func Exists(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

// IsRegularFile reports whether path is a regular file, not a link.
func IsRegularFile(path string) bool {
	info, err := os.Lstat(path)

	return err == nil && info.Mode().IsRegular()
}

// LinkTarget returns where the symlink at path points, or false when path
// is not a link.
func LinkTarget(path string) (string, bool) {
	target, err := os.Readlink(path)

	return target, err == nil
}

// Existing returns the paths, relative to root, that exist under root.
func Existing(root string, paths []string) []string {
	var found []string

	for _, path := range paths {
		if Exists(filepath.Join(root, filepath.FromSlash(path))) {
			found = append(found, path)
		}
	}

	return found
}

// skipped are directories no check reads: dependencies, build output, agent
// worktrees, and OpenSpec proposals.
func skipped(part string) bool {
	switch part {
	case ".build", ".claude", ".git", ".gradle", "build", "node_modules", "openspec", "target":
		return true
	default:
		return false
	}
}

// ListFiles returns the slash-separated paths of the regular files under
// root that match keeps, without the ones git ignores and those in
// dependency or build directories. Outside a git work tree it walks root.
func ListFiles(
	ctx context.Context,
	runner execx.Runner,
	root string,
	keep func(path string) bool,
) ([]string, error) {
	listed, err := gitListed(ctx, runner, root)
	if err != nil {
		listed, err = walked(root)
		if err != nil {
			return nil, err
		}
	}

	var files []string

	for _, path := range listed {
		if path == "" || !keep(path) || slices.ContainsFunc(strings.Split(path, "/"), skipped) {
			continue
		}
		// The index still lists a file deleted since the last commit.
		if IsRegularFile(filepath.Join(root, filepath.FromSlash(path))) {
			files = append(files, path)
		}
	}

	slices.Sort(files)

	return slices.Compact(files), nil
}

func gitListed(ctx context.Context, runner execx.Runner, root string) ([]string, error) {
	out, err := execx.Output(ctx, runner, execx.Command{
		Dir:  root,
		Name: "git",
		Args: []string{"ls-files", "--cached", "--others", "--exclude-standard", "-z"},
	})
	if err != nil {
		return nil, err
	}

	return strings.Split(out, "\x00"), nil
}

func walked(root string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() && path != root && skipped(entry.Name()) {
			return filepath.SkipDir
		}

		if entry.Type().IsRegular() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return fmt.Errorf("relative path of `%s`: %w", path, relErr)
			}

			paths = append(paths, filepath.ToSlash(rel))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk `%s`: %w", root, err)
	}

	return paths, nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
