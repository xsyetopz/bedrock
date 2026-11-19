package dotnet_test

import (
	"maps"
	"os/exec"
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
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
