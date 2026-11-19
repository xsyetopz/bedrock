package haxe

import (
	"context"
	"fmt"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

// passes runs command in the root with its output shown, after printing it,
// and reports whether it passed; a program that cannot start fails.
func passes(ctx context.Context, c *language.Context, command []string) bool {
	_, _ = fmt.Fprintln(c.Stderr, "$ "+strings.Join(command, " "))

	result, err := c.Runner.Run(ctx, execx.Command{
		Dir: c.Root, Name: command[0], Args: command[1:], Stream: true,
	})
	if err != nil {
		_, _ = fmt.Fprintln(c.Stderr, err)

		return false
	}

	return result.OK()
}
