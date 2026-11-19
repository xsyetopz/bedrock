package cli

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/xsyetopz/bedrock/internal/commitmsg"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/selection"
)

// subcommandUsage is a language subcommand as its usage line names it, such
// as jvm j2k <path>....
func subcommandUsage(id string, sub language.Subcommand) string {
	return strings.TrimSpace(id + " " + sub.Name + " " + sub.Args)
}

// subcommand runs bedrock <language> <name> in the working directory.
func (a *App) subcommand(
	ctx context.Context,
	lang language.Language,
	provider language.SubcommandProvider,
	args []string,
) (int, error) {
	id := lang.Spec().ID

	name, rest := "", []string(nil)
	if len(args) > 0 {
		name, rest = args[0], args[1:]
	}

	subs := provider.Subcommands()

	index := slices.IndexFunc(subs, func(sub language.Subcommand) bool { return sub.Name == name })
	if index < 0 {
		names := make([]string, 0, len(subs))
		for _, sub := range subs {
			names = append(names, sub.Name)
		}

		return ExitUsage, fmt.Errorf("%w: `bedrock %s` has no subcommand `%s`. Subcommands: %s",
			selection.ErrUsage, id, name, strings.Join(names, ", "))
	}

	sub := subs[index]

	// Without this, --help reaches the subcommand as a path to convert.
	if slices.ContainsFunc(rest, isHelp) {
		a.print("Usage: bedrock " + subcommandUsage(id, sub) + "\n\n" + sub.Summary)

		return ExitOK, nil
	}

	cfg, _, err := config.Read(a.Dir)
	if err != nil {
		return ExitFailed, fmt.Errorf("read `%s`: %w", config.Path, err)
	}

	selected, err := a.selector().Select(cfg, []selection.Named{{Language: lang, Kind: ""}}, nil)
	if err != nil {
		return ExitFailed, err
	}

	entry := plan.Selected{Language: lang, Options: config.Options{}}

	if found := slices.IndexFunc(selected, func(s plan.Selected) bool { return s.Language == lang }); found >= 0 {
		entry = selected[found]
	}

	return sub.Run(ctx, a.engine().Context(a.Dir, selected, entry, false), rest)
}

const commitMsgUsage = `Usage: bedrock commit-msg <file>

Checks the commit message in <file>, as the commit-msg hook hands it, against
Conventional Commits with no line over 100 characters.`

func commitMsgHelp() string {
	return fmt.Sprintf("commit-msg: use Conventional Commits, for example \"feat(cli): add `--dry-run`\", "+
		"with no line over %d characters.\nTypes: %s. https://www.conventionalcommits.org",
		commitmsg.MaxLine, strings.Join(commitmsg.Types(), ", "))
}

func (a *App) commitMsg(_ context.Context, args []string) (int, error) {
	if len(args) > 0 && isHelp(args[0]) {
		a.print(commitMsgUsage)

		return ExitOK, nil
	}

	if len(args) == 1 {
		text, err := os.ReadFile(a.root(args[0]))
		if err == nil && commitmsg.Conventional(string(text)) {
			return ExitOK, nil
		}
	}

	_, _ = fmt.Fprintln(a.Stderr, commitMsgHelp())

	return ExitFailed, nil
}
