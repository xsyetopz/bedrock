package rust

import (
	"context"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// manifest is one `Cargo.toml` of the repository.
type manifest struct {
	// path is relative to the repository root.
	path string
	// dir is the directory of the manifest, "." at the root.
	dir  string
	text string
	// hasPackage and hasWorkspace say which tables the manifest has.
	hasPackage   bool
	hasWorkspace bool
}

// move renames a `foo/mod.rs` to the `foo.rs` beside its directory.
type move struct {
	from string
	to   string
	// content is the new text when the rename changes what its relative paths
	// mean, empty otherwise.
	content string
	// rewritten is set when content replaces the file's text.
	rewritten bool
}

// rustFiles returns the Rust files git tracks or would track.
func rustFiles(ctx context.Context, runner execx.Runner, root string) ([]string, error) {
	return fsutil.ListFiles(ctx, runner, root, func(file string) bool { return strings.HasSuffix(file, ".rs") })
}

func listNamed(ctx context.Context, runner execx.Runner, root, name string) ([]string, error) {
	return fsutil.ListFiles(ctx, runner, root, func(file string) bool { return path.Base(file) == name })
}

// manifests returns the manifests the repository's own Cargo workspace
// covers. A directory with its own `[workspace]` below the root is another
// project, so it and everything under it stay out.
func manifests(ctx context.Context, runner execx.Runner, root string) ([]manifest, error) {
	paths, err := listNamed(ctx, runner, root, "Cargo.toml")
	if err != nil {
		return nil, err
	}

	found := make([]manifest, 0, len(paths))

	for _, file := range paths {
		text, _, readErr := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(file)))
		if readErr != nil {
			return nil, readErr
		}

		doc, _ := parseToml(text)
		found = append(found, manifest{
			path:         file,
			dir:          path.Dir(file),
			text:         text,
			hasPackage:   isTable(at(doc, "package")),
			hasWorkspace: isTable(at(doc, "workspace")),
		})
	}

	var separate []string

	for _, entry := range found {
		if entry.hasWorkspace && entry.dir != "." {
			separate = append(separate, entry.dir)
		}
	}

	return slices.DeleteFunc(found, func(entry manifest) bool {
		return slices.ContainsFunc(separate, func(dir string) bool {
			return entry.dir == dir || strings.HasPrefix(entry.dir, dir+"/")
		})
	}), nil
}

// moduleMoves finds every `foo/mod.rs` under a `src/` directory whose
// `foo.rs` does not exist yet. Moving the file up a directory changes what its
// relative `#[path]` and `include!` paths reach, so those gain a `foo/`
// prefix; a file whose paths cannot be rewritten safely stays where it is, and
// kept lists it.
func moduleMoves(
	ctx context.Context,
	runner execx.Runner,
	root string,
	found []manifest,
) ([]move, []string, error) {
	sources := make([]string, 0, len(found))

	for _, entry := range found {
		if entry.dir == "." {
			sources = append(sources, "src/")
		} else {
			sources = append(sources, entry.dir+"/src/")
		}
	}

	var (
		moves []move
		kept  []string
	)

	listed, err := listNamed(ctx, runner, root, "mod.rs")
	if err != nil {
		return nil, nil, err
	}

	for _, from := range listed {
		if !slices.ContainsFunc(sources, func(source string) bool {
			return strings.HasPrefix(from, source) && from != source+"mod.rs"
		}) {
			continue
		}

		to := strings.TrimSuffix(from, "/mod.rs") + ".rs"
		if fsutil.Exists(filepath.Join(root, filepath.FromSlash(to))) {
			continue
		}

		text, _, readErr := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(from)))
		if readErr != nil {
			return nil, nil, readErr
		}

		prefix := path.Base(path.Dir(from)) + "/"

		prefixed, ok := prefixPaths(text, pathPrefixes{include: prefix, path: prefix})

		switch {
		case !ok:
			kept = append(kept, from)
		case prefixed == text:
			moves = append(moves, move{from: from, to: to, content: "", rewritten: false})
		default:
			moves = append(moves, move{from: from, to: to, content: prefixed, rewritten: true})
		}
	}

	return moves, kept, nil
}
