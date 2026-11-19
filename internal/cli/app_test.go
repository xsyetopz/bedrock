package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock"
	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/catalog"
	"github.com/xsyetopz/bedrock/internal/cli"
	"github.com/xsyetopz/bedrock/internal/execx"
)

// toolsPresent runs real programs but reports every tool as installed, so a
// test does not depend on which toolchains this machine has.
type toolsPresent struct{ execx.System }

func (toolsPresent) LookPath(string) bool { return true }

type run struct {
	stdout string
	stderr string
	code   int
}

// bedrockIn runs bedrock with args in dir, as a person at no terminal does
// in CI.
func bedrockIn(t *testing.T, dir string, args ...string) run {
	t.Helper()

	registry, err := catalog.Registry()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	app := &cli.App{
		Registry:  registry,
		Templates: bedrock.Templates(),
		Runner:    toolsPresent{execx.System{Stdout: &stdout, Stderr: &stderr}},
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Getenv: func(key string) string {
			if key == "CI" {
				return "true"
			}

			return ""
		},
		Version: bedrocktest.Version,
		Dir:     dir,
	}

	code := app.Run(context.Background(), args)

	return run{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func expectCode(t *testing.T, got run, want int) {
	t.Helper()

	if got.code != want {
		t.Fatalf("exit %d, want %d\nstdout:\n%s\nstderr:\n%s", got.code, want, got.stdout, got.stderr)
	}
}

func expectContains(t *testing.T, text, want string) {
	t.Helper()

	if !strings.Contains(text, want) {
		t.Errorf("output lacks %q:\n%s", want, text)
	}
}

func goRepo(t *testing.T) string {
	t.Helper()

	return bedrocktest.Repo(t, map[string]string{"go.mod": "module example.com/app\n\ngo 1.27\n"})
}

func TestNoCommandPrintsUsage(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, t.TempDir())

	// Assert
	expectCode(t, got, cli.ExitOK)
	expectContains(t, got.stdout, "Usage: bedrock <command>")
	expectContains(t, got.stdout, "go (kinds: command, library)")
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, t.TempDir(), "frobnicate")

	// Assert
	expectCode(t, got, cli.ExitUsage)
	expectContains(t, got.stdout, "Usage: bedrock <command>")
}

func TestUnknownFlagIsAUsageError(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, goRepo(t), "adopt", "--frobnicate")

	// Assert
	expectCode(t, got, cli.ExitUsage)
	expectContains(t, got.stderr, "--frobnicate")
}

func TestAdoptDryRunChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)

	// Act
	got := bedrockIn(t, root, "adopt", "--dry-run")

	// Assert
	expectCode(t, got, cli.ExitOK)
	expectContains(t, got.stdout, "Plan for "+root+":")
	expectContains(t, got.stdout, "justfile")

	if _, err := os.Stat(filepath.Join(root, "justfile")); err == nil {
		t.Error("a dry run wrote the `justfile`")
	}
}

func TestAdoptWithoutTerminalOrYesChangesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)

	// Act
	got := bedrockIn(t, root, "adopt", "--no-run")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stderr, "Pass `--yes` to apply")

	if _, err := os.Stat(filepath.Join(root, "justfile")); err == nil {
		t.Error("an unconfirmed adopt wrote the `justfile`")
	}
}

func TestAdoptDetectsTheLanguageAndSettles(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)

	// Act
	first := bedrockIn(t, root, "adopt", "--yes", "--no-run")
	bedrocktest.CommitAll(t, root)
	second := bedrockIn(t, root, "adopt", "--yes", "--no-run")

	// Assert
	expectCode(t, first, cli.ExitOK)
	expectContains(t, bedrocktest.Read(t, root, ".editorconfig"), "bedrock_languages = go")
	expectCode(t, second, cli.ExitOK)
	expectContains(t, second.stdout, "nothing to change")
}

func TestGuardPassesAFreshAdopt(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)
	expectCode(t, bedrockIn(t, root, "adopt", "--yes", "--no-run"), cli.ExitOK)

	// Act
	got := bedrockIn(t, root, "guard", "base")

	// Assert
	expectCode(t, got, cli.ExitOK)
}

func TestGuardReportsADriftedManagedFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)
	expectCode(t, bedrockIn(t, root, "adopt", "--yes", "--no-run"), cli.ExitOK)
	bedrocktest.Write(t, root, "justfile", "# changed by hand\n")

	// Act
	got := bedrockIn(t, root, "guard", "base")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stderr, "justfile: differs from what bedrock writes; run `bedrock update`")
}

func TestGuardRejectsAScopeNotAdopted(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)
	expectCode(t, bedrockIn(t, root, "adopt", "--yes", "--no-run"), cli.ExitOK)

	// Act
	got := bedrockIn(t, root, "guard", "cobol")

	// Assert
	expectCode(t, got, cli.ExitUsage)
	expectContains(t, got.stderr, "not adopted here: cobol. Scopes: base, go")
}

func TestUpdateKeepsALanguageThisBuildDoesNotKnow(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)
	expectCode(t, bedrockIn(t, root, "adopt", "--yes", "--no-run"), cli.ExitOK)

	config, err := os.ReadFile(filepath.Join(root, ".editorconfig"))
	if err != nil {
		t.Fatal(err)
	}

	recorded := strings.Replace(string(config), "bedrock_languages = go", "bedrock_languages = cobol, go", 1)
	bedrocktest.Write(t, root, ".editorconfig", recorded)

	// Act
	got := bedrockIn(t, root, "update", "--yes", "--no-run", "--allow-dirty")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stderr, "`.editorconfig` lists `cobol`, which this bedrock does not know")

	after, err := os.ReadFile(filepath.Join(root, ".editorconfig"))
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != recorded {
		t.Errorf("`.editorconfig` changed:\n%s", after)
	}
}

