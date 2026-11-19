package haxe

import (
	"context"
	"fmt"

	"github.com/xsyetopz/bedrock/internal/language"
)

// typecheck compiles each hxml file that names its target, from the root,
// with `--no-output`, which `--each` applies to every compilation `--next`
// separates. An hxml file without a target is a part the command line
// completes, such as one a recipe adds `-js <file>` to, or a list of
// libraries; its target decides which packages it can use, so it is left to
// the recipe that completes it.
func typecheck(ctx context.Context, c *language.Context, _ []string) (int, error) {
	builds, err := hxmlBuilds(ctx, c.Runner, c.Root)
	if err != nil {
		return 1, err
	}

	var commands [][]string

	for _, b := range builds {
		if b.target {
			commands = append(commands, []string{program, "--no-output", "--each", b.path})
		}
	}

	return exitCode(everyPasses(ctx, c, commands)), nil
}

// test runs each test hxml file that says how its tests run: `--interp`,
// `--run`, or a `--cmd` after the compilation. One that does not is named,
// since the target it runs on is the project's choice, and a recipe in
// `project.just` runs it.
func test(ctx context.Context, c *language.Context, _ []string) (int, error) {
	builds, err := hxmlBuilds(ctx, c.Runner, c.Root)
	if err != nil {
		return 1, err
	}

	var commands [][]string

	for _, b := range builds {
		switch {
		case !isTestHxml(b.path):
		case b.runs:
			commands = append(commands, []string{program, b.path})
		default:
			_, _ = fmt.Fprintf(c.Stderr,
				"not run `%s`: it names no `--interp`, `--run`, or `--cmd`, so a recipe in `project.just` runs it\n",
				b.path)
		}
	}

	return exitCode(everyPasses(ctx, c, commands)), nil
}

// everyPasses reports whether every command passes, run one after another,
// since each compilation can take much of the machine's memory. All of them
// run even after one fails, so a run reports them all.
func everyPasses(ctx context.Context, c *language.Context, commands [][]string) bool {
	all := true

	for _, command := range commands {
		if !passes(ctx, c, command) {
			all = false
		}
	}

	return all
}

func exitCode(passed bool) int {
	if passed {
		return 0
	}

	return 1
}
