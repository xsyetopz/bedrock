package jvm

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// chosenKinds returns the kinds in the comma-separated `kind` option.
func chosenKinds(c *language.Context) ([]string, error) {
	kinds, err := config.SplitList(c.Options["kind"])
	if err != nil {
		return nil, fmt.Errorf("`kind` option: %w", err)
	}

	return kinds, nil
}

// packageName turns a directory name into a package name: lowercase letters
// and digits, starting with a letter.
func packageName(name string) string {
	cleaned := strings.TrimLeft(nonAlphanumeric().ReplaceAllString(strings.ToLower(name), ""), "0123456789")
	if cleaned == "" {
		return "app"
	}

	return cleaned
}

// kindPlan is the files the chosen kinds add and the module directories to
// include in the settings.
type kindPlan struct {
	steps   []step.Step
	modules []string
}

// kindSteps returns the files of each chosen kind that the repository lacks,
// with `__path__` in directory names filled with the package path, and the
// modules to include. Kinds live under `modules/` because a kind here is a
// module tree, not a starter that core copies.
func kindSteps(c *language.Context, vars template.Variables) (kindPlan, error) {
	kinds, err := chosenKinds(c)
	if err != nil {
		return kindPlan{}, err
	}

	var made kindPlan

	for _, kind := range kinds {
		files, filesErr := c.Dir("jvm", "modules/"+kind).Files(vars)
		if filesErr != nil {
			return kindPlan{}, fmt.Errorf("kind `%s`: %w", kind, filesErr)
		}

		for _, file := range files {
			if fsutil.Exists(filepath.Join(c.Root, filepath.FromSlash(file.Path))) {
				continue
			}

			made.steps = append(made.steps, step.Write{Path: file.Path, Content: file.Content, Executable: false})
		}

		// Modules come from every file a kind has, so a kind whose files all
		// exist still keeps its include.
		for _, file := range files {
			module, _, _ := strings.Cut(file.Path, "/")
			if !slices.Contains(made.modules, module) {
				made.modules = append(made.modules, module)
			}
		}
	}

	return made, nil
}