func TestAdoptRefusesToOverwriteUncommittedWork(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)
	bedrocktest.Write(t, root, ".gitignore", "/scratch\n")

	// Act
	got := bedrockIn(t, root, "adopt", "--yes", "--no-run")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stderr, "commit or stash these first")
	expectContains(t, got.stderr, "  .gitignore")
}

func TestAdoptOutsideARepositoryFails(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()

	// Act
	got := bedrockIn(t, root, "adopt", "go", "--yes")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stderr, "is not a git repository")
}

func TestAdoptRejectsATrailingCommaInKinds(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, goRepo(t), "adopt", "go:command,", "--dry-run")

	// Assert
	expectCode(t, got, cli.ExitUsage)
	expectContains(t, got.stderr, "remove the extra comma")
}

func TestUpdateTakesNoLanguages(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, goRepo(t), "update", ".", "go")

	// Assert
	expectCode(t, got, cli.ExitUsage)
	expectContains(t, got.stderr, "`bedrock update` takes no languages")
}

func TestUpdateNeedsAnAdoptedRepository(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, goRepo(t), "update", "--dry-run")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stderr, "records no `bedrock_version`")
}

func TestInitWritesTheKindsStarter(t *testing.T) {
	t.Parallel()

	// Arrange
	parent := t.TempDir()

	// Act
	got := bedrockIn(t, parent, "init", "app", "go:command", "--yes", "--no-run")

	// Assert
	expectCode(t, got, cli.ExitOK)

	if _, err := os.Stat(filepath.Join(parent, "app", ".git")); err != nil {
		t.Errorf("init started no git repository: %v", err)
	}

	expectContains(t, bedrocktest.Read(t, filepath.Join(parent, "app"), ".editorconfig"), "bedrock_languages = go")
}

func TestInitRefusesAFolderWithFiles(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, goRepo(t), "init", ".", "go", "--yes")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stderr, "is not empty")
}

func TestCleanDeletesHeldBackupsAndKeepsOthers(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"a.txt": "held\n"})
	bedrocktest.Write(t, root, "a.txt.bak", "held\n")
	bedrocktest.Write(t, root, "b.txt.bak", "only here\n")

	// Act
	got := bedrockIn(t, root, "clean")

	// Assert
	expectCode(t, got, cli.ExitFailed)
	expectContains(t, got.stdout, "  delete  a.txt.bak")
	expectContains(t, got.stdout, "  keep    b.txt.bak")

	if _, err := os.Stat(filepath.Join(root, "b.txt.bak")); err != nil {
		t.Error("clean deleted a backup git does not hold")
	}
}

func TestCleanForceDeletesEveryBackup(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{})
	bedrocktest.Write(t, root, "b.txt.bak", "only here\n")

	// Act
	got := bedrockIn(t, root, "clean", "--force")

	// Assert
	expectCode(t, got, cli.ExitOK)

	if _, err := os.Stat(filepath.Join(root, "b.txt.bak")); err == nil {
		t.Error("`--force` kept a backup")
	}
}

func TestCommitMsgChecksTheFile(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		message string
		code    int
	}{
		"a conventional message":        {message: "feat(cli): add a flag\n", code: cli.ExitOK},
		"a message with no type":        {message: "added a flag\n", code: cli.ExitFailed},
		"a subject with a missing type": {message: ": add a flag\n", code: cli.ExitFailed},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := t.TempDir()
			bedrocktest.Write(t, root, "COMMIT_EDITMSG", test.message)

			// Act
			got := bedrockIn(t, root, "commit-msg", "COMMIT_EDITMSG")

			// Assert
			expectCode(t, got, test.code)
		})
	}
}

func TestLanguagesStagedListsOnlyTouchedOnes(t *testing.T) {
	t.Parallel()

	// Arrange
	root := goRepo(t)
	expectCode(t, bedrockIn(t, root, "adopt", "--yes", "--no-run"), cli.ExitOK)
	bedrocktest.CommitAll(t, root)
	bedrocktest.Write(t, root, "notes.txt", "unrelated\n")
	gitAdd(t, root, "notes.txt")

	// Act
	unrelated := bedrockIn(t, root, "languages", "--staged")

	bedrocktest.Write(t, root, "main.go", "package main\n")
	gitAdd(t, root, "main.go")

	related := bedrockIn(t, root, "languages", "--staged")

	// Assert
	expectCode(t, unrelated, cli.ExitOK)

	if unrelated.stdout != "" {
		t.Errorf("an unrelated change lists %q", unrelated.stdout)
	}

	if related.stdout != "go\n" {
		t.Errorf("a Go change lists %q, want go", related.stdout)
	}
}

func gitAdd(t *testing.T, root, path string) {
	t.Helper()

	result, err := execx.System{}.Run(
		context.Background(),
		execx.Command{Dir: root, Name: "git", Args: []string{"add", path}},
	)
	if err != nil || !result.OK() {
		t.Fatalf("git add %s: %v %s", path, err, result.Stderr)
	}
}

func TestVersionPrintsTheRelease(t *testing.T) {
	t.Parallel()

	// Act
	got := bedrockIn(t, t.TempDir(), "version")

	// Assert
	expectCode(t, got, cli.ExitOK)

	if got.stdout != bedrocktest.Version+"\n" {
		t.Errorf("prints %q", got.stdout)
	}
}
