package rust

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// legacy is what a repository adopted by rust-bedrock still carries.
type legacy struct {
	language.Contribution

	// dropXtask is set when the xtask crate is bedrock's and goes, so the root
	// manifest drops its member entry.
	dropXtask bool
}

// xtaskState is what the repository's xtask crate is.
type xtaskState struct {
	notes   []string
	present bool
	// ours is set when the crate depends on rust-bedrock, which only bedrock's
	// xtask did.
	ours bool
}

// dependsOnBedrock reports whether the manifest depends on the rust-bedrock
// crate.
func dependsOnBedrock(text string) bool {
	doc, _ := parseToml(text)

	for _, name := range []string{"dependencies", "dev-dependencies", "build-dependencies"} {
		if deps, ok := at(doc, name).(table); ok {
			if _, has := deps["rust-bedrock"]; has {
				return true
			}
		}
	}

	return false
}

// xtaskPlan inspects the xtask crate: removed whole when bedrock wrote it, with
// a note for files it did not write.
func xtaskPlan(ctx context.Context, runner execx.Runner, root string) (xtaskState, error) {
	manifestText, present, err := fsutil.ReadText(filepath.Join(root, "xtask", "Cargo.toml"))
	if err != nil {
		return xtaskState{}, err
	}

	if !present {
		return xtaskState{notes: nil, present: false, ours: false}, nil
	}

	if !dependsOnBedrock(manifestText) {
		return xtaskState{
			present: true,
			ours:    false,
			notes: []string{
				"`xtask` does not depend on `rust-bedrock`, so it stays; remove it by hand if nothing else uses it.",
			},
		}, nil
	}

	files, err := fsutil.ListFiles(
		ctx,
		runner,
		root,
		func(file string) bool { return strings.HasPrefix(file, "xtask/") },
	)
	if err != nil {
		return xtaskState{}, err
	}

	extra := slices.DeleteFunc(files, func(file string) bool {
		return file == "xtask/Cargo.toml" || file == "xtask/src/main.rs"
	})

	notes := []string{
		"`Cargo.lock` still lists `xtask` and `rust-bedrock` until `cargo` runs again (`cargo fetch` does it).",
	}

	if len(extra) > 0 {
		notes = append(notes,
			"`xtask` held files bedrock did not write: `"+strings.Join(extra, "`, `")+
				"`. They go with the crate; git still has them.")
	}

	return xtaskState{present: true, ours: true, notes: notes}, nil
}

// cargoConfigStep returns `.cargo/config.toml` without the bedrock alias,
// removed when nothing else is in it.
func cargoConfigStep(root string, removable bool) (step.Step, bool, error) {
	text, present, err := fsutil.ReadText(filepath.Join(root, ".cargo", "config.toml"))
	if err != nil || !present || !removable {
		return nil, false, err
	}

	edited := withoutBedrockAlias(text)
	if edited == text {
		return nil, false, nil
	}

	if strings.TrimSpace(edited) == "" {
		return step.Remove{Path: ".cargo/config.toml"}, true, nil
	}

	return step.Write{Path: ".cargo/config.toml", Content: []byte(edited), Executable: false}, true, nil
}

// denyStep returns `deny.toml` without the `allow-git` entry the xtask
// dependency needed.
func denyStep(root string) (step.Step, bool, error) {
	text, present, err := fsutil.ReadText(filepath.Join(root, "deny.toml"))
	if err != nil || !present {
		return nil, false, err
	}

	edited := withoutBedrockGit(text)
	if edited == text {
		return nil, false, nil
	}

	return step.Write{Path: "deny.toml", Content: []byte(edited), Executable: false}, true, nil
}

// legacyPlan returns what to remove from a repository that rust-bedrock
// adopted: its xtask crate (only when that depends on rust-bedrock), the
// `bedrock` cargo alias, and the `deny.toml` entry that allowed the git
// dependency.
func legacyPlan(ctx context.Context, runner execx.Runner, root string) (legacy, error) {
	xtask, err := xtaskPlan(ctx, runner, root)
	if err != nil {
		return legacy{}, err
	}

	var result legacy

	if xtask.ours {
		result.Steps = append(result.Steps, step.Remove{Path: "xtask"})
	}

	config, hasConfig, err := cargoConfigStep(root, xtask.ours || !xtask.present)
	if err != nil {
		return legacy{}, err
	}

	deny, hasDeny, err := denyStep(root)
	if err != nil {
		return legacy{}, err
	}

	for _, edit := range []struct {
		step step.Step
		has  bool
	}{{config, hasConfig}, {deny, hasDeny}} {
		if edit.has {
			result.Steps = append(result.Steps, edit.step)
			result.ProjectFiles = append(result.ProjectFiles, edit.step.Target())
		}
	}

	result.Notes = xtask.notes
	result.dropXtask = xtask.ours

	return result, nil
}
