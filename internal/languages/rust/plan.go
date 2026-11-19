package rust

import (
	"context"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// replacedFiles are the files whose job a managed file took over.
func replacedFiles() []string {
	return []string{".cargo/config", ".clippy.toml", ".rustfmt.toml", "rust-toolchain"}
}

// toolchainPart returns the toolchain file at the pinned channel; a wasm crate
// adds its target.
func toolchainPart(c *language.Context, wanted []string) (language.Contribution, error) {
	base, _, err := c.Dir("rust", "parts").Read(toolchainPath, template.Variables{})
	if err != nil {
		return language.Contribution{}, err
	}

	current, _, err := fsutil.ReadText(filepath.Join(c.Root, toolchainPath))
	if err != nil {
		return language.Contribution{}, err
	}

	var targets []string
	if slices.Contains(wanted, "wasm") {
		targets = []string{wasmTarget}
	}

	content := toolchainFile(base, current, targets)

	return language.Contribution{
		Steps:        []step.Step{step.Write{Path: toolchainPath, Content: []byte(content), Executable: false}},
		ProjectFiles: []string{toolchainPath},
	}, nil
}

// testsPart returns the integration tests that lack `#![cfg(test)]`, written
// with it.
func testsPart(ctx context.Context, c *language.Context, found []manifest) (language.Contribution, error) {
	dirs := map[string]bool{}
	for _, entry := range found {
		dirs[entry.dir] = true
	}

	listed, err := rustFiles(ctx, c.Runner, c.Root)
	if err != nil {
		return language.Contribution{}, err
	}

	var result language.Contribution

	for _, file := range listed {
		if !isIntegrationTest(file) || !dirs[path.Dir(path.Dir(file))] {
			continue
		}

		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(file)))
		if readErr != nil {
			return language.Contribution{}, readErr
		}

		if content, ok := gated(text); ok {
			result.Steps = append(result.Steps, step.Write{Path: file, Content: []byte(content), Executable: false})
			result.ProjectFiles = append(result.ProjectFiles, file)
		}
	}

	return result, nil
}

// movesPart returns `foo/mod.rs` moved to `foo.rs`. A rename that rewrites
// paths is a write of the new text plus a removal of the old file.
func movesPart(moves []move, kept []string) language.Contribution {
	var result language.Contribution

	for _, entry := range moves {
		if entry.rewritten {
			result.Steps = append(result.Steps,
				step.Write{Path: entry.to, Content: []byte(entry.content), Executable: false},
				step.Remove{Path: entry.from},
			)
		} else {
			result.Steps = append(result.Steps, step.Move{Path: entry.to, From: entry.from})
		}

		result.ProjectFiles = append(result.ProjectFiles, entry.from, entry.to)
	}

	for _, file := range kept {
		result.Notes = append(
			result.Notes,
			"`"+file+"`: not renamed, because a `#[path]` or `include!` in it cannot be rewritten safely; move it by hand.",
		)
	}

	return result
}

// replacedPart removes the files whose job a managed file took over.
func replacedPart(root string) language.Contribution {
	present := fsutil.Existing(root, replacedFiles())
	if len(present) == 0 {
		return language.Contribution{}
	}

	result := language.Contribution{
		Notes: []string{
			"Removed `" + strings.Join(present, "`, `") +
				"`: the toolchain, `clippy.toml`, `rustfmt.toml`, and the `justfile` replace them. " +
				"Their settings are not carried over; git still has them.",
		},
	}

	for _, file := range present {
		result.Steps = append(result.Steps, step.Remove{Path: file})
	}

	return result
}

// initPart returns, at init, the crates for each kind and a README when there
// is none.
func initPart(c *language.Context, wanted []string) (language.Contribution, error) {
	if !c.Init {
		return language.Contribution{}, nil
	}

	steps, notes, err := kindSteps(c, wanted)
	if err != nil {
		return language.Contribution{}, err
	}

	readme, found, err := c.Dir("rust", "parts").Read("README.md", template.Variables{"name": packageName(c.Root)})
	if err != nil {
		return language.Contribution{}, err
	}

	if found && !fsutil.Exists(filepath.Join(c.Root, "README.md")) {
		steps = append(steps, step.Write{Path: "README.md", Content: []byte(readme), Executable: false})
	}

	return language.Contribution{Steps: steps, Notes: notes}, nil
}

// coveredManifests are the manifests the plan edits: all of them, less the
// xtask crate when it goes.
func coveredManifests(all []manifest, dropXtask bool) []manifest {
	if !dropXtask {
		return all
	}

	return slices.DeleteFunc(slices.Clone(all), func(entry manifest) bool {
		return entry.dir == "xtask" || strings.HasPrefix(entry.dir, "xtask/")
	})
}

// planRust plans the Rust part of an adopt: manifests on the workspace lints,
// the pinned toolchain, test files and modules in the layout the guard checks,
// what older rust-bedrock releases left behind, and at init the crates.
func planRust(ctx context.Context, c *language.Context) (language.Contribution, error) {
	var wanted []string

	if c.Init {
		var err error
		if wanted, err = kindsOf(c.Options); err != nil {
			return language.Contribution{}, err
		}
	}

	old, err := legacyPlan(ctx, c.Runner, c.Root)
	if err != nil {
		return language.Contribution{}, err
	}

	all, err := manifests(ctx, c.Runner, c.Root)
	if err != nil {
		return language.Contribution{}, err
	}

	covered := coveredManifests(all, old.dropXtask)

	moves, kept, err := moduleMoves(ctx, c.Runner, c.Root, covered)
	if err != nil {
		return language.Contribution{}, err
	}

	return mergeParts(ctx, c, wanted, covered, old, movesPart(moves, kept))
}

// mergeParts joins the parts in the order their steps apply.
func mergeParts(
	ctx context.Context,
	c *language.Context,
	wanted []string,
	covered []manifest,
	old legacy,
	moved language.Contribution,
) (language.Contribution, error) {
	toolchain, err := toolchainPart(c, wanted)
	if err != nil {
		return language.Contribution{}, err
	}

	manifestsPlan, err := manifestPart(c, covered, old.dropXtask)
	if err != nil {
		return language.Contribution{}, err
	}

	tests, err := testsPart(ctx, c, covered)
	if err != nil {
		return language.Contribution{}, err
	}

	started, err := initPart(c, wanted)
	if err != nil {
		return language.Contribution{}, err
	}

	return mergeContributions(
		toolchain, manifestsPlan, tests, moved, replacedPart(c.Root), started, old.Contribution,
	), nil
}
