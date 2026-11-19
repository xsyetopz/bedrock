package typescript

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

// runOrFail runs a command in root with the terminal attached, or returns
// the failure.
func runOrFail(ctx context.Context, c *language.Context, failure string, args ...string) error {
	result, err := c.Runner.Run(ctx, execx.Command{Dir: c.Root, Name: nameBun, Args: args, Stream: true})
	if err != nil {
		return fmt.Errorf("run `bun %s`: %w", args[0], err)
	}

	if !result.OK() {
		return errors.New(failure)
	}

	return nil
}

// AfterApply installs the dependencies, which also migrates another package
// manager's lockfile into `bun.lock`, then formats the tree. Those lockfiles
// go only after `bun.lock` exists, so a failed install loses nothing.
// Formatting here keeps the adopt commit apart from the code changes that
// follow it, and lets `just verify` pass its format check on the first run.
func (Typescript) AfterApply(ctx context.Context, c *language.Context) error {
	if err := runOrFail(ctx, c, "`bun install` failed; fix it, then run `just setup`.", nameInstall); err != nil {
		return err
	}

	if fsutil.Exists(filepath.Join(c.Root, "bun.lock")) {
		for _, path := range lockfilesIn(c.Root) {
			if err := os.RemoveAll(filepath.Join(c.Root, filepath.FromSlash(path))); err != nil {
				return fmt.Errorf("remove `%s`: %w", path, err)
			}
		}
	}

	return runOrFail(ctx, c, "`bun fmt` failed; fix what it reports, then run `just format`.", nameFmt)
}

// Runs names what AfterApply does.
func (Typescript) Runs() string { return "`bun install`, then `bun fmt`" }
