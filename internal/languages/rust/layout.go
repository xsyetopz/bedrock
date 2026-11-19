package rust

import (
	"context"
	"slices"

	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
)

// layout returns the renames, manifests, toolchain, and test gates the plan
// would still change.
func layout(ctx context.Context, c *language.Context) ([]string, error) {
	plan, err := planRust(ctx, c)
	if err != nil {
		return nil, err
	}

	pending, err := step.Pending(c.Root, plan.Steps)
	if err != nil {
		return nil, err
	}

	open := slices.DeleteFunc(pending, func(entry step.Step) bool {
		return !slices.Contains(plan.ProjectFiles, entry.Target())
	})

	// A rename that rewrites paths is a write of `foo.rs` plus a removal of
	// `foo/mod.rs`.
	writes := map[string]bool{}

	for _, entry := range plan.Steps {
		if write, ok := entry.(step.Write); ok {
			writes[write.Path] = true
		}
	}

	superseded := map[string]bool{}

	for _, entry := range open {
		if remove, ok := entry.(step.Remove); ok {
			if target, renamed := renamedTo(writes, remove.Path); renamed {
				superseded[target] = true
			}
		}
	}

	var findings []string

	for _, entry := range open {
		if !superseded[entry.Target()] {
			findings = append(findings, layoutFinding(writes, entry))
		}
	}

	return findings, nil
}

// layoutFinding describes one open step.
func layoutFinding(writes map[string]bool, entry step.Step) string {
	if moved, ok := entry.(step.Move); ok {
		return renameFinding(moved.From, moved.Path)
	}

	if removed, ok := entry.(step.Remove); ok {
		if target, renamed := renamedTo(writes, removed.Path); renamed {
			return renameFinding(removed.Path, target)
		}
	}

	return "`" + entry.Target() + "`: differs from what `bedrock` writes; run `bedrock update`"
}

func renameFinding(from, to string) string {
	return "`" + from + "`: name it `" + to + "`, beside its directory (`bedrock update` does it)"
}
