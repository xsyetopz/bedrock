package dotnet

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

const (
	exitOK     = 0
	exitFailed = 1
)

// Subcommands are the commands `bedrock dotnet` runs: the recipes of
// `.just/dotnet.just` call them.
func (Dotnet) Subcommands() []language.Subcommand {
	return []language.Subcommand{
		{
			Name:    "restore",
			Args:    "",
			Summary: "Restore the NuGet packages of each solution or project.",
			Run:     restore,
		},
		{
			Name:    formatName,
			Args:    "[--check]",
			Summary: "Format the C#, F#, and MSBuild files, or with --check fail if one is not.",
			Run:     format,
		},
		{
			Name:    "typecheck",
			Args:    "",
			Summary: "Build each solution or project without analyzers, warnings as errors.",
			Run:     typecheck,
		},
		{
			Name:    "lint",
			Args:    "[--fix]",
			Summary: "Build with the analyzers and code style as errors, then run FSharpLint.",
			Run:     lint,
		},
		{
			Name:    testName,
			Args:    "",
			Summary: "Run the tests of each solution, or of each test project.",
			Run:     test,
		},
	}
}

// msbuild is a `dotnet <verb>` MSBuild run on one solution or project,
// within the MSBuild bounds.
func msbuild(verb, entry string, flags ...string) []string {
	return slices.Concat([]string{id, verb, entry}, flags, msbuildBounds())
}

// passes runs the command with its output shown, after printing it, and
// reports whether it passed; a program that cannot start fails.
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

// everyPasses reports whether every command passes, run one after another,
// since each build takes much of the machine. All run even after one fails,
// so a run reports them all.
func everyPasses(ctx context.Context, c *language.Context, commands [][]string) bool {
	all := true

	for _, command := range commands {
		all = passes(ctx, c, command) && all
	}

	return all
}

func exitCode(passed bool) int {
	if passed {
		return exitOK
	}

	return exitFailed
}

func restore(ctx context.Context, c *language.Context, _ []string) (int, error) {
	entries, err := buildEntries(ctx, c)
	if err != nil {
		return exitFailed, err
	}

	commands := make([][]string, 0, len(entries))
	for _, entry := range entries {
		commands = append(commands, msbuild("restore", entry))
	}

	return exitCode(everyPasses(ctx, c, commands)), nil
}

// formatCommands are the formatter runs for `bedrock dotnet format`:
// CSharpier over the root, which takes the C# and MSBuild files and skips
// what `.gitignore` lists, and Fantomas over the tracked F# files, since it
// reads only `.fantomasignore` and would otherwise format the generated
// files under `obj/`.
func formatCommands(check, hasCsharp bool, fsharp []string) [][]string {
	var commands [][]string

	if hasCsharp {
		verb := "format"
		if check {
			verb = "check"
		}

		commands = append(commands, []string{id, csharpier, verb, "."})
	}

	if len(fsharp) > 0 {
		args := []string{id, fantomas}
		if check {
			args = append(args, "check")
		}

		commands = append(commands, slices.Concat(args, fsharp))
	}

	return commands
}

// runFormat runs the formatters, or with check fails when a file is not
// formatted.
func runFormat(ctx context.Context, c *language.Context, check bool) (int, error) {
	csharp, err := listMatching(ctx, c, csharpGlob)
	if err != nil {
		return exitFailed, err
	}

	projects, err := projectFiles(ctx, c)
	if err != nil {
		return exitFailed, err
	}

	fsharp, err := listMatching(ctx, c, fsharpGlob)
	if err != nil {
		return exitFailed, err
	}

	commands := formatCommands(check, len(csharp) > 0 || len(projects) > 0, fsharp)

	return exitCode(everyPasses(ctx, c, commands)), nil
}

// format runs CSharpier for C# and the MSBuild files and Fantomas for F#;
// both read their layout from `.editorconfig`.
func format(ctx context.Context, c *language.Context, args []string) (int, error) {
	return runFormat(ctx, c, slices.Contains(args, "--check"))
}

// typecheck builds each solution or project with the analyzers off, so it
// reports what the compilers find, with each warning an error.
func typecheck(ctx context.Context, c *language.Context, _ []string) (int, error) {
	entries, err := buildEntries(ctx, c)
	if err != nil {
		return exitFailed, err
	}

	commands := make([][]string, 0, len(entries))
	for _, entry := range entries {
		commands = append(commands, msbuild("build", entry, "-warnaserror", "-p:RunAnalyzers=false"))
	}

	return exitCode(everyPasses(ctx, c, commands)), nil
}

// lintCommands are the lint commands for entries: a full rebuild of each,
// since the analyzers run only on what compiles, with the `.editorconfig`
// code style enforced and each warning an error; then FSharpLint over each
// F# project, which reads a `fsharplint.json` beside it when the project has
// one.
func lintCommands(entries, fsprojs []string) [][]string {
	commands := make([][]string, 0, len(entries)+len(fsprojs))

	for _, entry := range entries {
		commands = append(commands,
			msbuild("build", entry, "--no-incremental", "-warnaserror", "-p:EnforceCodeStyleInBuild=true"))
	}

	for _, project := range fsprojs {
		commands = append(commands, []string{id, "fsharplint", "lint", project})
	}

	return commands
}

