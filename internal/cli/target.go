package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/git"
	"github.com/xsyetopz/bedrock/internal/selection"
)

// mode is which entry point runs; they differ in where the language list
// comes from.
type mode string

const (
	modeAdopt  mode = "adopt"
	modeInit   mode = "init"
	modeUpdate mode = "update"
)

// target splits the positionals into the repository and the named
// languages. A first positional that names a language, not a folder, means
// the working directory.
func (a *App) target(mode mode, positionals []string) (string, []string, error) {
	first := ""
	if len(positionals) > 0 {
		first = positionals[0]
	}

	firstIsTarget := mode == modeInit || (first != "" && !a.isLanguageName(first))

	root, names := a.Dir, positionals
	if firstIsTarget {
		root, names = a.root(first), positionals[1:]
	}

	if mode == modeUpdate && len(names) > 0 {
		return "", nil, fmt.Errorf("%w: `bedrock update` takes no languages. Use `bedrock adopt` to add one",
			selection.ErrUsage)
	}

	return root, names, nil
}

// isLanguageName reports whether arg means a language rather than a folder.
// Only the name before the colon counts, so a malformed kind list, such as
// one with a stray comma, reaches the parser and its error.
func (a *App) isLanguageName(arg string) bool {
	name, _, _ := strings.Cut(arg, ":")
	if _, ok := a.Registry.Find(name); !ok {
		return false
	}

	return !fsutil.Exists(a.root(arg))
}

// prepare makes root ready: a fresh repository for init, an existing one otherwise.
func (a *App) prepare(ctx context.Context, mode mode, root string, dryRun bool) error {
	repo := git.Git{Runner: a.Runner, Root: root}

	if mode != modeInit {
		if !repo.IsRepository(ctx) {
			return fmt.Errorf("%s is not a git repository. bedrock changes files in place and relies on git "+
				"to review and undo them; run `git init` and commit first", root)
		}

		return nil
	}

	if dryRun {
		return nil
	}

	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", root, err)
	}

	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty. Run `bedrock adopt` there instead", root)
	}

	if mkdirErr := os.MkdirAll(root, dirMode); mkdirErr != nil {
		return fmt.Errorf("create %s: %w", root, mkdirErr)
	}

	if initErr := repo.Init(ctx); initErr != nil {
		return fmt.Errorf("start a git repository in %s: %w", root, initErr)
	}

	return nil
}

const dirMode = 0o755

func (a *App) adoptUsage(mode mode) string {
	var text strings.Builder

	switch mode {
	case modeAdopt:
		text.WriteString("Usage: bedrock adopt [<dir>] [<language>[:<kind>]...] [options]\n\n" +
			"Moves an existing repo onto bedrock: writes the shared `justfile`, hooks, and\n" +
			"configs, migrates each language's tooling, and removes the tools it replaces.\n" +
			"Without a language, it adopts the ones `.editorconfig` lists or the repo has.\n")
	case modeInit:
		text.WriteString("Usage: bedrock init <dir> <language>[:<kind>[@<platform>,...]]... [options]\n\n" +
			"Starts a repo in <dir>, which must be empty or missing, then adopts it.\n" +
			"A platform list builds every kind before the at sign for those targets, as in\n" +
			"`swift:app@ios,macos`; `bedrock` lists each kind's platforms.\n")
	case modeUpdate:
		text.WriteString("Usage: bedrock update [<dir>] [options]\n\n" +
			"Rewrites every managed file for the languages `.editorconfig` lists, with the\n" +
			"templates of this bedrock release.\n")
	}

	type option struct{ name, usage string }

	options := []option{{name: "--dry-run", usage: "Print the plan. Change nothing."}}

	for _, owned := range a.Registry.Flags() {
		name := "--" + owned.Flag.Name
		if owned.Flag.Value != "" {
			name += " " + owned.Flag.Value
		}

		options = append(options, option{name: name, usage: owned.Flag.Usage})
	}

	options = append(options,
		option{name: "--yes", usage: "Apply without asking (agents, CI)."},
		option{name: "--no-run", usage: "Skip the installs that follow, such as `bun install`."},
		option{name: "--allow-dirty", usage: "Apply even where a changed file holds uncommitted work."},
		option{name: "-h, --help", usage: "Show this help."},
	)

	width := 0
	for _, entry := range options {
		width = max(width, len(entry.name))
	}

	text.WriteString("\nOptions:\n")

	for _, entry := range options {
		fmt.Fprintf(&text, "  %-*s  %s\n", width, entry.name, entry.usage)
	}

	return strings.TrimSuffix(text.String(), "\n")
}
