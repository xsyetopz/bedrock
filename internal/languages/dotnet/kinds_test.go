package dotnet_test

import (
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/plan"
)

// selectKind selects dotnet as `bedrock init dotnet:<kind>@<platform>` does.
func selectKind(t *testing.T, kind, platform string) plan.Selected {
	t.Helper()

	options := config.Options{"kind": kind}
	if platform != "" {
		options["platform"] = platform
	}

	return bedrocktest.Select(t, "dotnet", options)
}

// matches reports whether any path fits the glob; `*` stands for the project
// name, which comes from the repository's directory.
func matches(paths []string, glob string) bool {
	return slices.ContainsFunc(paths, func(p string) bool {
		ok, err := path.Match(glob, p)

		return err == nil && ok
	})
}

func TestInitPlansEachKindWithItsTestProject(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, kind, platform string
		want                 []string
	}{
		{"library", "library", "", []string{
			"src/*/*.csproj", "tests/*.Tests/*.Tests.csproj", "tests/*.Tests/GreetingTests.cs",
		}},
		{"console", "console", "", []string{
			"src/*.Cli/Program.cs", "tests/*.Cli.Tests/*.Cli.Tests.csproj",
		}},
		{"worker", "worker", "", []string{
			"src/*.Worker/Heartbeat.cs", "tests/*.Worker.Tests/HeartbeatTests.cs",
		}},
		{"webapi", "webapi", "", []string{
			"src/*.Api/Program.cs", "src/*.Api/Properties/launchSettings.json", "tests/*.Api.Tests/GreetingEndpointTests.cs",
		}},
		{"android", "android", "", []string{
			"src/*.Android/MainActivity.cs", "src/*.Android/AndroidManifest.xml", "tests/*.Core.Tests/GreetingTests.cs",
		}},
		{"avalonia", "avalonia", "", []string{
			"src/*.Ui/App.axaml", "src/*.Desktop/Program.cs", "tests/*.Ui.Tests/MainViewModelTests.cs",
		}},
		{"avalonia-mobile-and-web", "avalonia", "ios,android,wasm", []string{
			"src/*.iOS/Info.plist", "src/*.Android/MainActivity.cs", "src/*.Browser/wwwroot/index.html",
			"src/*.Desktop/Program.cs",
		}},
		{"apple-all", "apple", "ios,macos,maccatalyst,tvos", []string{
			"src/*.Core/Greeting.cs", "tests/*.Core.Tests/GreetingTests.cs",
			"src/*.iOS/SceneLifecycle.cs", "src/*.MacOS/AppLifecycle.cs",
			"src/*.MacCatalyst/Info.plist", "src/*.TvOS/*.csproj",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, map[string]string{})

			// Act
			result := bedrocktest.Plan(t, root, true, selectKind(t, tc.kind, tc.platform))

			// Assert
			paths := bedrocktest.Paths(result.Steps)
			for _, glob := range tc.want {
				if !matches(paths, glob) {
					t.Errorf("plan misses %s: %v", glob, paths)
				}
			}
		})
	}
}

func TestInitLeavesOutThePlatformsNotNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{})

	// Act
	result := bedrocktest.Plan(t, root, true, selectKind(t, "avalonia", "wasm"))

	// Assert
	paths := bedrocktest.Paths(result.Steps)
	if !matches(paths, "src/*.Browser/Program.cs") {
		t.Errorf("plan misses the browser project: %v", paths)
	}

	for _, glob := range []string{"src/*.iOS/*", "src/*.Android/*"} {
		if matches(paths, glob) {
			t.Errorf("plan holds %s: %v", glob, paths)
		}
	}
}

func TestInitFillsTheProjectNameAndAppID(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{})

	// Act
	result := bedrocktest.Plan(t, root, true, selectKind(t, "apple", "ios"))

	// Assert
	var plist, program string

	for _, s := range bedrocktest.Paths(result.Steps) {
		switch {
		case matches([]string{s}, "src/*.iOS/Info.plist"):
			plist = bedrocktest.Written(result.Steps, s)
		case matches([]string{s}, "src/*.Core/Greeting.cs"):
			program = bedrocktest.Written(result.Steps, s)
		}
	}

	if !strings.Contains(plist, "<string>com.example.app") {
		t.Errorf("Info.plist lacks the application id:\n%s", plist)
	}

	if strings.Contains(plist+program, "@@") || !strings.Contains(program, "namespace App") {
		t.Errorf("a placeholder is left or the namespace is not the project name:\n%s", program)
	}
}

func TestInitAppleNeedsAPlatform(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{})
	engine := bedrocktest.Engine(t)

	// Act
	_, err := engine.Build(t.Context(), root, []plan.Selected{selectKind(t, "apple", "")},
		plan.Options{Version: bedrocktest.Version, Dropped: nil, Init: true})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "needs a platform") {
		t.Errorf("error = %v", err)
	}
}

func TestSecondInitOfEachKindPlansNothing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ kind, platform string }{
		{"library", ""}, {"console", ""}, {"worker", ""}, {"webapi", ""}, {"android", ""},
		{"avalonia", "ios,android,wasm"}, {"apple", "ios,macos,maccatalyst,tvos"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := bedrocktest.Repo(t, map[string]string{})
			bedrocktest.Adopt(t, root, true, selectKind(t, tc.kind, tc.platform))

			// Act
			again := bedrocktest.Plan(t, root, false, selectKind(t, tc.kind, tc.platform))

			// Assert
			if len(again.Steps) != 0 {
				t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
			}
		})
	}
}
