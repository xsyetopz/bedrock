package swift

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xsyetopz/bedrock/internal/blocks"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// replacedFiles are files of tools that swift-format, SwiftLint, and SwiftPM
// replaced.
func replacedFiles() []string { return []string{".swiftformat", ".swiftlint.yaml", "Mintfile"} }

// referencingFiles are files the project owns that an old bedrock pointed at
// BuildTools.
func referencingFiles() []string {
	return []string{"project.just", ".gitignore", ".just/*.just", ".github/**/*.{yml,yaml}"}
}

// blockNames are every block name a release of bedrock wrote, in file order.
func blockNames() []string {
	return []string{
		"bedrock", "typescript-bedrock", "rust-bedrock", swiftBlock, cxxBlock,
		"jvm-bedrock", "go-bedrock", "haxe-bedrock", "dotnet-bedrock",
	}
}

func ownedFile() *regexp.Regexp {
	return regexp.MustCompile(`^\.just/(?:swift|cpp)\.just$|^\.github/workflows/swift\.yml$`)
}

// isBedrockBuildTools reports whether the old bedrock wrote BuildTools: its
// package depends on swift-bedrock.
func isBedrockBuildTools(root string) (bool, error) {
	manifest, found, err := fsutil.ReadText(filepath.Join(root, "BuildTools", "Package.swift"))

	return found && strings.Contains(manifest, swiftBlock), err
}

// outsideBlocks returns the text of a file without the blocks bedrock keeps
// up to date.
func outsideBlocks(text string) string {
	for _, name := range blockNames() {
		text = blocks.Remove(text, blocks.Hash, name)
	}

	return text
}

func matches(pattern, file string) bool {
	ok, err := doublestar.Match(pattern, file)

	return err == nil && ok
}

func referenceNotes(ctx context.Context, c *language.Context) ([]string, error) {
	var notes []string

	for _, pattern := range referencingFiles() {
		files, err := fsutil.ListFiles(ctx, c.Runner, c.Root, func(file string) bool {
			return matches(pattern, file) && !ownedFile().MatchString(file)
		})
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			text, found, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
			if readErr != nil {
				return nil, readErr
			}

			if found && strings.Contains(outsideBlocks(text), "BuildTools") {
				notes = append(notes, "`"+file+"`: mentions `BuildTools`, which bedrock no longer uses; "+
					"remove the reference")
			}
		}
	}

	return notes, nil
}

// legacyPlan removes what an old swift-bedrock left behind: the BuildTools
// package it wrote, the files of tools it replaced, and notes the project
// files that still point at it.
func legacyPlan(ctx context.Context, c *language.Context) (language.Contribution, error) {
	var contribution language.Contribution

	for _, file := range fsutil.Existing(c.Root, replacedFiles()) {
		contribution.Steps = append(contribution.Steps, step.Remove{Path: file})
	}

	legacy, err := isBedrockBuildTools(c.Root)
	if err != nil || !legacy {
		return contribution, err
	}

	contribution.Steps = append(contribution.Steps, step.Remove{Path: "BuildTools"})
	contribution.Notes, err = referenceNotes(ctx, c)

	return contribution, err
}
