package jvm

import (
	"context"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// FindJDK exposes findJdk to the black-box tests, which cannot set JAVA_HOME
// while they run in parallel.
func FindJDK(ctx context.Context, runner execx.Runner, configured string) (string, bool) {
	found, ok := findJdk(ctx, runner, configured)

	return found.home, ok
}
