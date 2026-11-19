package swift

import (
	"context"
	"path/filepath"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

const manifestFile = "Package.swift"

// xcodeSteps wires each Xcode project to `Config/Bedrock.xcconfig`.
func xcodeSteps(root string, projects []string) ([]step.Step, error) {
	var steps []step.Step

	for _, project := range projects {
		file := project + "/project.pbxproj"

		text, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return nil, err
		}

		if next, ok := wired(text); found && ok {
			steps = append(steps, step.Write{Path: file, Content: []byte(next), Executable: false})
		}
	}

	return steps, nil
}

// writtenPaths returns the paths of the write steps.
func writtenPaths(steps []step.Step) []string {
	var paths []string

	for _, planned := range steps {
		if write, ok := planned.(step.Write); ok {
			paths = append(paths, write.Path)
		}
	}

	return paths
}

// parts are the contributions that one plan joins.
type parts struct {
	optional, xcode, kit []step.Step
	legacy, conversion   language.Contribution
	hint                 []string
}

func gather(ctx context.Context, c *language.Context, projects []string, hasManifest bool) (parts, error) {
	var (
		p   parts
		err error
	)

	cxx := c.Has("cpp")
	hasTests := fsutil.Exists(filepath.Join(c.Root, "Tests")) || c.Init

	chosen, err := kindsOf(c)
	if err != nil {
		return p, err
	}

	hasXcode := len(projects) > 0 || (c.Init && chosen["app"])

	if p.optional, err = optionalSteps(c, cxx, hasXcode, hasTests); err != nil {
		return p, err
	}

	if p.xcode, err = xcodeSteps(c.Root, projects); err != nil {
		return p, err
	}

	if p.legacy, err = legacyPlan(ctx, c); err != nil {
		return p, err
	}

	if p.conversion, err = conversionPlan(ctx, c); err != nil {
		return p, err
	}

	if p.hint, err = cxxNote(ctx, c, cxx); err != nil {
		return p, err
	}

	if c.Init {
		managed, blockErr := manifestBlocks(c, cxx)
		if blockErr != nil {
			return p, blockErr
		}

		p.kit, err = kitSteps(c, cxx, managed, hasManifest)
	}

	return p, err
}

// Plan adds the Swift part of an adopt: the strict manifest, Xcode wiring,
// optional configs, XCTest conversion, and legacy cleanup.
func (Swift) Plan(ctx context.Context, c *language.Context) (language.Contribution, error) {
	manifest, hasManifest, err := fsutil.ReadText(filepath.Join(c.Root, manifestFile))
	if err != nil {
		return language.Contribution{}, err
	}

	projects := xcodeProjects(c.Root)

	p, err := gather(ctx, c, projects, hasManifest)
	if err != nil {
		return language.Contribution{}, err
	}

	notes := append(append(p.legacy.Notes, p.conversion.Notes...), p.hint...)
	steps := append(append(append(p.optional, p.xcode...), p.legacy.Steps...), p.conversion.Steps...)
	files := append(append(writtenPaths(p.xcode), p.conversion.ProjectFiles...), writtenPaths(p.kit)...)

	switch {
	case hasManifest:
		managed, blockErr := manifestBlocks(c, c.Has("cpp"))
		if blockErr != nil {
			return language.Contribution{}, blockErr
		}

		steps = append(steps, step.Write{
			Path: manifestFile, Content: []byte(adoptedManifest(manifest, managed)), Executable: false,
		})
		files = append(files, manifestFile)
	case !c.Init && len(projects) == 0:
		notes = append(notes, "`Package.swift` and an `.xcodeproj` are both missing; "+
			"run `bedrock init` to start a package.")
	}

	return language.Contribution{Steps: append(steps, p.kit...), Notes: notes, ProjectFiles: files}, nil
}
