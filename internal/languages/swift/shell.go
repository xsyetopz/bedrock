package swift

import (
	"context"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// swiftCommand runs `swift` in dir without the git variables a hook sets,
// which would send SwiftPM's own git checkouts to the hook's repository.
func swiftCommand(dir string, args ...string) execx.Command {
	return execx.Command{
		Stdin:  nil,
		Dir:    dir,
		Name:   swiftTool,
		Args:   args,
		Env:    nil,
		Unset:  execx.HookGitVars(),
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
