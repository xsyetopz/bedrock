package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/pflag"

	"github.com/xsyetopz/bedrock/internal/selection"
)

// flagSet returns a flag set that parses flags anywhere among the
// positionals, as the TS release's parseArgs did, and reports its own errors
// through the returned error rather than printing them.
func flagSet(name string) (*pflag.FlagSet, *bool) {
	flags := pflag.NewFlagSet(name, pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	help := flags.BoolP("help", "h", false, "Show this help.")

	return flags, help
}

// parse parses args into flags, wrapping a bad flag as a usage error.
func parse(flags *pflag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}

		return fmt.Errorf("%w: %w; run `bedrock %s --help`", selection.ErrUsage, err, flags.Name())
	}

	return nil
}
