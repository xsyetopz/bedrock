package dotnet_test

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
	"github.com/xsyetopz/bedrock/internal/config"
	"github.com/xsyetopz/bedrock/internal/execx"
	"github.com/xsyetopz/bedrock/internal/language"
)

const greeting = `namespace Greet;

/// <summary>Greetings for a name.</summary>
public static class Greeting
{
    /// <summary>The greeting for <paramref name="name"/>.</summary>
    public static string Of(string name) => $"Hello, {name}";
}
`

const xunitV3 = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <OutputType>Exe</OutputType>
    <IsPackable>false</IsPackable>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="xunit.v3" Version="4.0.3" />
  </ItemGroup>
  <ItemGroup>
    <ProjectReference Include="../../src/Greet/Greet.csproj" />
  </ItemGroup>
</Project>
`

// sources are a solution with a C# library on .NET 8, which adopt moves to
// 10, an F# library, and an xUnit test project, each laid out as CSharpier
// and Fantomas write them.
func sources() map[string]string {
	return map[string]string{
		"Demo.slnx": `<Solution>
  <Project Path="src/Greet/Greet.csproj" />
  <Project Path="src/Shout/Shout.fsproj" />
  <Project Path="tests/Greet.Tests/Greet.Tests.csproj" />
</Solution>
`,
		"src/Greet/Greet.csproj": `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>
`,
		"src/Greet/Greeting.cs": greeting,
		"src/Shout/Shout.fsproj": `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <Compile Include="Shout.fs" />
  </ItemGroup>
</Project>
`,
		"src/Shout/Shout.fs": "module Shout\n\n/// The text in upper case.\nlet loud (text: string) = text.ToUpperInvariant()\n",
		"tests/Greet.Tests/Greet.Tests.csproj": `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <IsPackable>false</IsPackable>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.14.1" />
    <PackageReference Include="xunit" Version="2.9.3" />
    <PackageReference Include="xunit.runner.visualstudio" Version="3.1.4" />
  </ItemGroup>
  <ItemGroup>
    <ProjectReference Include="../../src/Greet/Greet.csproj" />
  </ItemGroup>
</Project>
`,
		"tests/Greet.Tests/GreetingTests.cs": `using Xunit;

namespace Greet.Tests;

