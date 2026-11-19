package swift

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/swift/xctest"
	"github.com/xsyetopz/bedrock/internal/step"
)

// xctestFiles lists the Swift files outside `*UITests` directories that
// import XCTest.
func xctestFiles(ctx context.Context, c *language.Context) ([]string, error) {
	listed, err := fsutil.ListFiles(ctx, c.Runner, c.Root, func(file string) bool {
		return strings.HasSuffix(file, ".swift") && !isUITest(file)
	})
	if err != nil {
		return nil, err
	}

	var paths []string

	for _, file := range listed {
		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
		if readErr != nil {
			return nil, readErr
		}

		if importsXCTest(text) {
			paths = append(paths, file)
		}
	}

	return paths, nil
}

// conversionPlan returns steps that convert each XCTest unit-test file to
// Swift Testing, with what is left for a person.
func conversionPlan(ctx context.Context, c *language.Context) (language.Contribution, error) {
	paths, err := xctestFiles(ctx, c)
	if err != nil {
		return language.Contribution{}, err
	}

	var contribution language.Contribution

	for _, file := range paths {
		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
		if readErr != nil {
			return language.Contribution{}, readErr
		}

		converted, ok := xctest.Convert(text)
		if !ok {
			continue
		}

		contribution.Steps = append(contribution.Steps,
			step.Write{Path: file, Content: []byte(converted.Text), Executable: false})
		contribution.ProjectFiles = append(contribution.ProjectFiles, file)

		for _, note := range converted.Notes {
			contribution.Notes = append(contribution.Notes, "`"+file+"`: "+note)
		}
	}

	return contribution, nil
}
