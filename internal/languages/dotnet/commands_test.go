package dotnet_test

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

// recorder runs git for real, so file listing behaves as in a repository,
// and answers each dotnet run with an exit code without starting the SDK.
type recorder struct {
	execx.System

	mu   sync.Mutex
	runs []string
	code int
}

func (r *recorder) Run(ctx context.Context, command execx.Command) (execx.Result, error) {
	if command.Name != "dotnet" {
		return r.System.Run(ctx, command)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.runs = append(r.runs, strings.Join(command.Args, " "))

	return execx.Result{Code: r.code}, nil
}

func (r *recorder) LookPath(string) bool { return true }

func subcommand(t *testing.T, name string) language.Subcommand {
	t.Helper()

	provider, ok := bedrocktest.Language(t, "dotnet").(language.SubcommandProvider)
	if !ok {
		t.Fatal("dotnet has no subcommands")
	}

	for _, sub := range provider.Subcommands() {
		if sub.Name == name {
			return sub
		}
	}

	t.Fatalf("no subcommand %s", name)

	return language.Subcommand{}
}

func run(t *testing.T, files map[string]string, code int, name string, args ...string) (int, []string) {
	t.Helper()

	root := bedrocktest.Repo(t, files)
	ctx := bedrocktest.Context(t, root, selectDotnet(t))
	fake := &recorder{code: code}
	fake.Stdout, fake.Stderr = io.Discard, io.Discard
	ctx.Runner = fake
	ctx.Stderr = io.Discard

	exit, err := subcommand(t, name).Run(t.Context(), ctx, args)
	if err != nil {
		t.Fatal(err)
	}

	return exit, fake.runs
}

func TestRestoreTakesTheRootSolutionsTheSlnxOverASameNamedSln(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"App.sln": "", "App.slnx": "<Solution />\n", "src/App/App.csproj": "<Project />\n"}

	// Act
	exit, runs := run(t, files, 0, "restore")

	// Assert
	want := []string{"restore App.slnx -maxCpuCount:4 -nodeReuse:false"}
	if exit != 0 || !slices.Equal(runs, want) {
		t.Errorf("exit %d, runs %q, want %q", exit, runs, want)
	}
}

func TestTypecheckBuildsEachProjectWithoutASolutionAndFailsWhenABuildFails(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"src/B/B.fsproj": "<Project />\n", "src/A/A.csproj": "<Project />\n"}

	// Act
	exit, runs := run(t, files, 1, "typecheck")

	// Assert
	want := []string{
		"build src/A/A.csproj -warnaserror -p:RunAnalyzers=false -maxCpuCount:4 -nodeReuse:false",
		"build src/B/B.fsproj -warnaserror -p:RunAnalyzers=false -maxCpuCount:4 -nodeReuse:false",
	}
	if exit != 1 || !slices.Equal(runs, want) {
		t.Errorf("exit %d, runs %q, want %q", exit, runs, want)
	}
}

func TestFormatChecksCSharpWithCSharpierAndFSharpWithFantomas(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"A.cs": "", "B.fs": ""}

	// Act
	exit, runs := run(t, files, 0, "format", "--check")

	// Assert
	want := []string{"csharpier check .", "fantomas check B.fs"}
	if exit != 0 || !slices.Equal(runs, want) {
		t.Errorf("exit %d, runs %q, want %q", exit, runs, want)
	}
}

func TestLintBuildsWithTheAnalyzersThenRunsFSharpLintOnEachFSharpProject(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"src/App/App.fsproj": "<Project />\n"}

	// Act
	exit, runs := run(t, files, 0, "lint")

	// Assert
	want := []string{
		"build src/App/App.fsproj --no-incremental -warnaserror -p:EnforceCodeStyleInBuild=true " +
			"-maxCpuCount:4 -nodeReuse:false",
		"fsharplint lint src/App/App.fsproj",
	}
	if exit != 0 || !slices.Equal(runs, want) {
		t.Errorf("exit %d, runs %q, want %q", exit, runs, want)
	}
}

func TestLintFixAppliesCodeStyleFixesToCSharpOnlyThenFormats(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"src/A/A.csproj": "<Project />\n",
		"src/A/A.cs":     "",
		"src/B/B.fsproj": "<Project />\n",
	}

	// Act
	exit, runs := run(t, files, 0, "lint", "--fix")

	// Assert
	want := []string{
		"format style src/A/A.csproj --severity warn",
		"format analyzers src/A/A.csproj --severity warn",
		"csharpier format .",
	}
	if exit != 0 || !slices.Equal(runs, want) {
		t.Errorf("exit %d, runs %q, want %q", exit, runs, want)
	}
}

func TestTestRunsOnlyTheTestProjectsOnVSTestWithoutASolution(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"src/App/App.csproj": "<Project />\n",
		"tests/A/A.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup>` +
			`<PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>` + "\n",
	}

	// Act
	exit, runs := run(t, files, 0, "test")

	// Assert
	want := []string{"test tests/A/A.csproj -maxCpuCount:4 -nodeReuse:false"}
	if exit != 0 || !slices.Equal(runs, want) {
		t.Errorf("exit %d, runs %q, want %q", exit, runs, want)
	}
}

func TestTestOnTheTestingPlatformBuildsFirstAndIgnoresAnEmptyRun(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{
		"App.slnx":    "<Solution />\n",
		"global.json": `{ "test": { "runner": "Microsoft.Testing.Platform" } }` + "\n",
		"tests/A/A.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup>` +
			`<PackageReference Include="xunit.v3" /></ItemGroup></Project>` + "\n",
	}

	// Act
	exit, runs := run(t, files, 0, "test")

	// Assert
	want := []string{
		"build App.slnx -maxCpuCount:4 -nodeReuse:false",
		"test --solution App.slnx --no-build --max-parallel-test-modules 4 --ignore-exit-code 8",
	}
	if exit != 0 || !slices.Equal(runs, want) {
		t.Errorf("exit %d, runs %q, want %q", exit, runs, want)
	}
}

func TestTestRunsNothingWhenNoProjectHasTests(t *testing.T) {
	t.Parallel()

	// Arrange
	files := map[string]string{"src/App/App.csproj": "<Project />\n"}

	// Act
	exit, runs := run(t, files, 0, "test")

	// Assert
	if exit != 0 || len(runs) != 0 {
		t.Errorf("exit %d, runs %q", exit, runs)
	}
}
