package cpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

// exitMissing is the status of a program that could not start, as shells
// report it.
const exitMissing = 127

// toolError reports a build tool that failed, with the tool's own words.
type toolError struct{ message string }

func (e *toolError) Error() string { return e.message }

// toolFailed formats a toolError.
func toolFailed(format string, args ...any) error {
	return &toolError{message: fmt.Sprintf(format, args...)}
}

// output is what a toolchain program printed.
type output struct {
	stdout string
	stderr string
	code   int
}

// run captures a program's output; a missing executable is code 127.
func run(ctx context.Context, c *language.Context, dir string, name string, args ...string) output {
	result, err := c.Runner.Run(ctx, execx.Command{Dir: dir, Name: name, Args: args})
	if err != nil {
		return output{stdout: "", stderr: err.Error(), code: exitMissing}
	}

	return output{stdout: string(result.Stdout), stderr: string(result.Stderr), code: result.Code}
}

// firstLine returns the first non-blank line of a tool's error output, for a
// message.
func firstLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}

	return "no output"
}

// strs is a JSON list of strings that reads as empty where a tool prints
// something else, such as xmake's `{}` for an empty list.
type strs []string

// UnmarshalJSON keeps the strings of a list and ignores any other value.
func (s *strs) UnmarshalJSON(data []byte) error {
	*s = nil

	// Not a list, such as xmake's `{}`, reads as empty rather than failing.
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) {
		return nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("parse a list of strings: %w", err)
	}

	for _, item := range items {
		var text string
		if json.Unmarshal(item, &text) == nil {
			*s = append(*s, text)
		}
	}

	return nil
}
