// Package cli is bedrock's command line. App holds everything a command
// touches outside its arguments, such as the streams, the environment, and
// the languages, so a test runs a command exactly as a person does and reads
// what it printed.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/selection"
	"github.com/xsyetopz/bedrock/internal/sizeguard"
)

// Exit codes a command returns.
const (
	ExitOK      = 0
	ExitFailed  = 1
	ExitUsage   = 2
	helpCommand = "help"
)

// Names more than one command matches.
const (
	versionCommand = "version"
	helpFlag       = "--help"
	shortHelpFlag  = "-h"
)

// isHelp reports whether arg asks for help.
func isHelp(arg string) bool { return arg == helpFlag || arg == shortHelpFlag }

// SizeReport is what the size guard found: errors fail the guard, and
// warnings only print. It is defined where the guard is, so the guard need
// not import cli.
type SizeReport = sizeguard.Report

// SizeGuard bounds the size of the selected languages' sources by the
// limits .editorconfig sets.
type SizeGuard func(ctx context.Context, engine *plan.Engine, root string, selected []plan.Selected) (SizeReport, error)

// App is one bedrock process.
type App struct {
	Registry  *language.Registry
	Templates fs.FS
	Runner    execx.Runner
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	// Getenv reads the environment, such as CI.
	Getenv func(key string) string
	// Sizes runs the size guard; nil skips it.
	Sizes SizeGuard
	// Version is this release, such as 0.9.0.
	Version string
	// Dir is the working directory, which commands without a <dir> act on.
	Dir string
	// Interactive is true when Stdin is a terminal a person answers at.
	Interactive bool
}

type command struct {
	run     func(ctx context.Context, args []string) (int, error)
	usage   string
	summary string
}

func (a *App) commands() map[string]command {
	return map[string]command{
		"init": {
			run:     a.adoptCommand(modeInit),
			usage:   "init <dir> <lang>[:<kind>]...",
			summary: "Start a repo.",
		},
		"adopt": {
			run:     a.adoptCommand(modeAdopt),
			usage:   "adopt [<dir>] [<lang>...]",
			summary: "Move an existing repo onto bedrock.",
		},
		"update": {
			run:     a.adoptCommand(modeUpdate),
			usage:   "update [<dir>]",
			summary: "Rewrite the managed files with this release.",
		},
		"guard": {run: a.guard, usage: "guard [base|<lang>...]", summary: "Report what the linters cannot see."},
		"clean": {run: a.clean, usage: "clean [<dir>]", summary: "Delete `*.bak` files older releases left."},
		"languages": {
			run:     a.languages,
			usage:   "languages [--staged | --unpushed]",
			summary: "List the adopted languages.",
		},
		"commit-msg": {
			run:     a.commitMsg,
			usage:   "commit-msg <file>",
			summary: "Check a commit message against Conventional Commits.",
		},
		versionCommand: {run: a.version, usage: versionCommand, summary: "Print the version."},
	}
}

// commandOrder is the order the usage lists the commands in.
func commandOrder() []string {
	return []string{"init", "adopt", "update", "guard", "clean", "languages", "commit-msg", versionCommand}
}

// Run runs the command args name and returns the process's exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	name, rest := "", []string(nil)
	if len(args) > 0 {
		name, rest = args[0], args[1:]
	}

	run := a.dispatch(name)
	if run == nil {
		a.print(a.usage())

		if slices.Contains([]string{"", shortHelpFlag, helpFlag, helpCommand}, name) {
			return ExitOK
		}

		return ExitUsage
	}

	code, err := run(ctx, rest)
	if err != nil {
		_, _ = fmt.Fprintln(a.Stderr, err)

		if errors.Is(err, selection.ErrUsage) {
			return ExitUsage
		}

		return ExitFailed
	}

	return code
}

func (a *App) dispatch(name string) func(context.Context, []string) (int, error) {
	if cmd, ok := a.commands()[name]; ok {
		return cmd.run
	}

	if lang, found := a.Registry.Find(name); found {
		if provider, ok := lang.(language.SubcommandProvider); ok {
			return func(ctx context.Context, args []string) (int, error) {
				return a.subcommand(ctx, lang, provider, args)
			}
		}
	}

	return nil
}

func (a *App) usage() string {
	type line struct{ usage, summary string }

	var lines []line

	commands := a.commands()
	for _, name := range commandOrder() {
		lines = append(lines, line{usage: commands[name].usage, summary: commands[name].summary})
	}

	var kinds []string

	for _, lang := range a.Registry.All() {
		spec := lang.Spec()
		if provider, ok := lang.(language.SubcommandProvider); ok {
			for _, sub := range provider.Subcommands() {
				lines = append(lines, line{usage: subcommandUsage(spec.ID, sub), summary: sub.Summary})
			}
		}

		entry := spec.ID
		if len(spec.Kinds) > 0 {
			entry += " (kinds: " + strings.Join(spec.Kinds, ", ") + ")"
		}

		kinds = append(kinds, entry)
	}

	// Summaries start two spaces past the longest usage, so a new subcommand never runs into its text.
	width := 0
	for _, entry := range lines {
		width = max(width, len(entry.usage))
	}

	var text strings.Builder

	fmt.Fprintf(&text, "bedrock %s: one setup for repos in any mix of its languages.\n\n", a.Version)
	text.WriteString("Usage: bedrock <command> [args]\n\nCommands:\n")

	for _, entry := range lines {
		fmt.Fprintf(&text, "  %-*s  %s\n", width, entry.usage, entry.summary)
	}

	fmt.Fprintf(&text, "\nLanguages: %s.\nRun `bedrock <command> --help` for its options.", strings.Join(kinds, "; "))

	return text.String()
}

func (a *App) version(context.Context, []string) (int, error) {
	a.print(a.Version)

	return ExitOK, nil
}

func (a *App) print(lines ...string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(a.Stdout, line)
	}
}

// root resolves a <dir> argument against the working directory; empty
// means the working directory itself.
func (a *App) root(arg string) string {
	if arg == "" {
		return a.Dir
	}

	if filepath.IsAbs(arg) {
		return filepath.Clean(arg)
	}

	return filepath.Join(a.Dir, arg)
}

func (a *App) engine() *plan.Engine {
	return &plan.Engine{
		Registry:  a.Registry,
		Templates: a.Templates,
		Runner:    a.Runner,
		Stdout:    a.Stdout,
		Stderr:    a.Stderr,
	}
}

func (a *App) selector() selection.Selector {
	return selection.Selector{Registry: a.Registry, Templates: a.Templates}
}

// confirm asks a person to approve; without a terminal there is no one to
// ask, so it refuses.
func (a *App) confirm(yes bool) bool {
	if yes {
		return true
	}

	if !a.Interactive {
		_, _ = fmt.Fprintln(a.Stderr, "No terminal to confirm. Pass `--yes` to apply, or `--dry-run` to preview.")

		return false
	}

	_, _ = fmt.Fprint(a.Stdout, "Apply these changes? [y/N] ")

	answer, _ := bufio.NewReader(a.Stdin).ReadString('\n')

	return strings.EqualFold(strings.TrimSpace(answer), "y")
}
