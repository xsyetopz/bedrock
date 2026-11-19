package plan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// leftovers is what languages a repository no longer uses left behind.
type leftovers struct {
	// Blocks are the dropped languages that still have a shared-file block.
	Blocks []language.Language
	// Files are the dropped languages' managed files the repository still has.
	Files []string
}

func (e *Engine) leftovers(root string, dropped []language.Language, vars template.Variables) (leftovers, error) {
	var paths []string

	for _, lang := range dropped {
		// Paths only: a dropped language's templates need variables this run does not compute.
		written, err := template.NewDir(e.Templates, lang.Spec().ID+"/files").Paths()
		if err != nil {
			return leftovers{}, fmt.Errorf("list %s files: %w", lang.Spec().ID, err)
		}

		for _, path := range written {
			rendered, renderErr := template.RenderPath(path, vars)
			if renderErr != nil {
				return leftovers{}, renderErr
			}

			if !slices.Contains(paths, rendered) {
				paths = append(paths, rendered)
			}
		}
	}

	blocks, err := WithBlocks(root, dropped)
	if err != nil {
		return leftovers{}, err
	}

	return leftovers{Blocks: blocks, Files: fsutil.Existing(root, paths)}, nil
}

// `notes` tells what removing the leftovers changes. A leftover file the base
// or a kept language still writes keeps its write step in steps, so the note
// names only the files the plan removes.
func (l leftovers) notes(steps []step.Step) []string {
	const reason = "neither `.editorconfig` nor a marker file at the root names the language"

	removed := map[string]bool{}

	for _, s := range steps {
		if remove, ok := s.(step.Remove); ok {
			removed[remove.Path] = true
		}
	}

	var notes []string

	if len(l.Blocks) > 0 {
		ids := make([]string, 0, len(l.Blocks))
		for _, lang := range l.Blocks {
			ids = append(ids, lang.Spec().ID)
		}

		notes = append(
			notes,
			"Removed the blocks of `"+strings.Join(ids, "`, `")+"` from the shared files: "+reason+".",
		)
	}

	files := slices.DeleteFunc(slices.Clone(l.Files), func(path string) bool { return !removed[path] })
	if len(files) > 0 {
		notes = append(notes, "Removed `"+strings.Join(files, "`, `")+"`: "+reason+". git still has them.")
	}

	return notes
}
