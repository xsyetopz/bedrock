package swift

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/swift/xctest"
	"github.com/xsyetopz/bedrock/internal/step"
)

// Subcommands adds `bedrock swift convert-tests`.
func (Swift) Subcommands() []language.Subcommand {
	return []language.Subcommand{{
		Name:    "convert-tests",
		Args:    "[<path>...]",
		Summary: "Convert XCTest unit tests to Swift Testing.",
		Run:     convertTests,
	}}
}

// convertTests rewrites XCTest unit tests to Swift Testing in place, the
// given files or every one outside `*UITests` directories, and prints what
// is left for a person.
func convertTests(ctx context.Context, c *language.Context, args []string) (int, error) {
	paths := args

	if len(paths) == 0 {
		listed, err := xctestFiles(ctx, c)
		if err != nil {
			return 1, err
		}

		paths = listed
	}

	status := 0

	for _, file := range paths {
		failed, err := convertOne(c, file)
		if err != nil {
			return 1, err
		}

		if failed {
			status = 1
		}
	}

	return status, nil
}

// convertOne converts one file and reports whether it was missing.
func convertOne(c *language.Context, file string) (bool, error) {
	text, found, err := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
	if err != nil {
		return false, err
	}

	if !found {
		_, err = fmt.Fprintf(c.Stderr, "`%s`: no such file\n", file)

		return true, err
	}

	converted, ok := xctest.Convert(text)
	if !ok {
		_, err = fmt.Fprintf(c.Stdout, "`%s`: no XCTest import; left as is\n", file)

		return false, err
	}

	err = step.Apply(c.Root, []step.Step{step.Write{Path: file, Content: []byte(converted.Text), Executable: false}})
	if err != nil {
		return false, err
	}

	if _, err = fmt.Fprintf(c.Stdout, "`%s`: converted\n", file); err != nil {
		return false, err
	}

	for _, note := range converted.Notes {
		if _, err = fmt.Fprintf(c.Stderr, "`%s`: %s\n", file, note); err != nil {
			return false, err
		}
	}

	return false, nil
}
