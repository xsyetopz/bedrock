package swift

import (
	"context"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// swiftCommand runs `swift` in dir without the git variables a hook sets.
// A hook in a worktree exports GIT_DIR and its siblings, which send
// SwiftPM's own git checkouts to the wrong repository. execx can only add
// variables, so `env -u` drops them.
func swiftCommand(dir string, args ...string) execx.Command {
	prefix := []string{"-u", "GIT_DIR", "-u", "GIT_WORK_TREE", "-u", "GIT_INDEX_FILE", swiftTool}

	return execx.Command{
		Stdin:  nil,
		Dir:    dir,
		Name:   "env",
		Args:   append(prefix, args...),
		Env:    nil,
		Stream: false,
	}
}

// runSwift runs `swift` with args in dir and captures its output.
func runSwift(ctx context.Context, runner execx.Runner, dir string, args ...string) (execx.Result, error) {
	return runner.Run(ctx, swiftCommand(dir, args...))
}

// firstLine returns the first non-blank line of a tool's error output, for
// a finding.
func firstLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}

	return "no output"
}
