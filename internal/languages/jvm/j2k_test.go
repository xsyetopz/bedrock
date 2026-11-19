package jvm_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	jvmlang "github.com/xsyetopz/bedrock/internal/languages/jvm"
)

// recorder is a runner that records each command and answers `java -version`
// with success and the Gradle wrapper with convertCode.
type recorder struct {
	mu          sync.Mutex
	calls       []string
	convertCode int
}

func (r *recorder) Run(_ context.Context, command execx.Command) (execx.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, filepath.Base(command.Name)+" "+strings.Join(command.Args, " "))

	if slices.Contains(command.Args, "j2k") {
		return execx.Result{Stdout: nil, Stderr: nil, Code: r.convertCode}, nil
	}

	return execx.Result{Stdout: nil, Stderr: nil, Code: 0}, nil
}

func (*recorder) LookPath(string) bool { return true }

func runJ2K(t *testing.T, convertCode int, args ...string) (int, *recorder, string) {
	t.Helper()

	selected := jvm(t, "", "")
	c := bedrocktest.Context(t, t.TempDir(), selected)
	runner := &recorder{mu: sync.Mutex{}, calls: nil, convertCode: convertCode}

	var stderr bytes.Buffer

	c.Runner = runner
	c.Stderr = &stderr

	var subcommand language.Subcommand

	for _, each := range selected.Language.(language.SubcommandProvider).Subcommands() {
		if each.Name == "j2k" {
			subcommand = each
		}
	}

	code, err := subcommand.Run(t.Context(), c, args)
	if err != nil {
		t.Fatal(err)
	}

	return code, runner, stderr.String()
}

func TestJ2KWithoutPathsIsAUsageError(t *testing.T) {
	t.Parallel()

	// Act
	code, runner, stderr := runJ2K(t, 0)

	// Assert
	if code != 2 || len(runner.calls) != 0 || !strings.Contains(stderr, "takes the Java files or directories") {
		t.Errorf("code %d, calls %v, stderr %q", code, runner.calls, stderr)
	}
}

func TestJ2KConvertsThenFormats(t *testing.T) {
	t.Parallel()

	// Act
	code, runner, _ := runJ2K(t, 0, "src/A.java", "src/b")

	// Assert
	runs := runner.calls[len(runner.calls)-2:]
	if code != 0 || !strings.HasSuffix(runs[0], "--console=plain j2k --file=src/A.java --file=src/b") ||
		!strings.HasSuffix(runs[1], "--console=plain spotlessApply") {
		t.Errorf("code %d, calls %v", code, runner.calls)
	}
}

func TestJ2KReturnsAFailedConversionWithoutFormatting(t *testing.T) {
	t.Parallel()

	// Act
	code, runner, _ := runJ2K(t, 3, "src/A.java")

	// Assert
	if code != 3 ||
		slices.ContainsFunc(runner.calls, func(call string) bool { return strings.Contains(call, "spotlessApply") }) {
		t.Errorf("code %d, calls %v", code, runner.calls)
	}
}

func TestFindJDKPrefersAValidJavaHome(t *testing.T) {
	t.Parallel()

	// Arrange
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(home, "bin", "java"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	// Act
	found, ok := jvmlang.FindJDK(t.Context(), &recorder{mu: sync.Mutex{}, calls: nil, convertCode: 0}, home)

	// Assert
	if !ok || found != home {
		t.Errorf("found %q, %v", found, ok)
	}
}
