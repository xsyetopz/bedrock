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

// chosenPlatforms returns the platforms in the comma-separated `platform`
// option, empty when the kinds take their default targets.
func chosenPlatforms(c *language.Context) ([]string, error) {
	platforms, err := config.SplitList(c.Options["platform"])
	if err != nil {
		return nil, fmt.Errorf("`platform` option: %w", err)
	}

	return platforms, nil
}

// targetModules maps a kind to the module whose convention plugin builds the
// platforms, which `<module>/gradle.properties` names. A kind left out has no
// target choice.
func targetModules() map[string]string {
	return map[string]string{kindCmp: "shared", kindKmp: "shared", kindKotlinNative: "app"}
}

// platformOnlyModules maps a module to the platform it exists for, so a kind
// leaves it out when the platforms name no such target: cmp's Android app has
// no use without android.
func platformOnlyModules() map[string]string { return map[string]string{"androidApp": kindAndroid} }

// withPlatforms drops the modules whose platform the build lacks and adds the
// property file that tells the kind's convention plugin which targets to
// build. Without platforms the files stay as the kind wrote them, and the
// plugin builds its default targets.
func withPlatforms(c *language.Context, kind string, platforms []string, files []template.Rendered) (
	[]template.Rendered, error,
) {
	module, takes := targetModules()[kind]
	if len(platforms) == 0 || !takes {
		return files, nil
	}

	kept := slices.DeleteFunc(slices.Clone(files), func(file template.Rendered) bool {
		top, _, _ := strings.Cut(file.Path, "/")
		only, restricted := platformOnlyModules()[top]

		return restricted && !slices.Contains(platforms, only)
	})

	content, _, err := c.Dir("jvm", "properties").Read("platforms.properties",
		template.Variables{"platforms": strings.Join(platforms, ",")})
	if err != nil {
		return nil, fmt.Errorf("kind `%s`: %w", kind, err)
	}

	return append(kept, template.Rendered{Path: module + "/gradle.properties", Content: []byte(content)}), nil
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

	platforms, err := chosenPlatforms(c)
	if err != nil {
		return kindPlan{}, err
	}

	var made kindPlan

	for _, kind := range kinds {
		files, filesErr := c.Dir("jvm", "modules/"+kind).Files(vars)
		if filesErr != nil {
			return kindPlan{}, fmt.Errorf("kind `%s`: %w", kind, filesErr)
		}

		files, filesErr = withPlatforms(c, kind, platforms, files)
		if filesErr != nil {
			return kindPlan{}, filesErr
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
