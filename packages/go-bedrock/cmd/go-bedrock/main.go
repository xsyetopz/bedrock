// Command go-bedrock starts and adopts strict Go modules.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xsyetopz/bedrock/packages/go-bedrock/internal/scaffold"
)

const usage = `go-bedrock new <dir> <module>
go-bedrock adopt [<dir>] [--dry-run | --yes]

new starts a module in <dir> with a cmd/<dir> command, golangci-lint's config, and the recipes.
adopt prints what it would change in an existing module (--dry-run, the default),
or makes the change (--yes).`

const version = "0.1.0"

// newArgs counts the arguments of `new`: the word, the folder, and the module.
const newArgs = 3

var errUsage = errors.New("usage")

func main() {
	err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "go-bedrock:", err)

		if errors.Is(err, errUsage) {
			fmt.Fprintln(os.Stderr, usage)
		}

		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errUsage
	}

	switch args[0] {
	case "--version":
		fmt.Fprintln(os.Stdout, version)

		return nil
	case "new":
		if len(args) != newArgs {
			return errUsage
		}

		plan, err := scaffold.NewPlan(args[1], args[2], filepath.Base(args[1]))
		if err != nil {
			return fmt.Errorf("plan: %w", err)
		}

		return finish(args[1], plan, true)
	case "adopt":
		return adopt(args[1:])
	default:
		return errUsage
	}
}

func adopt(args []string) error {
	dir, apply := ".", false

	for _, arg := range args {
		switch arg {
		case "--yes":
			apply = true
		case "--dry-run":
			apply = false
		default:
			dir = arg
		}
	}

	module, err := scaffold.ModulePath(dir)
	if err != nil {
		return fmt.Errorf("%s is not a Go module, run go mod init first: %w", dir, err)
	}

	plan, err := scaffold.AdoptPlan(dir, module)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	return finish(dir, plan, apply)
}

func finish(dir string, plan []scaffold.Entry, apply bool) error {
	for _, entry := range plan {
		fmt.Fprintf(os.Stdout, "%s %s\n", entry.Action, entry.Path)
	}

	if !apply {
		return nil
	}

	err := scaffold.Apply(dir, plan)
	if err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	return nil
}
