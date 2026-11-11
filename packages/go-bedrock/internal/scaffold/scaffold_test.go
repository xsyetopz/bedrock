package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/packages/go-bedrock/internal/scaffold"
)

func actions(plan []scaffold.Entry) map[string]scaffold.Action {
	out := make(map[string]scaffold.Action, len(plan))
	for _, entry := range plan {
		out[entry.Path] = entry.Action
	}

	return out
}

func TestNewWritesTheLayoutAndBoundedRecipes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	plan, err := scaffold.NewPlan(dir, "example.com/app", "app")
	if err != nil {
		t.Fatal(err)
	}

	err = scaffold.Apply(dir, plan)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"go.mod", "cmd/app/main.go", ".golangci.yml", ".just/go.just"} {
		_, err = os.Stat(filepath.Join(dir, path))
		if err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}

	recipes, err := os.ReadFile(filepath.Join(dir, ".just/go.just"))
	if err != nil {
		t.Fatal(err)
	}

	for _, bound := range []string{"GOMEMLIMIT", "GOFLAGS", "-p="} {
		if !strings.Contains(string(recipes), bound) {
			t.Errorf("recipes lack %s", bound)
		}
	}

	lint, err := os.ReadFile(filepath.Join(dir, ".golangci.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(lint), "concurrency:") || !strings.Contains(string(lint), "- example.com/app") {
		t.Error("lint config lacks run.concurrency or the module prefix")
	}
}

func TestAdoptKeepsTheProjectsLintConfigAndReplacesStaleRecipes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	for path, text := range map[string]string{
		"go.mod":        "module example.com/old\n\ngo 1.27\n",
		".golangci.yml": "version: \"2\"\n",
		".just/go.just": "stale\n",
	} {
		full := filepath.Join(dir, path)

		err := os.MkdirAll(filepath.Dir(full), 0o755)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(full, []byte(text), 0o644)
		if err != nil {
			t.Fatal(err)
		}
	}

	module, err := scaffold.ModulePath(dir)
	if err != nil || module != "example.com/old" {
		t.Fatalf("ModulePath = %q, %v", module, err)
	}

	plan, err := scaffold.AdoptPlan(dir, module)
	if err != nil {
		t.Fatal(err)
	}

	got := actions(plan)
	if got[".golangci.yml"] != scaffold.Keep || got[".just/go.just"] != scaffold.Replace {
		t.Errorf("actions = %v", got)
	}
}

func TestModulePathRejectsAModFileWithoutAModule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("go 1.27\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = scaffold.ModulePath(dir)
	if err == nil {
		t.Fatal("want an error")
	}
}
