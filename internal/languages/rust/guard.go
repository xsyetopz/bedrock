package rust

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// misplaced returns one finding per test module that keeps its body in the
// file or sets its file with `#[path]`.
func misplaced(root, file, source string) []string {
	destination := func(name string) string {
		return relativeTo(root, testsFile(filepath.Join(root, filepath.FromSlash(file)), name))
	}

	var findings []string

	for _, found := range inlineTests(source) {
		findings = append(findings, fmt.Sprintf(
			"%s:%d: mod `%s` keeps its tests in this file; move them to `%s` and declare `#[cfg(test)] mod %s;` "+
				"(`just lint-fix` does it)",
			file, found.line, found.name, destination(found.name), found.name))
	}

	for _, found := range pathTests(source) {
		findings = append(findings, fmt.Sprintf(
			"%s:%d: mod `%s` sets its file with `#[path]`; drop the attribute and keep the tests in `%s` "+
				"(`just lint-fix` does it)",
			file, found.line, found.name, destination(found.name)))
	}

	return findings
}

// checkFile returns the findings for one source file; a test-only file may hold
// test modules.
func checkFile(root, file string, testOnly bool) ([]string, error) {
	source, _, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return nil, err
	}

	var findings []string
	if !testOnly {
		findings = misplaced(root, file, source)
	}

	for _, problem := range suppressionProblems(source) {
		findings = append(findings, fmt.Sprintf("%s:%d: %s", file, problem.line, problem.message))
	}

	return findings, nil
}

// siteLocation matches the `file:line` a finding is about, with or without
// backticks around it, since repair messages quote it and checks do not.
func siteLocation() *regexp.Regexp { return regexp.MustCompile("^`?(.*?:\\d+)`?:") }

// site returns the `file:line` a finding is about, or the whole finding when it
// names no line.
func site(finding string) string {
	if groups := siteLocation().FindStringSubmatch(finding); groups != nil {
		return groups[1]
	}

	return finding
}

// guardRust reports what Clippy cannot see: unit tests that sit in the file
// they test or behind `#[path]`, lint suppressions that skip
// `#[expect(..., reason)]`, and files that differ from what adopt writes, such
// as a member that skips the workspace lints, `foo/mod.rs`, or an integration
// test without `#![cfg(test)]`. With fix, it moves test modules to their own
// files and gates integration tests first.
func guardRust(ctx context.Context, c *language.Context, fix bool) ([]string, error) {
	var unrepaired []string

	if fix {
		listed, err := rustFiles(ctx, c.Runner, c.Root)
		if err != nil {
			return nil, err
		}

		if unrepaired, err = repairRust(ctx, c.Runner, c.Root, listed); err != nil {
			return nil, err
		}
	}

	current, err := rustFiles(ctx, c.Runner, c.Root)
	if err != nil {
		return nil, err
	}

	testOnly, err := testOnlyFiles(c.Root, current)
	if err != nil {
		return nil, err
	}

	explained := map[string]bool{}
	for _, finding := range unrepaired {
		explained[site(finding)] = true
	}

	findings := unrepaired

	for _, file := range current {
		found, checkErr := checkFile(c.Root, file, testOnly[file])
		if checkErr != nil {
			return nil, checkErr
		}

		for _, finding := range found {
			if !explained[site(finding)] {
				findings = append(findings, finding)
			}
		}
	}

	drift, err := layout(ctx, c)
	if err != nil {
		return nil, err
	}

	return append(findings, drift...), nil
}

// renamedTo returns the `foo.rs` a `foo/mod.rs` becomes when the plan writes
// it, and false otherwise.
func renamedTo(plannedWrites map[string]bool, file string) (string, bool) {
	if !strings.HasSuffix(file, "/mod.rs") {
		return "", false
	}

	target := strings.TrimSuffix(file, "/mod.rs") + ".rs"

	return target, plannedWrites[target]
}
