package rust

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// maxEditionRounds takes 2015 to 2024 one edition at a time.
const maxEditionRounds = 3

// nextEdition is each edition `cargo fix --edition` migrates code from, with
// the one it migrates to.
func nextEdition() map[string]string {
	return map[string]string{"2015": "2018", "2018": "2021", "2021": "2024"}
}

// raisedEdition returns the manifest with every declared edition older than
// 2024, at a package, workspace package, or target, raised by one step.
func raisedEdition(text string) string {
	pattern := regexp.MustCompile(`(?m)^(\s*edition\s*=\s*)"(2015|2018|2021)"`)
	next := nextEdition()

	return pattern.ReplaceAllStringFunc(text, func(found string) string {
		groups := pattern.FindStringSubmatch(found)

		return groups[1] + `"` + next[groups[2]] + `"`
	})
}

// oldManifests returns the manifests that still declare an edition older than
// 2024.
func oldManifests(ctx context.Context, runner execx.Runner, root string) ([]manifest, error) {
	found, err := manifests(ctx, runner, root)
	if err != nil {
		return nil, err
	}

	return slices.DeleteFunc(found, func(entry manifest) bool { return raisedEdition(entry.text) == entry.text }), nil
}

// editionPending reports whether a manifest still declares an edition older
// than 2024, as a skipped or failed migration leaves it.
func editionPending(ctx context.Context, runner execx.Runner, root string) (bool, error) {
	old, err := oldManifests(ctx, runner, root)

	return len(old) > 0, err
}

// migrateEditions migrates the workspace to edition 2024 one edition at a
// time: `cargo fix --edition` rewrites the code for the next edition, then
// the manifests declare it. Three rounds take 2015 to 2024. The plan has
// already named the edition of each package that declared none, so those move
// as 2015. Each round depends on the code the previous one rewrote, so they run
// in turn.
func migrateEditions(ctx context.Context, runner execx.Runner, root string) error {
	for range maxEditionRounds {
		old, err := oldManifests(ctx, runner, root)
		if err != nil {
			return err
		}

		if len(old) == 0 {
			return nil
		}

		fix, err := runner.Run(ctx, execx.Command{
			Dir:    root,
			Name:   cargoTool,
			Args:   []string{"fix", "--edition", "--workspace", "--all-targets", "--allow-dirty"},
			Stream: true,
		})
		if err != nil {
			return fmt.Errorf("run `cargo fix --edition`: %w", err)
		}

		if !fix.OK() {
			return failedEditionError(old)
		}

		for _, entry := range old {
			target := filepath.Join(root, filepath.FromSlash(entry.path))
			if writeErr := os.WriteFile(target, []byte(raisedEdition(entry.text)), fileMode); writeErr != nil {
				return fmt.Errorf("write `%s`: %w", entry.path, writeErr)
			}
		}
	}

	return nil
}

// failedEditionError names the editions a failed `cargo fix --edition` was
// moving from.
func failedEditionError(old []manifest) error {
	var declared []string

	for _, entry := range old {
		for _, edition := range editionsOf(entry.text) {
			if edition != "2024" && !slices.Contains(declared, edition) {
				declared = append(declared, edition)
			}
		}
	}

	return fmt.Errorf(
		"`cargo fix --edition` failed moving from edition %s, which the manifests still declare.\n"+
			"Fix the errors it printed, then run `bedrock update` to go on from there; "+
			"`git checkout` undoes the rounds before it",
		strings.Join(declared, ", "),
	)
}
