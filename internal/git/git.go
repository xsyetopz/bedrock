// Package git asks git what bedrock needs to know before it changes files:
// whether a directory is a work tree, which paths hold uncommitted work, and
// which paths a commit or push changes.
package git

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// Git runs git in one repository.
type Git struct {
	Runner execx.Runner
	Root   string
}

func (g Git) output(ctx context.Context, stdin string, args ...string) (string, error) {
	command := execx.Command{Dir: g.Root, Name: "git", Args: args}
	if stdin != "" {
		command.Stdin = strings.NewReader(stdin)
	}

	return execx.Output(ctx, g.Runner, command)
}

// IsRepository reports whether Root sits inside a git work tree.
func (g Git) IsRepository(ctx context.Context) bool {
	_, err := g.output(ctx, "", "rev-parse", "--is-inside-work-tree")

	return err == nil
}

// Init makes Root a new repository.
func (g Git) Init(ctx context.Context) error {
	_, err := g.output(ctx, "", "init", "--quiet")

	return err
}

// lefthookHooks are the hooks lefthook.yml lists, which just setup installs.
func lefthookHooks() []string { return []string{"pre-commit", "commit-msg", "pre-push"} }

// HooksOn reports whether git runs each lefthook hook. --git-path resolves
// the hooks directory git uses, which core.hooksPath moves and which every
// worktree shares with the main checkout.
func (g Git) HooksOn(ctx context.Context) bool {
	dir, err := g.output(ctx, "", "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return false
	}

	dir = strings.TrimSpace(dir)

	for _, hook := range lefthookHooks() {
		text, readErr := os.ReadFile(
			filepath.Join(dir, hook),
		)
		if readErr != nil || !strings.Contains(string(text), `call_lefthook run "`+hook+`"`) {
			return false
		}
	}

	return true
}

// Uncommitted returns the paths among paths that hold work git does not
// have: modified, staged, or untracked. Ignored files are build output and
// do not count.
func (g Git) Uncommitted(ctx context.Context, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	args := append([]string{"status", "--porcelain=v1", "-z", "--untracked-files=all", "--"}, paths...)

	out, err := g.output(ctx, "", args...)
	if err != nil {
		return nil, err
	}

	var dirty []string

	for entry := range strings.SplitSeq(out, "\x00") {
		if len(entry) > 3 { //nolint:mnd // A porcelain entry is "XY path".
			dirty = append(dirty, entry[3:])
		}
	}

	slices.Sort(dirty)

	return dirty, nil
}

// HeldByGit returns the files among paths whose exact content is a blob in
// the object store, so deleting them loses nothing git cannot restore.
func (g Git) HeldByGit(ctx context.Context, paths []string) (map[string]bool, error) {
	held := map[string]bool{}
	if len(paths) == 0 {
		return held, nil
	}

	hashes, err := g.output(ctx, strings.Join(paths, "\n"), "hash-object", "--stdin-paths")
	if err != nil {
		return nil, err
	}

	checked, err := g.output(ctx, strings.TrimSpace(hashes)+"\n", "cat-file", "--batch-check")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(checked), "\n")
	for index, path := range paths {
		if index < len(lines) && !strings.HasSuffix(lines[index], " missing") {
			held[path] = true
		}
	}

	return held, nil
}

// Staged returns the staged paths: added, copied, modified, or renamed.
func (g Git) Staged(ctx context.Context) ([]string, error) {
	out, err := g.output(ctx, "", "diff", "--cached", "--name-only", "-z", "--diff-filter=ACMR")
	if err != nil {
		return nil, err
	}

	return split(out), nil
}

// Unpushed returns the paths HEAD's commits change that no remote-tracking
// branch has yet, deletions included: the diff from the oldest such commit's
// parent. It returns false when that commit has no parent, since then the
// whole history is new.
func (g Git) Unpushed(ctx context.Context) ([]string, bool, error) {
	commits, err := g.output(ctx, "", "rev-list", "--reverse", "HEAD", "--not", "--remotes")
	if err != nil {
		return nil, false, err
	}

	oldest, _, _ := strings.Cut(strings.TrimSpace(commits), "\n")
	if oldest == "" {
		return nil, true, nil
	}

	base, err := g.output(ctx, "", "rev-parse", "--verify", "--quiet", oldest+"^")
	if err != nil {
		return nil, false, nil //nolint:nilerr // A root commit has no parent: every path is new.
	}

	diff, err := g.output(ctx, "", "diff", "--name-only", "-z", strings.TrimSpace(base), "HEAD")
	if err != nil {
		return nil, false, err
	}

	return split(diff), true, nil
}

func split(out string) []string {
	var paths []string

	for path := range strings.SplitSeq(out, "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}

	return paths
}
