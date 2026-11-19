// Package sizeguard bounds the size of a repository's sources, which some
// languages' linters leave unmeasured, by the limits .editorconfig records:
// lizard measures each file and function, and the bedrock_*_warning and
// bedrock_*_error pairs that apply to the file say what is too much.
package sizeguard

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/editorconfig"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
)

// Guard runs lizard over each selected language with a size gap, under the
// bounds in .editorconfig. A malformed bound leaves the reports
// untrustworthy, so it alone is reported, as errors.
func Guard(ctx context.Context, engine *plan.Engine, root string, selected []plan.Selected) (Report, error) {
	tree := editorconfig.NewTree(root)

	var combined Report

	for _, entry := range selected {
		spec := entry.Language.Spec()
		if len(spec.Lizard) == 0 {
			continue
		}

		var classify language.TestFileClassifier
		if classifier, ok := entry.Language.(language.TestFileClassifier); ok {
			classify = classifier
		}

		found, err := checkSize(ctx, sizeCheck{
			engine:   engine,
			root:     root,
			selected: selected,
			entry:    entry,
			tree:     tree,
			classify: classify,
		})
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", spec.ID, err)
		}

		combined.Errors = append(combined.Errors, found.Errors...)
		combined.Warnings = append(combined.Warnings, found.Warnings...)
	}

	files, err := tree.Files()
	if err != nil {
		return Report{}, err
	}

	var invalid []string

	for _, file := range files {
		var pairs []editorconfig.Pair
		for _, section := range file.Sections {
			pairs = append(pairs, section.Pairs...)
		}

		invalid = append(invalid, invalidBounds(pairs, file.Path)...)
	}

	if len(invalid) > 0 {
		return Report{Errors: invalid}, nil
	}

	return combined, nil
}

// sizeCheck is what checkSize needs to bound one language.
type sizeCheck struct {
	engine   *plan.Engine
	tree     *editorconfig.Tree
	classify language.TestFileClassifier
	root     string
	selected []plan.Selected
	entry    plan.Selected
}

// checkSize runs lizard over root for the language's lizard names, under the
// bounds the .editorconfig files in the tree set. The language's test files
// take the test file bound.
func checkSize(ctx context.Context, check sizeCheck) (Report, error) {
	languages := check.entry.Language.Spec().Lizard

	lizard, err := findLizard(ctx, check.engine.Runner)
	if err != nil {
		return Report{}, err
	}

	if lizard == nil {
		return Report{Errors: []string{fmt.Sprintf(
			"`lizard` %s cannot run: install `uv` or `pipx` (`just setup` does), so bedrock can bound %s sizes",
			LizardVersion, strings.Join(languages, ", "))}}, nil
	}

	measured, err := measure(ctx, check.engine.Runner, lizard, check.root, languages)
	if err != nil {
		return Report{}, err
	}

	properties := map[string]map[string]string{}

	paths := make([]string, 0, len(measured.files))

	for _, file := range measured.files {
		paths = append(paths, file.path)
	}

	for _, fn := range measured.functions {
		paths = append(paths, fn.path)
	}

	slices.Sort(paths)

	for _, path := range slices.Compact(paths) {
		properties[path], err = check.tree.Properties(path)
		if err != nil {
			return Report{}, err
		}
	}

	tests, err := check.testFiles(ctx, measured.files)
	if err != nil {
		return Report{}, err
	}

	return judge(
		measured,
		func(path string) map[string]string { return properties[path] },
		func(path string) bool { return tests[path] },
	), nil
}

// testFiles returns which of the measured files the language treats as tests.
func (c sizeCheck) testFiles(ctx context.Context, files []fileMeasure) (map[string]bool, error) {
	if c.classify == nil {
		return map[string]bool{}, nil
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.path)
	}

	tests, err := c.classify.TestFiles(ctx, c.engine.Context(c.root, c.selected, c.entry, false), paths)
	if err != nil {
		return nil, fmt.Errorf("find the test files: %w", err)
	}

	return tests, nil
}
