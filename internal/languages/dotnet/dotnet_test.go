package dotnet_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/plan"
)

const csproj = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <LangVersion>12.0</LangVersion>
  </PropertyGroup>
</Project>
`

func selectDotnet(t *testing.T) plan.Selected {
	t.Helper()

	return bedrocktest.Select(t, "dotnet", nil)
}

func notesText(result plan.Plan) string { return strings.Join(result.Notes, "\n") }

func TestAdoptWritesRecipesWorkflowPinsToolsAndStarterProps(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"src/App/App.csproj": csproj})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	for _, want := range []string{
		".just/dotnet.just",
		".github/workflows/dotnet.yml",
		"global.json",
		".config/dotnet-tools.json",
		"Directory.Build.props",
		"src/App/App.csproj",
	} {
		if !slices.Contains(paths, want) {
			t.Errorf("plan misses %s: %v", want, paths)
		}
	}
}

func TestAdoptRetargetsToNet10AndNotesTheOldLangVersion(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"src/App/App.csproj": csproj})

	// Act
	result := bedrocktest.Adopt(t, root, false, selectDotnet(t))

	// Assert
	if project := bedrocktest.Read(t, root, "src/App/App.csproj"); !strings.Contains(
		project, "<TargetFramework>net10.0</TargetFramework>") {
		t.Errorf("project:\n%s", project)
	}

	if !strings.Contains(notesText(result), "`src/App/App.csproj`: `LangVersion` 12") {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestAdoptRetargetsFrameworkListsAndKeepsOlderLibrariesAndProperties(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj": "<Project>\n<TargetFrameworks>net8.0;net9.0-windows;netstandard2.0;net8.0</TargetFrameworks>\n" +
			"<TargetFramework>$(AppTarget)</TargetFramework>\n</Project>\n",
		"src/Old/Old.fsproj": "<Project><TargetFramework>netcoreapp3.1</TargetFramework></Project>\n",
		"src/New/New.csproj": "<Project><TargetFramework>net48</TargetFramework></Project>\n",
	})

	// Act
	bedrocktest.Adopt(t, root, false, selectDotnet(t))

	// Assert
	want := map[string]string{
		"src/App/App.csproj": "<Project>\n<TargetFrameworks>net10.0;net10.0-windows;netstandard2.0</TargetFrameworks>\n" +
			"<TargetFramework>$(AppTarget)</TargetFramework>\n</Project>\n",
		"src/Old/Old.fsproj": "<Project><TargetFramework>net10.0</TargetFramework></Project>\n",
		"src/New/New.csproj": "<Project><TargetFramework>net48</TargetFramework></Project>\n",
	}
	for path, content := range want {
		if got := bedrocktest.Read(t, root, path); got != content {
			t.Errorf("%s:\n%s\nwant:\n%s", path, got, content)
		}
	}
}

func TestSecondAdoptPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{"src/App/App.csproj": csproj})
	bedrocktest.Adopt(t, root, false, selectDotnet(t))

	// Act
	again := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestAdoptKeepsARootToolManifestThatDotnetRewroteInItsOwnLayout(t *testing.T) {
	t.Parallel()

	// Arrange
	manifest := `{
  "version": 1,
  "isRoot": true,
  "tools": {
    "csharpier": { "version": "1.4.0", "commands": ["csharpier"], "rollForward": false },
    "fantomas": { "version": "8.1.3", "commands": ["fantomas"], "rollForward": false },
    "dotnet-fsharplint": { "version": "0.27.2", "commands": ["dotnet-fsharplint"], "rollForward": false }
  }
}
`
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.fsproj": "<Project Sdk=\"Microsoft.NET.Sdk\" />\n",
		"dotnet-tools.json":  manifest,
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	if slices.Contains(paths, "dotnet-tools.json") || slices.Contains(paths, ".config/dotnet-tools.json") {
		t.Errorf("plan rewrites the manifest: %v", paths)
	}
}

func TestAdoptMovesPinnedToolsInAProjectManifestAndKeepsTheOthers(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj": csproj,
		"dotnet-tools.json": `{
  "version": 1,
  "tools": {
    "csharpier": { "version": "0.1.0", "commands": ["csharpier"] },
    "dotnet-ef": { "version": "9.0.0", "commands": ["dotnet-ef"] }
  }
}
`,
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	manifest := bedrocktest.Written(result.Steps, "dotnet-tools.json")

	for _, want := range []string{`"version": "1.4.0"`, `"dotnet-ef"`, `"fantomas"`, `"rollForward": false`} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest lacks %s:\n%s", want, manifest)
		}
	}

	if strings.Contains(manifest, ",\n\t}") || strings.Contains(manifest, ",\n\t\t}") {
		t.Errorf("manifest has a trailing comma:\n%s", manifest)
	}
}

func TestAdoptNotesTheSettingsAProjectsOwnDirectoryBuildPropsLacks(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj": csproj,
		"Directory.Build.props": "<Project>\n  <PropertyGroup>\n    <Nullable>enable</Nullable>\n" +
			"  </PropertyGroup>\n</Project>\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if slices.Contains(bedrocktest.Paths(result.Steps), "Directory.Build.props") {
		t.Error("plan rewrites the project's `Directory.Build.props`")
	}

	if !strings.Contains(notesText(result),
		"`Directory.Build.props`: kept as yours; bedrock's starter also sets `<AnalysisLevel>") {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestAdoptNotesAHandWrittenWorkflowItReplacesAndNotItsOwn(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj":           csproj,
		".github/workflows/dotnet.yml": "name: .NET\n",
	})

	// Act
	first := bedrocktest.Adopt(t, root, false, selectDotnet(t))
	again := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if !strings.Contains(notesText(first), "`.github/workflows/dotnet.yml`: replaced with bedrock's") {
		t.Errorf("first notes: %v", first.Notes)
	}

	if strings.Contains(notesText(again), "dotnet.yml") {
		t.Errorf("second notes: %v", again.Notes)
	}
}

