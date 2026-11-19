package typescript

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// formatterConfigs are other formatters' configs: every adopted repo takes
// the bedrock style.
func formatterConfigs() []string {
	return []string{
		".oxfmtrc.json", ".oxfmtrc.jsonc", ".prettierrc", ".prettierrc.cjs", ".prettierrc.js",
		".prettierrc.json", ".prettierrc.json5", ".prettierrc.mjs", ".prettierrc.toml", ".prettierrc.ts",
		".prettierrc.yaml", ".prettierrc.yml", "prettier.config.cjs", "prettier.config.js",
		"prettier.config.mjs", "prettier.config.ts",
	}
}

// replacedConfigs are configs of tools Bun and bedrock replace. Core removes
// the hook managers and Makefile.
func replacedConfigs() []string {
	return []string{
		".eslintignore", ".eslintrc", ".eslintrc.cjs", ".eslintrc.js", ".eslintrc.json", ".eslintrc.yaml",
		".eslintrc.yml", ".lintstagedrc", ".lintstagedrc.json", ".lintstagedrc.js", ".pnp.cjs",
		".pnp.loader.mjs", ".pnpmfile.cjs", ".yarn", ".yarnrc", ".yarnrc.yml", "biome.json", "biome.jsonc",
		"eslint.config.cjs", "eslint.config.js", "eslint.config.mjs", "eslint.config.mts",
		"eslint.config.ts", "jest.config.cjs", "jest.config.js", "jest.config.json", "jest.config.mjs",
		"jest.config.ts", "lint-staged.config.js", "lint-staged.config.mjs", "pnpm-workspace.yaml",
		"vitest.config.js", "vitest.config.mjs", "vitest.config.mts", "vitest.config.ts",
	}
}

// lockfiles are other package managers' lockfiles; `bun install` migrates
// them into `bun.lock` first.
func lockfiles() []string {
	return []string{nameBunLockb, "npm-shrinkwrap.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock"}
}

// oldCI is the CI file of the old template, removed when it runs another
// package manager.
const oldCI = ".github/workflows/ci.yml"

func oldCICommand() *regexp.Regexp {
	return regexp.MustCompile(`\b(?:npm|npx|pnpm|yarn)\b|bun run verify`)
}

// managedWorkflow is written by bedrock, so a mention there is bedrock's own
// to replace.
const managedWorkflow = ".github/workflows/typescript.yml"

// removalSteps removes replaced tool configs and an old-template CI file.
func removalSteps(root string) ([]step.Step, error) {
	paths := fsutil.Existing(root, slices.Concat(formatterConfigs(), replacedConfigs()))

	ci, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(oldCI)))
	if err != nil {
		return nil, err
	}

	if found && oldCICommand().MatchString(ci) {
		paths = append(paths, oldCI)
	}

	steps := make([]step.Step, 0, len(paths))
	for _, path := range paths {
		steps = append(steps, step.Remove{Path: path})
	}

	return steps, nil
}

// lockfilesIn lists which of the other package managers' lockfiles sit in
// root.
func lockfilesIn(root string) []string { return fsutil.Existing(root, lockfiles()) }

// withoutLegacy is a package.json without the `typescript-bedrock`
// dependency, or nil when it had none.
func withoutLegacy(manifest *object) *object {
	next := manifest.clone()
	changed := false

	for _, key := range []string{nameDependencies, nameDevDeps} {
		if asObject(manifest.get(key)).has(legacyPackage) {
			next.set(key, asObject(manifest.get(key)).without(func(name string) bool { return name == legacyPackage }))

			changed = true
		}
	}

	if !changed {
		return nil
	}

	return next
}

// workspaceManifestSteps drops `typescript-bedrock` from the workspace
// packages' manifests; the root one has its own migration.
func workspaceManifestSteps(ctx context.Context, c *language.Context) ([]step.Step, error) {
	paths, err := listSources(ctx, c.Runner, c.Root, "**/package.json", nil)
	if err != nil {
		return nil, err
	}

	var steps []step.Step

	for _, path := range paths {
		if path == namePackageJSON {
			continue
		}

		manifest, readErr := readJSONC(filepath.Join(c.Root, filepath.FromSlash(path)))
		if readErr != nil {
			return nil, readErr
		}

		if next := withoutLegacy(asObject(manifest)); next != nil {
			steps = append(steps, step.Write{Path: path, Content: []byte(toJSON(next)), Executable: false})
		}
	}

	return steps, nil
}

// legacyMentions lists the files bedrock does not write that still name
// `typescript-bedrock`: a person moves what they ran.
func legacyMentions(ctx context.Context, c *language.Context) ([]string, error) {
	paths, err := listSources(ctx, c.Runner, c.Root, ".github/workflows/*.{yml,yaml}", nil)
	if err != nil {
		return nil, err
	}

	paths = append(paths, fsutil.Existing(c.Root, []string{"project.just"})...)

	var notes []string

	for _, path := range paths {
		if path == managedWorkflow {
			continue
		}

		text, _, readErr := fsutil.ReadText(filepath.Join(c.Root, filepath.FromSlash(path)))
		if readErr != nil {
			return nil, readErr
		}

		if strings.Contains(text, legacyPackage) {
			notes = append(notes, "`"+path+"` still mentions `"+legacyPackage+
				"`, which bedrock no longer installs. Run `just typescript::guard` and `just lint` instead.")
		}
	}

	return notes, nil
}