public class GreetingTests
{
    [Fact]
    public void GreetsByName() => Assert.Equal("Hello, you", Greeting.Of("you"));
}
`,
	}
}

func requireDotnet(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("e2e runs the real .NET SDK")
	}

	if _, err := exec.LookPath("dotnet"); err != nil {
		t.Skip("dotnet is not installed")
	}
}

// adopted is a repo with sources() and overrides, adopted, with the pinned
// tools and the packages restored.
func adopted(t *testing.T, overrides map[string]string) *language.Context {
	t.Helper()

	files := sources()
	maps.Copy(files, overrides)

	root := bedrocktest.Repo(t, files)
	bedrocktest.Adopt(t, root, false, selectDotnet(t))

	ctx := bedrocktest.Context(t, root, selectDotnet(t))

	restored, err := execx.Output(t.Context(), ctx.Runner,
		execx.Command{Dir: root, Name: "dotnet", Args: []string{"tool", "restore"}})
	if err != nil {
		t.Fatalf("dotnet tool restore: %v\n%s", err, restored)
	}

	if exit := runSub(t, ctx, "restore"); exit != 0 {
		t.Fatalf("restore exits %d", exit)
	}

	return ctx
}

func runSub(t *testing.T, ctx *language.Context, name string, args ...string) int {
	t.Helper()

	exit, err := subcommand(t, name).Run(t.Context(), ctx, args)
	if err != nil {
		t.Fatal(err)
	}

	return exit
}

func TestAnAdoptedCSharpAndFSharpSolutionPassesFormatTypecheckLintGuardAndTest(t *testing.T) {
	t.Parallel()
	requireDotnet(t)

	// Arrange
	ctx := adopted(t, nil)
	guarder, _ := bedrocktest.Language(t, "dotnet").(language.Guarder)

	// Act
	formatted := runSub(t, ctx, "format", "--check")
	typechecked := runSub(t, ctx, "typecheck")
	linted := runSub(t, ctx, "lint")
	tested := runSub(t, ctx, "test")

	found, err := guarder.Guard(t.Context(), ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	for name, exit := range map[string]int{"format": formatted, "typecheck": typechecked, "lint": linted, "test": tested} {
		if exit != 0 {
			t.Errorf("%s exits %d", name, exit)
		}
	}

	if len(found) != 0 {
		t.Errorf("guard findings: %v", found)
	}
}

func TestUnformattedCodeAndACodeStyleOrFSharpLintFindingFailFormatAndLint(t *testing.T) {
	t.Parallel()
	requireDotnet(t)

	// Arrange
	ctx := adopted(t, map[string]string{
		"src/Greet/Greeting.cs": `namespace Greet
{
    /// <summary>Greetings for a name.</summary>
    public static class Greeting
    {
        /// <summary>The greeting for <paramref name="name"/>.</summary>
        public static string Of(string name) { return "Hi, " + name; }
    }
}
`,
		"src/Shout/Shout.fs": "module Shout\n\n/// The text in upper case.\nlet Loud_Text (text:string) =   text.ToUpperInvariant()\n",
	})

	// Act
	formatted := runSub(t, ctx, "format", "--check")
	linted := runSub(t, ctx, "lint")

	// Assert
	if formatted != 1 || linted != 1 {
		t.Errorf("format exits %d and lint exits %d, want 1 and 1", formatted, linted)
	}
}

func TestXUnitV3TestsRunOnTheTestingPlatformAdoptOptsInto(t *testing.T) {
	t.Parallel()
	requireDotnet(t)

	for word, want := range map[string]int{"Hello": 0, "Hi": 1} {
		t.Run(word, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ctx := adopted(t, map[string]string{
				"tests/Greet.Tests/Greet.Tests.csproj": xunitV3,
				"src/Greet/Greeting.cs":                strings.Replace(greeting, "Hello", word, 1),
			})

			// Act
			tested := runSub(t, ctx, "test")

			// Assert
			if tested != want {
				t.Errorf("test exits %d, want %d", tested, want)
			}
		})
	}
}

func TestAnXUnitV3ProjectWithNoTestYetPassesAsVSTestPassesOne(t *testing.T) {
	t.Parallel()
	requireDotnet(t)

	// Arrange
	ctx := adopted(t, map[string]string{
		"tests/Greet.Tests/Greet.Tests.csproj": xunitV3,
		"tests/Greet.Tests/GreetingTests.cs":   "namespace Greet.Tests;\n\ninternal static class GreetingTests;\n",
	})

	// Act
	tested := runSub(t, ctx, "test")

	// Assert
	if tested != 0 {
		t.Errorf("test exits %d, want 0", tested)
	}
}

func TestAFailingTestFailsTheTestCommand(t *testing.T) {
	t.Parallel()
	requireDotnet(t)

	// Arrange
	ctx := adopted(t, map[string]string{"src/Greet/Greeting.cs": strings.Replace(greeting, "Hello", "Hi", 1)})

	// Act
	tested := runSub(t, ctx, "test")

	// Assert
	if tested != 1 {
		t.Errorf("test exits %d, want 1", tested)
	}
}

// workloads returns the installed .NET workload ids.
func workloads(t *testing.T) []string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "dotnet", "workload", "list").Output()
	if err != nil {
		t.Fatalf("dotnet workload list: %v", err)
	}

	var ids []string

	for line := range strings.SplitSeq(string(out), "\n") {
		if fields := strings.Fields(line); len(fields) > 1 && strings.Contains(fields[1], "/") {
			ids = append(ids, fields[0])
		}
	}

	return ids
}

// requireWorkloads skips the test unless every workload is installed.
func requireWorkloads(t *testing.T, need ...string) {
	t.Helper()

	have := workloads(t)
	for _, id := range need {
		if !slices.Contains(have, id) {
			t.Skipf("the %s workload is not installed (dotnet workload install %s)", id, id)
		}
	}
}

// androidAPI is the platform the android workload compiles against.
const androidAPI = "android-36"

// requireAndroidSDK skips the test unless an Android SDK with the platform
// the workload compiles against is installed, which `dotnet workload` does
// not provide and installing it means accepting the Android SDK licenses.
func requireAndroidSDK(t *testing.T) {
	t.Helper()

	roots := []string{
		os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"),
		"/opt/homebrew/share/android-commandlinetools",
	}
	for _, root := range roots {
		if root == "" {
			continue
		}

		if _, err := os.Stat(filepath.Join(root, "platforms", androidAPI, "android.jar")); err == nil {
			return
		}
	}

	t.Skipf("no Android SDK with %s (set ANDROID_HOME)", androidAPI)
}

// started is a repo a kind starts, with the pinned tools and the packages
// restored.
func started(t *testing.T, kind, platform string) *language.Context {
	t.Helper()

	options := config.Options{"kind": kind}
	if platform != "" {
		options["platform"] = platform
	}

	selected := bedrocktest.Select(t, "dotnet", options)
	root := bedrocktest.Repo(t, map[string]string{})
	bedrocktest.Adopt(t, root, true, selected)

	ctx := bedrocktest.Context(t, root, selected)

	restored, err := execx.Output(t.Context(), ctx.Runner,
		execx.Command{Dir: root, Name: "dotnet", Args: []string{"tool", "restore"}})
	if err != nil {
		t.Fatalf("dotnet tool restore: %v\n%s", err, restored)
	}

	if exit := runSub(t, ctx, "restore"); exit != 0 {
		t.Fatalf("restore exits %d", exit)
	}

	return ctx
}

// kindCase is one kind and platform with what building it needs besides the
// SDK: a workload, or a host OS for the Apple targets.
type kindCase struct {
	name, kind, platform string
	workloads            []string
	goos                 string
}

func kindCases() []kindCase {
	return []kindCase{
		{"console", "console", "", nil, ""},
		{"library", "library", "", nil, ""},
		{"webapi", "webapi", "", nil, ""},
		{"worker", "worker", "", nil, ""},
		{"avalonia", "avalonia", "", nil, ""},
		{"avalonia-wasm", "avalonia", "wasm", []string{"wasm-tools"}, ""},
		{"avalonia-ios", "avalonia", "ios", []string{"ios"}, "darwin"},
		{"avalonia-android", "avalonia", "android", []string{"android"}, ""},
		{"apple-ios", "apple", "ios", []string{"ios"}, "darwin"},
		{"apple-macos", "apple", "macos", []string{"macos"}, "darwin"},
		{"apple-maccatalyst", "apple", "maccatalyst", []string{"maccatalyst"}, "darwin"},
		{"apple-tvos", "apple", "tvos", []string{"tvos"}, "darwin"},
		{"android", "android", "", []string{"android"}, ""},
	}
}

func TestStartedKindsPassFormatTypecheckLintGuardAndTest(t *testing.T) {
	t.Parallel()
	requireDotnet(t)

	var buildSlot sync.Mutex

	for _, kase := range kindCases() {
		t.Run(kase.name, func(t *testing.T) {
			t.Parallel()
			checkKind(t, kase, &buildSlot)
		})
	}
}

// checkKind starts the kind and runs the recipes a change must pass.
func checkKind(t *testing.T, kase kindCase, buildSlot *sync.Mutex) {
	t.Helper()

	if kase.goos != "" && runtime.GOOS != kase.goos {
		t.Skipf("%s builds on %s only", kase.name, kase.goos)
	}

	requireWorkloads(t, kase.workloads...)

	if slices.Contains(kase.workloads, "android") {
		requireAndroidSDK(t)
	}

	// The kinds build one at a time: each build holds a toolchain and, for the
	// Apple and Android targets, gigabytes of memory.
	buildSlot.Lock()
	defer buildSlot.Unlock()

	// Arrange
	ctx := started(t, kase.kind, kase.platform)
	guarder, _ := bedrocktest.Language(t, "dotnet").(language.Guarder)

	// Act
	exits := map[string]int{
		"format":    runSub(t, ctx, "format", "--check"),
		"typecheck": runSub(t, ctx, "typecheck"),
		"lint":      runSub(t, ctx, "lint"),
		"test":      runSub(t, ctx, "test"),
	}

	found, err := guarder.Guard(t.Context(), ctx, false)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	for name, exit := range exits {
		if exit != 0 {
			t.Errorf("%s exits %d", name, exit)
		}
	}

	if len(found) != 0 {
		t.Errorf("guard findings: %v", found)
	}
}
