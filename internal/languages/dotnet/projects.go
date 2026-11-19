package dotnet

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
)

const (
	// projectGlob matches the MSBuild project files of the .NET languages.
	projectGlob = "**/*.{csproj,fsproj}"
	// solutionGlob matches the root solutions: the XML `.slnx` of .NET 9 and
	// later, and the older `.sln`.
	solutionGlob = "*.{slnx,sln}"
	// csharpGlob and fsharpGlob match the sources the formatters read.
	csharpGlob = "**/*.{cs,csx}"
	fsharpGlob = "**/*.{fs,fsi,fsx}"
	// defaultJobs bounds MSBuild when `BEDROCK_DOTNET_JOBS` is unset.
	defaultJobs = 4
)

// jobs is how many MSBuild nodes, test applications, or project evaluations
// run at once: `BEDROCK_DOTNET_JOBS` when it is a positive integer, else 4,
// since each takes much of the machine's memory.
func jobs() int {
	count, err := strconv.Atoi(strings.TrimSpace(os.Getenv("BEDROCK_DOTNET_JOBS")))
	if err != nil || count <= 0 {
		return defaultJobs
	}

	return count
}

// msbuildBounds are the MSBuild switches every build takes: at most `jobs`
// nodes, and no node left alive afterwards, since an idle node holds its
// memory for minutes.
func msbuildBounds() []string {
	return []string{"-maxCpuCount:" + strconv.Itoa(jobs()), "-nodeReuse:false"}
}

// listMatching returns the files under the root that git tracks or would
// track and that match the glob.
func listMatching(ctx context.Context, c *language.Context, glob string) ([]string, error) {
	return fsutil.ListFiles(ctx, c.Runner, c.Root, func(path string) bool {
		return doublestar.MatchUnvalidated(glob, path)
	})
}

// projectFiles returns the project files under the root.
func projectFiles(ctx context.Context, c *language.Context) ([]string, error) {
	return listMatching(ctx, c, projectGlob)
}

// buildEntries are what `dotnet build` and `dotnet test` run on: each
// solution at the root, since a solution lists the projects that build
// together, or, without one, each project file. A `.sln` beside the `.slnx`
// of the same name is the same solution in the old format, so only the
// `.slnx` counts.
func buildEntries(ctx context.Context, c *language.Context) ([]string, error) {
	solutions, err := listMatching(ctx, c, solutionGlob)
	if err != nil {
		return nil, err
	}

	kept := slices.DeleteFunc(slices.Clone(solutions), func(path string) bool {
		return strings.HasSuffix(path, ".sln") && slices.Contains(solutions, path+"x")
	})
	if len(kept) > 0 {
		return kept, nil
	}

	return projectFiles(ctx, c)
}
