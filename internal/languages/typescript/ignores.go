package typescript

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// globsOf are the globs a gitignore-style pattern stands for: the path
// itself and what is under it.
func globsOf(pattern string) []string {
	dir := strings.TrimRight(pattern, "/")
	anchored := strings.HasPrefix(dir, "/") || strings.Contains(dir[min(1, len(dir)):], "/")
	base := strings.TrimPrefix(dir, "/")

	glob := base
	if !anchored {
		glob = "**/" + base
	}

	return []string{glob, glob + "/**"}
}

// isIgnored reports whether path, relative to the repo root, matches the
// patterns: gitignore-style entries where a bare name matches at any depth,
// `dir/` matches a directory, and a later `!entry` re-includes.
func isIgnored(path string, patterns []string) bool {
	ignored := false

	for _, entry := range patterns {
		pattern, negated := strings.CutPrefix(entry, "!")
		if pattern == "" {
			continue
		}

		for _, glob := range globsOf(pattern) {
			if ok, err := doublestar.Match(glob, path); err == nil && ok {
				ignored = !negated

				break
			}
		}
	}

	return ignored
}

// readOxlintIgnores reads `ignorePatterns` from the repo's `.oxlintrc.json`.
func readOxlintIgnores(root string) ([]string, error) {
	config, err := readJSONC(filepath.Join(root, ".oxlintrc.json"))
	if err != nil {
		return nil, err
	}

	return asStrings(asObject(config).get("ignorePatterns")), nil
}

// readBunfigTestIgnores reads `[test] pathIgnorePatterns` from `bunfig.toml`:
// the paths `bun test` skips.
func readBunfigTestIgnores(root string) ([]string, error) {
	text, found, err := fsutil.ReadText(filepath.Join(root, nameBunfig))
	if err != nil || !found {
		return nil, err
	}

	// A copy that is not TOML gets replaced by the template, so it names no
	// ignores.
	parsed := tomlOrNil(text)
	if parsed == nil {
		return nil, nil
	}

	patterns := tomlGet(parsed, tomlPath{nameTest, "pathIgnorePatterns"})
	if one, ok := patterns.(string); ok {
		return []string{one}, nil
	}

	var out []string

	for _, item := range asList(patterns) {
		if pattern, ok := item.(string); ok {
			out = append(out, pattern)
		}
	}

	return out, nil
}

// listSources lists the files under root that the glob matches, minus
// git-ignored files, skipped directories, and the ignore entries.
func listSources(ctx context.Context, runner execx.Runner, root, glob string, ignore []string) ([]string, error) {
	files, err := fsutil.ListFiles(ctx, runner, root, func(path string) bool {
		ok, matchErr := doublestar.Match(glob, path)

		return matchErr == nil && ok && !isIgnored(path, ignore)
	})
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	return files, nil
}
