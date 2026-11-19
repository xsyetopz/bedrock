// Command bedrock sets up repositories in any mix of its languages.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/xsyetopz/bedrock"
	"github.com/xsyetopz/bedrock/internal/catalog"
	"github.com/xsyetopz/bedrock/internal/cli"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/sizeguard"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app, err := newApp()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		return cli.ExitFailed
	}

	return app.Run(ctx, os.Args[1:])
}

func newApp() (*cli.App, error) {
	registry, err := catalog.Registry()
	if err != nil {
		return nil, fmt.Errorf("load the languages: %w", err)
	}

	version, err := bedrock.Version()
	if err != nil {
		return nil, err
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("find the working directory: %w", err)
	}

	return &cli.App{
		Registry:    registry,
		Templates:   bedrock.Templates(),
		Runner:      execx.System{Stdout: os.Stdout, Stderr: os.Stderr},
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Getenv:      os.Getenv,
		Version:     version,
		Dir:         dir,
		Interactive: isTerminal(os.Stdin),
		Sizes:       sizeguard.Guard,
	}, nil
}

// isTerminal reports whether file is a character device, as a terminal a
// person types at is; a pipe or a file is not.
func isTerminal(file *os.File) bool {
	info, err := file.Stat()

	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
