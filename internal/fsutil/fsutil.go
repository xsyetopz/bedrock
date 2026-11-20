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

// ListFiles returns the slash-separated paths of the regular files under
// root that match keeps. Git lists them, so the repository's own ignore
// rules decide what a check skips: each language's gitignore block covers
// its toolchain's output, such as target/ or node_modules, and a nested
// repository, such as an agent's worktree, is not listed at all. Neither is
// a generated file; see NotGenerated.
func ListFiles(
	ctx context.Context,
	runner execx.Runner,
	root string,
	keep func(path string) bool,
) ([]string, error) {
	listed, err := gitListed(ctx, runner, root)
	if err != nil {
		return nil, fmt.Errorf("list the files git sees under `%s`: %w", root, err)
	}

	var files []string

	for _, path := range listed {
		if path == "" || !keep(path) {
			continue
		}
		// The index still lists a file deleted since the last commit, and a
		// nested repository is listed as its directory.
		if IsRegularFile(filepath.Join(root, filepath.FromSlash(path))) {
			files = append(files, path)
		}
	}

	slices.Sort(files)

	return slices.Compact(files), nil
}

// NotGenerated returns the git pathspecs that leave out each file
// .gitattributes marks linguist-generated, the attribute GitHub also reads, so
// no check spends time on code a tool rewrites. Git matches the bare attribute
// and `=true` separately. The base justfile's `files` recipe lists the same.
func NotGenerated() []string {
	return []string{":(exclude,attr:linguist-generated)", ":(exclude,attr:linguist-generated=true)"}
}

func gitListed(ctx context.Context, runner execx.Runner, root string) ([]string, error) {
	out, err := execx.Output(ctx, runner, execx.Command{
		Dir:  root,
		Name: "git",
		Args: append(
			[]string{"ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "."},
			NotGenerated()...),
	})
	if err != nil {
		return nil, err
	}

	return strings.Split(out, "\x00"), nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
