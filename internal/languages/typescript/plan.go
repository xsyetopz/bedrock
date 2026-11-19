package typescript

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// bunVersion is the Bun release bedrock pins; a test keeps it equal to the
// repository's own `.bun-version`.
const bunVersion = "1.4.5"

// Variables fill the Bun version into the templates.
func (Typescript) Variables(_ context.Context, _ *language.Context) (template.Variables, error) {
	return bunVariables(), nil
}

func bunVariables() template.Variables {
	return template.Variables{"bunVersion": bunVersion, "packageManager": "bun@" + bunVersion}
}

// planManifest migrates the repo's package.json onto the template and the
// bun toolchain.
func planManifest(c *language.Context) (part, error) {
	text, _, err := c.Dir("typescript", "manifest").Read(namePackageJSON, bunVariables())
	if err != nil {
		return part{}, fmt.Errorf("read the manifest template: %w", err)
	}

	shipped, err := parseObject(text)
	if err != nil {
		return part{}, fmt.Errorf("parse the manifest template: %w", err)
	}

	name := filepath.Base(c.Root)

	fields, err := kindsFields(c, name)
	if err != nil {
		return part{}, err
	}

	current, err := readJSONC(filepath.Join(c.Root, namePackageJSON))
	if err != nil {
		return part{}, err
	}

	if current == nil {
		current = newObject()
		current.set("name", name)
		current.set("version", "0.0.0")
		current.set("private", true)
		current.set("type", "module")
	}

	pnpm := newObject()

	pnpmText, found, err := fsutil.ReadText(filepath.Join(c.Root, "pnpm-workspace.yaml"))
	if err != nil {
		return part{}, err
	}

	if found {
		if pnpm, err = parsePnpm(pnpmText); err != nil {
			return part{}, err
		}
	}

	migrated, notes := migrateManifest(fields.overlay(current), shipped, pnpm)

	steps, err := jsonWrite(c.Root, namePackageJSON, sortManifest(migrated))

	return part{steps: steps, notes: notes}, err
}

// planTsconfig inlines the template's compiler options into the repo's
// `tsconfig.json`, or writes the template.
func planTsconfig(c *language.Context) (part, error) {
	text, _, err := c.Dir("typescript", "merge").Read(nameTsconfig, nil)
	if err != nil {
		return part{}, fmt.Errorf("read the tsconfig template: %w", err)
	}

	current, err := readJSONC(filepath.Join(c.Root, nameTsconfig))
	if err != nil {
		return part{}, err
	}

	if current == nil {
		return part{
			steps: []step.Step{step.Write{Path: nameTsconfig, Content: []byte(text), Executable: false}},
		}, nil
	}

	base, err := parseObject(text)
	if err != nil {
		return part{}, fmt.Errorf("parse the tsconfig template: %w", err)
	}

	migrated, notes := migrateTsconfig(current, base)
	steps, err := jsonWrite(c.Root, nameTsconfig, migrated)

	return part{steps: steps, notes: notes}, err
}

// planBunfig writes the template's `bunfig.toml` plus the settings the
// repo's copy adds.
func planBunfig(c *language.Context) (part, error) {
	text, _, err := c.Dir("typescript", "merge").Read(nameBunfig, bunVariables())
	if err != nil {
		return part{}, fmt.Errorf("read the bunfig template: %w", err)
	}

	var current *string

	if own, found, readErr := fsutil.ReadText(filepath.Join(c.Root, nameBunfig)); readErr != nil {
		return part{}, readErr
	} else if found {
		current = &own
	}

	merged, notes, err := mergeBunfig(text, current)
	if err != nil {
		return part{}, err
	}

	return part{
		steps: []step.Step{step.Write{Path: nameBunfig, Content: []byte(merged), Executable: false}},
		notes: notes,
	}, nil
}

// projectFiles stay the project's own after the first write, so drift checks
// skip them. The lint configs are not among them: adopt rewrites each from
// the template plus the repo's own ignore entries, so a changed rule is
// drift.
func projectFiles() []string { return []string{nameBunfig, namePackageJSON, nameTsconfig} }

// Plan plans the TypeScript part of an adopt: the manifest, configs, removed
// tools, and test files.
func (Typescript) Plan(ctx context.Context, c *language.Context) (language.Contribution, error) {
	var parts []part

	for _, plan := range []func(*language.Context) (part, error){
		planManifest, planTsconfig, planBunfig, planIgnoreLists,
	} {
		next, err := plan(c)
		if err != nil {
			return language.Contribution{}, err
		}

		parts = append(parts, next)
	}

	var out language.Contribution

	for _, next := range parts {
		out.Steps = append(out.Steps, next.steps...)
		out.Notes = append(out.Notes, next.notes...)
	}

	return finishPlan(ctx, c, out)
}

// finishPlan adds the removals, workspace manifests, test files, and legacy
// mentions that need the file listing.
func finishPlan(ctx context.Context, c *language.Context, out language.Contribution) (language.Contribution, error) {
	removals, err := removalSteps(c.Root)
	if err != nil {
		return out, err
	}

	workspaces, err := workspaceManifestSteps(ctx, c)
	if err != nil {
		return out, err
	}

	tests, err := testSteps(ctx, c)
	if err != nil {
		return out, err
	}

	mentions, err := legacyMentions(ctx, c)
	if err != nil {
		return out, err
	}

	out.Steps = append(append(append(out.Steps, removals...), workspaces...), tests...)
	out.Notes = append(out.Notes, mentions...)
	out.ProjectFiles = projectFiles()

	return out, nil
}