func TestAdoptNotesAnotherWorkflowThatRunsDotnet(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj":       csproj,
		".github/workflows/ci.yml": "jobs:\n  build:\n    steps:\n      - run: dotnet test\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if !strings.Contains(notesText(result), "`.github/workflows/ci.yml`: sets up .NET or runs `dotnet`") {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestAdoptOptsGlobalJSONIntoTheTestingPlatformForXUnitV3AndTUnitTests(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"tests/A/A.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="xunit.v3" /></ItemGroup></Project>` + "\n",
		"tests/B/B.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="TUnit" /></ItemGroup></Project>` + "\n",
	})

	// Act
	bedrocktest.Adopt(t, root, false, selectDotnet(t))
	again := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if global := bedrocktest.Read(t, root, "global.json"); !strings.Contains(
		global, `"runner": "Microsoft.Testing.Platform"`) {
		t.Errorf("global.json:\n%s", global)
	}

	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

func TestAdoptNotesTestProjectsSplitAcrossBothRunnersAndKeepsVSTest(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"tests/A/A.csproj": `<Project Sdk="MSTest.Sdk/4.5.3"><PropertyGroup><TargetFramework>net10.0</TargetFramework></PropertyGroup></Project>` + "\n",
		"tests/B/B.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>` + "\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if global := bedrocktest.Written(
		result.Steps,
		"global.json",
	); strings.Contains(
		global,
		"Microsoft.Testing.Platform",
	) {
		t.Errorf("global.json opts into the testing platform:\n%s", global)
	}

	want := "`global.json`: tests/A/A.csproj run on Microsoft.Testing.Platform and tests/B/B.csproj on VSTest"
	if !strings.Contains(notesText(result), want) {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestAdoptNotesAVSTestProjectBesideATestingPlatformRunner(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"global.json":      `{ "sdk": { "version": "10.0.100" }, "test": { "runner": "Microsoft.Testing.Platform" } }` + "\n",
		"tests/B/B.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="xunit.runner.visualstudio" /></ItemGroup></Project>` + "\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if !strings.Contains(
		notesText(result),
		"`test.runner` is `Microsoft.Testing.Platform`, but tests/B/B.csproj run on VSTest",
	) {
		t.Errorf("notes: %v", result.Notes)
	}
}

func TestAdoptMovesAnOlderSDKToTenAndKeepsTheOtherKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj": csproj,
		"global.json":        `{ "sdk": { "version": "8.0.100" }, "msbuild-sdks": { "X": "1.0" } } // mine` + "\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	want := "{\n\t\"sdk\": {\n\t\t\"version\": \"10.0.100\",\n\t\t\"rollForward\": \"latestFeature\"\n\t},\n" +
		"\t\"msbuild-sdks\": {\n\t\t\"X\": \"1.0\"\n\t}\n}\n"
	if got := bedrocktest.Written(result.Steps, "global.json"); got != want {
		t.Errorf("global.json:\n%s\nwant:\n%s", got, want)
	}
}

func TestAdoptLeavesAnSDKOnTenOrLater(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"src/App/App.csproj": csproj,
		"global.json":        `{ "sdk": { "version": "11.0.100" } }` + "\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	if slices.Contains(bedrocktest.Paths(result.Steps), "global.json") {
		t.Error("plan rewrites a `global.json` already on .NET 10 or later")
	}
}

func TestAdoptAddsTheTestingPlatformRunnerBesideACurrentSDK(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"global.json":      `{ "sdk": { "version": "10.0.100" } }` + "\n",
		"tests/A/A.csproj": `<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="TUnit" /></ItemGroup></Project>` + "\n",
	})

	// Act
	result := bedrocktest.Plan(t, root, false, selectDotnet(t))

	// Assert
	want := "{\n\t\"sdk\": {\n\t\t\"version\": \"10.0.100\"\n\t},\n\t\"test\": {\n\t\t\"runner\": \"Microsoft.Testing.Platform\"\n\t}\n}\n"
	if got := bedrocktest.Written(result.Steps, "global.json"); got != want {
		t.Errorf("global.json:\n%s\nwant:\n%s", got, want)
	}
}

func TestAdoptBesideGoWritesBothLanguagesRecipes(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		"go.mod":             "module m\n\ngo 1.27\n",
		"src/App/App.csproj": csproj,
	})

	// Act
	result := bedrocktest.Plan(t, root, false,
		bedrocktest.Select(t, "go", nil), selectDotnet(t))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	for _, want := range []string{".just/dotnet.just", ".just/go.just"} {
		if !slices.Contains(paths, want) {
			t.Errorf("plan misses %s: %v", want, paths)
		}
	}
}
