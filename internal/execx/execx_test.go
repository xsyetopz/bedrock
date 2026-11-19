package execx_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/execx"
)

// childEnv runs `env` with command's environment settings and returns its variables.
func childEnv(t *testing.T, command execx.Command) []string {
	t.Helper()

	command.Name = "env"

	output, err := execx.Output(t.Context(), execx.System{}, command)
	if err != nil {
		t.Fatal(err)
	}

	return strings.Split(strings.TrimSpace(output), "\n")
}

func TestUnsetHidesAnInheritedVariableAndKeepsTheRest(t *testing.T) {
	// Arrange
	t.Setenv("GIT_DIR", "/elsewhere/.git")
	t.Setenv("BEDROCK_KEPT", "yes")

	// Act
	env := childEnv(t, execx.Command{Unset: execx.HookGitVars(), Env: []string{"BEDROCK_ADDED=1"}})

	// Assert
	if slices.ContainsFunc(env, func(pair string) bool { return strings.HasPrefix(pair, "GIT_DIR=") }) {
		t.Errorf("the program sees `GIT_DIR`: %v", env)
	}

	for _, want := range []string{"BEDROCK_KEPT=yes", "BEDROCK_ADDED=1"} {
		if !slices.Contains(env, want) {
			t.Errorf("the program misses %s", want)
		}
	}
}

func TestACommandWithoutEnvironmentSettingsInheritsEverything(t *testing.T) {
	// Arrange
	t.Setenv("GIT_DIR", "/elsewhere/.git")

	// Act
	env := childEnv(t, execx.Command{})

	// Assert
	if !slices.Contains(env, "GIT_DIR=/elsewhere/.git") {
		t.Errorf("the program misses the inherited `GIT_DIR`: %v", env)
	}
}