// fixCommands are the fixes `dotnet format` applies for entries that hold
// C#: the `.editorconfig` code style, then the analyzers' code fixes. It
// reads no F#, so an F# project stays out.
func fixCommands(entries []string) [][]string {
	var commands [][]string

	for _, entry := range entries {
		if strings.HasSuffix(entry, ".fsproj") {
			continue
		}

		commands = append(commands,
			[]string{id, formatName, "style", entry, "--severity", "warn"},
			[]string{id, formatName, "analyzers", entry, "--severity", "warn"})
	}

	return commands
}

// lint runs the analyzer and code-style build, then FSharpLint. With
// `--fix`, `dotnet format` first applies the fixes it can, and the
// formatters lay the result out.
func lint(ctx context.Context, c *language.Context, args []string) (int, error) {
	entries, err := buildEntries(ctx, c)
	if err != nil {
		return exitFailed, err
	}

	if slices.Contains(args, "--fix") {
		csharp, listErr := listMatching(ctx, c, csharpGlob)
		if listErr != nil {
			return exitFailed, listErr
		}

		var fixes [][]string
		if len(csharp) > 0 {
			fixes = fixCommands(entries)
		}

		if !everyPasses(ctx, c, fixes) {
			return exitFailed, nil
		}

		return runFormat(ctx, c, false)
	}

	projects, err := projectFiles(ctx, c)
	if err != nil {
		return exitFailed, err
	}

	fsprojs := slices.DeleteFunc(slices.Clone(projects), func(project string) bool {
		return !strings.HasSuffix(project, ".fsproj")
	})

	return exitCode(everyPasses(ctx, c, lintCommands(entries, fsprojs))), nil
}

// isSolution reports whether the entry is a solution file.
func isSolution(entry string) bool {
	return regexp.MustCompile(`\.slnx?$`).MatchString(entry)
}

// usesTestingPlatform reports whether `global.json` opts `dotnet test` into
// Microsoft.Testing.Platform, which takes the solution or project as
// `--solution` or `--project`, where VSTest takes it as an argument.
func usesTestingPlatform(c *language.Context) (bool, error) {
	global, err := readObject(c.Root, globalJSON)
	if err != nil || global == nil {
		return false, err
	}

	return global.section("test").get("runner") == testingPlatform, nil
}

// testCommands are the commands that test one solution or project. VSTest
// takes the MSBuild switches of the bounds. Microsoft.Testing.Platform hands
// unknown switches to each test application, which exits 5 on them, so on it
// a bounded build runs first and `dotnet test --no-build` then runs at most
// `BEDROCK_DOTNET_JOBS` test applications at once.
func testCommands(entry string, platform bool) [][]string {
	if !platform {
		return [][]string{slices.Concat([]string{id, testName, entry}, msbuildBounds())}
	}

	flag := "--project"
	if isSolution(entry) {
		flag = "--solution"
	}

	return [][]string{
		msbuild("build", entry),
		{
			id, testName, flag, entry, "--no-build",
			"--max-parallel-test-modules", strconv.Itoa(jobs()),
			// Exit code 8 is a run with no test. VSTest passes a test project
			// that has none yet, so this runner does too; a failing test
			// still fails.
			"--ignore-exit-code", "8",
		},
	}
}

// testEntries are the solutions or projects `dotnet test` runs: each
// solution, or without one, each test project, since `dotnet test` on a
// product project finds no tests and builds it for nothing. None when no
// project is a test project.
func testEntries(ctx context.Context, c *language.Context) ([]string, error) {
	entries, err := buildEntries(ctx, c)
	if err != nil {
		return nil, err
	}

	tests, err := testProjects(ctx, c)
	if err != nil || len(tests) == 0 {
		return nil, err
	}

	if slices.ContainsFunc(entries, isSolution) {
		return entries, nil
	}

	paths := make([]string, 0, len(tests))
	for _, project := range tests {
		paths = append(paths, filepath.ToSlash(project.path))
	}

	return paths, nil
}

// inTurn reports whether each command passes, stopping at the first that
// fails.
func inTurn(ctx context.Context, c *language.Context, commands [][]string) bool {
	for _, command := range commands {
		if !passes(ctx, c, command) {
			return false
		}
	}

	return true
}

// test runs the tests of each solution, or each test project, on the runner
// `global.json` names. A build failure skips that entry's test run, since its
// test applications would be stale; each entry runs even after one fails.
func test(ctx context.Context, c *language.Context, _ []string) (int, error) {
	entries, err := testEntries(ctx, c)
	if err != nil {
		return exitFailed, err
	}

	platform, err := usesTestingPlatform(c)
	if err != nil {
		return exitFailed, err
	}

	all := true
	for _, entry := range entries {
		all = inTurn(ctx, c, testCommands(entry, platform)) && all
	}

	return exitCode(all), nil
}
