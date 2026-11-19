package haxe

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

func totalPattern() *regexp.Regexp { return regexp.MustCompile(`Total Issues: \d+ \(Errors: (\d+)`) }

// sourceRoots returns the top directories, or top-level files, that hold the
// Haxe sources git tracks or would track in root, as `-s` arguments for
// checkstyle and the formatter. The local haxelib repo holds libraries, not
// sources.
func sourceRoots(ctx context.Context, runner execx.Runner, root string) ([]string, error) {
	paths, err := gitFiles(ctx, runner, root, "*.hx")
	if err != nil {
		return nil, err
	}

	tops := make([]string, 0, len(paths))

	for _, path := range paths {
		top, _, _ := strings.Cut(path, "/")
		tops = append(tops, top)
	}

	slices.Sort(tops)

	return slices.Compact(tops), nil
}

// sourceArgs returns `-s <root>` for each of roots.
func sourceArgs(roots []string) []string {
	var args []string

	for _, top := range roots {
		args = append(args, "-s", top)
	}

	return args
}

// checkstyleErrors counts the errors a checkstyle text report names in its
// summary line, such as `Total Issues: 3 (Errors: 1, Warnings: 2, Infos: 0)`,
// or 0 for `No issues found.`; finished is false when it printed neither, so
// it did not finish.
func checkstyleErrors(report string) (int, bool) {
	if match := totalPattern().FindStringSubmatch(report); match != nil {
		count, err := strconv.Atoi(match[1])

		return count, err == nil
	}

	if strings.Contains(report, "No issues found.") {
		return 0, true
	}

	return 0, false
}

// lint runs the pinned checkstyle over the Haxe sources with
// `checkstyle.json`. It fails on an error-level finding; warnings only print.
// The summary decides, since `--exitcode` gives the error count as the exit
// status, which wraps to 0 at 256 errors.
func lint(ctx context.Context, c *language.Context, _ []string) (int, error) {
	roots, err := sourceRoots(ctx, c.Runner, c.Root)
	if err != nil {
		return 1, err
	}

	if len(roots) == 0 {
		return 0, nil
	}

	args := append([]string{runFlag, "checkstyle"}, sourceArgs(roots)...)
	args = append(args, "-c", "checkstyle.json", "--nostyle")

	// A program that cannot start is the same failure as one that prints no
	// summary: checkstyle did not finish.
	result, runErr := c.Runner.Run(ctx, execx.Command{Dir: c.Root, Name: "haxelib", Args: args})
	_, _ = c.Stdout.Write(result.Stdout)
	_, _ = c.Stderr.Write(result.Stderr)

	if runErr != nil {
		_, _ = fmt.Fprintln(c.Stderr, runErr)
	}

	errorCount, finished := checkstyleErrors(string(result.Stdout))
	if !finished {
		_, _ = fmt.Fprintln(c.Stderr,
			"checkstyle did not finish: run `just haxe::setup`, which installs it, then check `checkstyle.json`")

		return 1, nil
	}

	return exitCode(errorCount == 0), nil
}

// format runs the pinned formatter over the Haxe sources, which reads
// `hxformat.json` where one sits; with `--check` it only fails when a file
// is not formatted.
func format(ctx context.Context, c *language.Context, args []string) (int, error) {
	roots, err := sourceRoots(ctx, c.Runner, c.Root)
	if err != nil {
		return 1, err
	}

	if len(roots) == 0 {
		return 0, nil
	}

	command := append([]string{"haxelib", runFlag, "formatter"}, sourceArgs(roots)...)
	if slices.Contains(args, "--check") {
		command = append(command, "--check")
	}

	return exitCode(passes(ctx, c, command)), nil
}
