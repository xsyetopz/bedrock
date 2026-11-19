package swift

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/blocks"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// kinds are the starters `bedrock init` writes; each one includes the
// library.
//
// The names are the presets of the Swift ecosystem: `swift package init
// --type` offers library, executable, tool, build-tool-plugin, command-plugin,
// and macro, and Xcode offers the app.
func kinds() []string {
	return []string{"library", "executable", "tool", "macro", "build-tool-plugin", "command-plugin", "app"}
}

// kindsOf returns the kinds chosen on the command line, the library alone by
// default.
func kindsOf(c *language.Context) (map[string]bool, error) {
	named, err := config.SplitList(c.Options["kind"])
	if err != nil {
		return nil, fmt.Errorf("kind: %w", err)
	}

	chosen := make(map[string]bool, len(named))
	for _, kind := range named {
		chosen[kind] = true
	}

	return chosen, nil
}

// packageType returns the type name of the package a repository directory
// starts, or an error that says why it cannot.
func packageType(directory string) (string, error) {
	name := typeName(directory)
	if !isTypeName(name) {
		return "", fmt.Errorf("`%s` cannot name a Swift package: its type name %q must start with a letter "+
			"and use only ASCII letters and digits; rename the directory", directory, name)
	}

	return name, nil
}

// treeSteps renders a starter tree, leaving out the files the repository
// already has.
func treeSteps(root string, tree template.Dir, vars template.Variables) ([]step.Step, error) {
	files, err := tree.Files(vars)
	if err != nil {
		return nil, fmt.Errorf("render `%s`: %w", tree.Name(), err)
	}

	var steps []step.Step

	for _, file := range files {
		if !fsutil.Exists(filepath.Join(root, filepath.FromSlash(file.Path))) {
			steps = append(steps, step.Write{Path: file.Path, Content: file.Content, Executable: false})
		}
	}

	return steps, nil
}

// kitSteps returns the starter files of the chosen kinds, and a
// `Package.swift` when the repository has none. Files the repository
// already has stay as they are.
//
// The core copies `kinds/<kind>` for `bedrock init`, but the swift templates
// keep their starters under `kit/` so the C++ interop starter can join them;
// this planner reads them.
func kitSteps(c *language.Context, cxx bool, managed []blocks.Block, hasManifest bool) ([]step.Step, error) {
	chosen, err := kindsOf(c)
	if err != nil {
		return nil, err
	}

	name, err := packageType(filepath.Base(c.Root))
	if err != nil {
		return nil, err
	}

	if chosen["executable"] && chosen["tool"] {
		return nil, errors.New("the executable and tool kinds both write the `" + name +
			"CLI` target; choose one, since a tool is an executable that parses arguments")
	}

	vars := template.Variables{"typeName": name, "lowerName": strings.ToLower(name)}

	if chosen["app"] {
		platforms, platformErr := chosenPlatforms(c)
		if platformErr != nil {
			return nil, platformErr
		}

		maps.Copy(vars, appVariables(platforms))
	}

	trees := []template.Dir{c.Dir(languageID, "kit/library")}

	for _, kind := range kinds() {
		if kind != "library" && chosen[kind] {
			trees = append(trees, c.Dir(languageID, "kit/"+kind))
		}
	}

	if cxx {
		trees = append(trees, c.Dir("cpp", "kit/cxx"))
	}

	trees = append(trees, c.Dir(languageID, "kit/new"))

	var steps []step.Step

	for _, tree := range trees {
		planned, treeErr := treeSteps(c.Root, tree, vars)
		if treeErr != nil {
			return nil, treeErr
		}

		steps = append(steps, planned...)
	}

	if !hasManifest {
		steps = append(steps, step.Write{
			Path:       "Package.swift",
			Content:    []byte(adoptedManifest(newManifest(name, chosen, cxx), managed)),
			Executable: false,
		})
	}

	return steps, nil
}
