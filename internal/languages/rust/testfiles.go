package rust

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// inIntegrationTests reports whether file sits under a `tests` directory of a
// crate. `src/tests/x.rs` sits under a crate's `src`, and a `tests` module
// there is no integration test, so only a crate's own `tests` directory counts.
func inIntegrationTests(file string, crates map[string]bool) bool {
	parts := strings.Split(file, "/")

	for index, part := range parts[:len(parts)-1] {
		if part != "tests" {
			continue
		}

		crate := "."
		if index > 0 {
			crate = strings.Join(parts[:index], "/")
		}

		if crates[crate] {
			return true
		}
	}

	return false
}

// testFiles returns which of files the toolchain compiles only under test: the
// files a `#[cfg(test)] mod name;` declares, a crate's `tests/` files, and any
// file gated with `#![cfg(test)]`.
func testFiles(ctx context.Context, c *language.Context, files []string) (map[string]bool, error) {
	testOnly, err := testOnlyFiles(c.Root, files)
	if err != nil {
		return nil, err
	}

	found, err := manifests(ctx, c.Runner, c.Root)
	if err != nil {
		return nil, err
	}

	crates := map[string]bool{}
	for _, entry := range found {
		crates[entry.dir] = true
	}

	result := map[string]bool{}

	for _, file := range files {
		if testOnly[file] || inIntegrationTests(file, crates) {
			result[file] = true

			continue
		}

		source, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
		if readErr != nil {
			return nil, readErr
		}

		if hasGate(source) {
			result[file] = true
		}
	}

	return result, nil
}
