package haxe

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

const workflow = ".github/workflows/haxe.yml"

// Plan leaves notes for a person; the templates carry every file.
func (Haxe) Plan(ctx context.Context, c *language.Context) (language.Contribution, error) {
	notes, err := workflowNote(c.Root)
	if err != nil {
		return language.Contribution{}, err
	}

	builds, err := hxmlBuilds(ctx, c.Runner, c.Root)
	if err != nil {
		return language.Contribution{}, err
	}

	for _, build := range builds {
		if isTestHxml(build.path) && !build.runs {
			notes = append(notes, "`"+build.path+"`: names no `--interp`, `--run`, or `--cmd`, so "+
				"`just haxe::test` does not run it; a `project-test` recipe in `project.just` runs it on its target.")
		}
	}

	return language.Contribution{Notes: notes}, nil
}

// workflowNote reports a Haxe workflow bedrock did not write, since its
// steps, such as checking out sibling repositories or running the project's
// own test recipe, are the project's and a person moves them.
func workflowNote(root string) ([]string, error) {
	text, found, err := fsutil.ReadText(filepath.Join(root, filepath.FromSlash(workflow)))
	if err != nil {
		return nil, err
	}

	if !found || strings.HasPrefix(text, "# Managed by bedrock.") {
		return nil, nil
	}

	return []string{"`" + workflow + "`: replaced with bedrock's; git has yours. Move the steps it still needs " +
		"into `.github/actions/project-setup/action.yml` and the recipes it ran into `project.just`."}, nil
}
