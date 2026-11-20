package language

import (
	"os"
	"strconv"
	"strings"
)

// DefaultJobs is how many processes a language's tools run at once when no
// variable sets it: enough to use a machine, few enough to leave it usable.
const DefaultJobs = 4

// Jobs returns how many build, test, or lint processes the tools of the
// language with the given ID run at once: BEDROCK_<ID>_JOBS, else
// BEDROCK_JOBS, else DefaultJobs. A value that is not a positive integer
// counts as unset, so a typo falls back to the bound instead of lifting it.
func Jobs(id string) int {
	for _, name := range []string{"BEDROCK_" + strings.ToUpper(id) + "_JOBS", "BEDROCK_JOBS"} {
		count, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
		if err == nil && count > 0 {
			return count
		}
	}

	return DefaultJobs
}
