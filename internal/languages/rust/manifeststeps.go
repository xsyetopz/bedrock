package rust

import (
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// mergeContributions joins the parts of a plan in order.
func mergeContributions(parts ...language.Contribution) language.Contribution {
	var merged language.Contribution

	for _, part := range parts {
		merged.Steps = append(merged.Steps, part.Steps...)
		merged.Notes = append(merged.Notes, part.Notes...)
		merged.ProjectFiles = append(merged.ProjectFiles, part.ProjectFiles...)
	}

	return merged
}

// edited returns `edit(text)` as a write step, or a note when the TOML does not
// parse, so a manifest bedrock cannot read stays as it is.
func edited(path, text string, edit func(string) (string, error)) language.Contribution {
	content, err := edit(text)
	if err != nil {
		return language.Contribution{Notes: []string{"`" + path + "`: " + err.Error() + "; left as it is."}}
	}

	return language.Contribution{
		Steps:        []step.Step{step.Write{Path: path, Content: []byte(content), Executable: false}},
		Notes:        editionNotes(path, text, content),
		ProjectFiles: []string{path},
	}
}

// editionNotes returns one note when the manifest declares an edition older
// than 2024, which the after-apply migration takes to 2024.
func editionNotes(path, before, after string) []string {
	old := slices.DeleteFunc(editionsOf(after), func(edition string) bool { return edition == "2024" })
	if len(old) == 0 {
		return nil
	}

	what := "edition " + strings.Join(old, ", ")
	if len(editionsOf(before)) < len(editionsOf(after)) {
		what = "declares no edition, which Cargo reads as 2015, so adopt names it"
	}

	return []string{
		"`" + path + "`: " + what + "; after applying, `cargo fix --edition` migrates it to 2024 (skipped with `--no-run`).",
	}
}

// manifestPart returns the manifests with the workspace lints, resolver, and
// release profile. At init, a repository without a `Cargo.toml` starts from the
// template's.
func manifestPart(c *language.Context, found []manifest, dropXtask bool) (language.Contribution, error) {
	parts := c.Dir("rust", "parts")

	workspace, _, err := parts.Read("workspace.toml", template.Variables{})
	if err != nil {
		return language.Contribution{}, err
	}

	wanted := readManaged(workspace)

	var (
		text    string
		present bool
	)

	if index := slices.IndexFunc(found, func(entry manifest) bool { return entry.dir == "." }); index >= 0 {
		text, present = found[index].text, true
	} else if c.Init {
		if text, present, err = parts.Read("Cargo.toml", template.Variables{}); err != nil {
			return language.Contribution{}, err
		}
	}

	if !present {
		return language.Contribution{
			Notes: []string{"No `Cargo.toml` at the root; `bedrock init` starts a project."},
		}, nil
	}

	results := []language.Contribution{edited("Cargo.toml", text, func(current string) (string, error) {
		if dropXtask {
			current = withoutXtaskMember(current)
		}

		return rootManifest(current, wanted)
	})}

	for _, entry := range found {
		if entry.hasPackage && entry.dir != "." {
			results = append(results, edited(entry.path, entry.text, memberManifest))
		}
	}

	return mergeContributions(results...), nil
}
