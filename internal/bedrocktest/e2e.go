package bedrocktest

import (
	"os"
	"testing"
)

// E2EVariable turns on the end-to-end tests. They build real projects with
// real toolchains, minutes of every core per language, so `go test ./...`
// skips them unless it is set; CI's e2e workflow sets it.
const E2EVariable = "BEDROCK_E2E"

// E2E skips t unless BEDROCK_E2E is set, and always in short mode. runs
// names what the test runs, such as "runs `cargo fix`", for the skip message.
func E2E(t *testing.T, runs string) {
	t.Helper()

	if testing.Short() || os.Getenv(E2EVariable) == "" {
		t.Skipf("%s; set %s=1 to run it", runs, E2EVariable)
	}
}
