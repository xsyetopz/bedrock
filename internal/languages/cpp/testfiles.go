package cpp

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// swiftTestFiles returns the files in a SwiftPM package's `Tests/` directory,
// where the package's test targets live and which SwiftPM builds only for
// `swift test`.
func swiftTestFiles(root string, files []string) []string {
	packages := map[string]bool{}
	isPackage := func(dir string) bool {
		found, ok := packages[dir]
		if !ok {
			found = fsutil.Exists(filepath.Join(root, filepath.FromSlash(dir), "Package.swift"))
			packages[dir] = found
		}

		return found
	}

	var picked []string

	for _, file := range files {
		parts := strings.Split(file, "/")

		for i, part := range parts {
			if part == "Tests" && i < len(parts)-1 && isPackage(strings.Join(parts[:i], "/")) {
				picked = append(picked, file)

				break
			}
		}
	}

	return picked
}

// buildTargets returns the targets of each CMake, Meson, and xmake project.
// CMake and Meson are read from the build directories `bedrock cpp
// compile-commands` configured, since a configure can take minutes; a
// project not configured yet has none. xmake loads its targets without
// configuring.
func buildTargets(ctx context.Context, c *language.Context) ([][]target, error) {
	var builds [][]target

	for _, project := range cmakeProjects(ctx, c) {
		targets, err := cmakeTargets(c.Root, project)
		if err != nil {
			return nil, err
		}

		builds = append(builds, targets)
	}

	for _, project := range mesonProjects(ctx, c) {
		builds = append(builds, mesonTargets(ctx, c, project))
	}

	for _, project := range xmakeProjects(ctx, c) {
		targets, err := xmakeTargets(ctx, c, project)
		if err != nil {
			return nil, err
		}

		builds = append(builds, targets)
	}

	return builds, nil
}

// TestFiles returns the files that only test code compiles: a SwiftPM
// package's `Tests/`, and the sources of the targets each CMake, Meson, or
// xmake build runs as tests, unless a product target of any build compiles
// them too.
func (Cpp) TestFiles(ctx context.Context, c *language.Context, files []string) (map[string]bool, error) {
	builds, err := buildTargets(ctx, c)
	if err != nil {
		return nil, err
	}

	tests, other := map[string]bool{}, map[string]bool{}

	for _, file := range swiftTestFiles(c.Root, files) {
		tests[file] = true
	}

	for _, build := range builds {
		parts := splitSources(build)

		for file := range parts.other {
			other[file] = true
		}

		for file := range parts.test {
			tests[file] = true
		}
	}

	picked := map[string]bool{}

	for _, file := range files {
		if tests[file] && !other[file] {
			picked[file] = true
		}
	}

	return picked, nil
}
