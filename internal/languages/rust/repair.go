package rust

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
)

// gateFile writes `#![cfg(test)]` into an integration test that lacks it.
func gateFile(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content, needed := gated(string(data))
	if !needed {
		return nil
	}

	if saveErr := (fileWrite{path: file, text: content}).save(); saveErr != nil {
		return fmt.Errorf("write: %w", saveErr)
	}

	return nil
}

// repairFile repairs one file and returns what it could not move or gate.
// A test-only file may hold test modules, so only the others move.
func repairFile(root, file string, testOnly, integration bool) []string {
	target := filepath.Join(root, filepath.FromSlash(file))

	// Moving an earlier file's tests can remove a later one in the list.
	if !fsutil.Exists(target) {
		return nil
	}

	var problems []string
	if !testOnly {
		problems = moveOut(target)
	}

	if integration {
		if err := gateFile(target); err != nil {
			problems = append(problems, "`"+target+"`: cannot gate: "+err.Error())
		}
	}

	for index, problem := range problems {
		problems[index] = strings.ReplaceAll(problem, root+"/", "")
	}

	return problems
}

// repairRust moves the test modules of files, relative to root, out to their
// own files and gates each crate's integration tests in `tests/`, one file at a
// time. It returns what it could not move or gate.
func repairRust(ctx context.Context, runner execx.Runner, root string, files []string) ([]string, error) {
	testOnly, err := testOnlyFiles(root, files)
	if err != nil {
		return nil, err
	}

	found, err := manifests(ctx, runner, root)
	if err != nil {
		return nil, err
	}

	crates := map[string]bool{}
	for _, entry := range found {
		crates[entry.dir] = true
	}

	var unrepaired []string

	for _, file := range slices.Sorted(slices.Values(files)) {
		// `src/tests/x.rs` under a `tests` module is no integration test.
		integration := isIntegrationTest(file) && crates[path.Dir(path.Dir(file))]
		unrepaired = append(unrepaired, repairFile(root, file, testOnly[file], integration)...)
	}

	return unrepaired, nil
}
