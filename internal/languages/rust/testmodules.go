package rust

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// outermost drops the modules inside another one's body, which move with it.
func outermost(found []module) []module {
	return slices.DeleteFunc(slices.Clone(found), func(inner module) bool {
		return slices.ContainsFunc(found, func(outer module) bool {
			return outer.body != nil && outer.body.from < inner.start && inner.start < outer.body.to
		})
	})
}

// inlineTests returns the `#[cfg(test)] mod name { ... }` modules of source,
// outside any other.
func inlineTests(source string) []module {
	return outermost(slices.DeleteFunc(modules(source), func(found module) bool {
		return !found.testOnly || found.body == nil
	}))
}

// pathTests returns the `#[cfg(test)] #[path = "..."] mod name;` modules of
// source.
func pathTests(source string) []module {
	return slices.DeleteFunc(modules(source), func(found module) bool {
		return !found.testOnly || found.body != nil || found.pathAttr == nil
	})
}

// testsFile returns where the test module name of file belongs: `foo/tests.rs`
// for `foo.rs`, and `tests.rs` beside a `lib.rs`, `main.rs`, or `mod.rs`.
func testsFile(file, name string) string {
	return filepath.Join(moduleDirectory(file), name+".rs")
}

// testOnlyFiles returns the files of files, relative to root, that compile
// only under test: each one a `#[cfg(test)] mod name;` declares, and every file
// below its module directory. Their own test modules already sit outside the
// code they test.
func testOnlyFiles(root string, files []string) (map[string]bool, error) {
	var owners []string

	for _, file := range files {
		path := filepath.Join(root, file)

		source, _, err := fsutil.ReadText(path)
		if err != nil {
			return nil, err
		}

		for _, found := range modules(source) {
			if !found.testOnly || found.body != nil {
				continue
			}

			if child, ok := childFile(path, found); ok {
				owners = append(owners, relativeTo(root, child))
			}
		}
	}

	directories := make([]string, 0, len(owners))
	for _, owner := range owners {
		directories = append(directories, relativeTo(root, moduleDirectory(filepath.Join(root, owner)))+"/")
	}

	only := map[string]bool{}

	for _, file := range files {
		if slices.Contains(owners, file) || slices.ContainsFunc(directories, func(dir string) bool {
			return strings.HasPrefix(file, dir)
		}) {
			only[file] = true
		}
	}

	return only, nil
}

// relativeTo returns path relative to root, slash-separated.
func relativeTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}
