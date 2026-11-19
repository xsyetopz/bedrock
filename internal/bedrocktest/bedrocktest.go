// Package bedrocktest runs bedrock against real repositories in temporary
// directories, so tests check what an adopt writes and what a guard reports
// rather than how either is built.
package bedrocktest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/xsyetopz/bedrock"
	"github.com/xsyetopz/bedrock/internal/catalog"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/plan"
	"github.com/xsyetopz/bedrock/internal/step"
)

// Version is the release a test plan records.
const Version = "0.0.0-test"

// Engine returns a plan engine over every language and the embedded
// templates, running real programs with their output discarded.
func Engine(t *testing.T) *plan.Engine {
	t.Helper()

	registry, err := catalog.Registry()
	if err != nil {
		t.Fatal(err)
	}

	return &plan.Engine{
		Registry:  registry,
		Templates: bedrock.Templates(),
		Runner:    execx.System{Stdout: io.Discard, Stderr: io.Discard},
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	}
}

// Language returns the catalog's language with id.
func Language(t *testing.T, id string) language.Language {
	t.Helper()

	lang, ok := Engine(t).Registry.Find(id)
	if !ok {
		t.Fatalf("no language %s", id)
	}

	return lang
}

// Select pairs the language with id with options, as a run would.
func Select(t *testing.T, id string, options config.Options) plan.Selected {
	t.Helper()

	if options == nil {
		options = config.Options{}
	}

	return plan.Selected{Language: Language(t, id), Options: options}
}

// Repo returns a new git repository with files committed.
func Repo(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	git(t, root, "init", "--quiet")

	for path, content := range files {
		Write(t, root, path, content)
	}

	CommitAll(t, root)

	return root
}

// Write writes content to path under root, making its directories.
func Write(t *testing.T, root, path, content string) {
	t.Helper()

	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Read returns the content of path under root.
func Read(t *testing.T, root, path string) string {
	t.Helper()

	text, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}

	return string(text)
}

// CommitAll commits every change in root, as a person would before the
// next bedrock run.
func CommitAll(t *testing.T, root string) {
	t.Helper()
	git(t, root, "add", "-A")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false",
		"commit", "--quiet", "--allow-empty", "--no-verify", "-m", "test")
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()

	_, err := execx.Output(context.Background(), execx.System{Stdout: io.Discard, Stderr: io.Discard},
		execx.Command{Dir: root, Name: "git", Args: args})
	if err != nil {
		t.Fatal(err)
	}
}

// Plan plans an adopt of selected into root; init adds the kinds' starters.
func Plan(t *testing.T, root string, init bool, selected ...plan.Selected) plan.Plan {
	t.Helper()

	engine := Engine(t)

	result, err := engine.Build(t.Context(), root, selected, plan.Options{Version: Version, Dropped: nil, Init: init})
	if err != nil {
		t.Fatal(err)
	}

	return result
}

// Adopt plans an adopt of selected into root and applies it.
func Adopt(t *testing.T, root string, init bool, selected ...plan.Selected) plan.Plan {
	t.Helper()

	result := Plan(t, root, init, selected...)
	if err := step.Apply(root, result.Steps); err != nil {
		t.Fatal(err)
	}

	return result
}

// Context returns what the language in selected plans and checks with in root.
func Context(t *testing.T, root string, selected plan.Selected) *language.Context {
	t.Helper()

	return Engine(t).Context(root, []plan.Selected{selected}, selected, false)
}

// Paths returns the paths the steps change.
func Paths(steps []step.Step) []string {
	paths := make([]string, 0, len(steps))
	for _, s := range steps {
		paths = append(paths, s.Target())
	}

	return paths
}

// Written returns what the steps write to path, or "" when none does.
func Written(steps []step.Step, path string) string {
	for _, s := range steps {
		if write, ok := s.(step.Write); ok && write.Path == path {
			return string(write.Content)
		}
	}

	return ""
}
