import { expect, test } from "bun:test";

import type { Context } from "../../core/language.ts";
import { buildPlan, contextFor } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { tempRepo } from "../../core/testing.ts";
import { passes } from "../swift/shell.ts";
import {
	dotnetFormat,
	dotnetLint,
	dotnetRestore,
	dotnetTest,
	dotnetTypecheck
} from "./commands.ts";
import { guardDotnet } from "./guard.ts";
import { dotnet } from "./index.ts";

const hasDotnet = Bun.which("dotnet") !== null;

/**
 * A solution with a C# library on .NET 8, which adopt moves to 10, an F#
 * library, and an xUnit test project, each laid out as CSharpier and Fantomas
 * write them.
 */
const SOURCES = {
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
	"src/Greet/Greeting.cs": `namespace Greet;

/// <summary>Greetings for a name.</summary>
public static class Greeting
{
    /// <summary>The greeting for <paramref name="name"/>.</summary>
    public static string Of(string name) => $"Hello, {name}";
}
`,
	"src/Shout/Shout.fsproj": `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <Compile Include="Shout.fs" />
  </ItemGroup>
</Project>
`,
	"src/Shout/Shout.fs": `module Shout

/// The text in upper case.
let loud (text: string) = text.ToUpperInvariant()
`,
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
`
};

const selected = [{ language: dotnet, options: {} }];

const contextIn = (root: string): Context =>
	contextFor(root, selected, dotnet, {}, false);

/** A repo with {@link SOURCES} and `overrides`, adopted, with the pinned tools restored. */
const adopted = async (
	overrides: Record<string, string> = {}
): Promise<string> => {
	const root = await tempRepo({ ...SOURCES, ...overrides });
	const plan = await buildPlan(root, selected, { init: false });
	await applySteps(root, plan.steps);
	expect(await passes(root, ["dotnet", "tool", "restore"])).toBe(true);
	expect(await dotnetRestore.run(contextIn(root), [])).toBe(0);
	return root;
};

test.skipIf(!hasDotnet)(
	"an adopted C# and F# solution passes format, typecheck, lint, guard, and test",
	async () => {
		// Arrange
		const root = await adopted();
		const context = contextIn(root);

		// Act
		const formatted = await dotnetFormat.run(context, ["--check"]);
		const typechecked = await dotnetTypecheck.run(context, []);
		const linted = await dotnetLint.run(context, []);
		const guarded = await guardDotnet(context);
		const tested = await dotnetTest.run(context, []);

		// Assert
		expect(formatted).toBe(0);
		expect(typechecked).toBe(0);
		expect(linted).toBe(0);
		expect(guarded).toEqual([]);
		expect(tested).toBe(0);
	},
	600_000
);

test.skipIf(!hasDotnet)(
	"unformatted code and a code-style or FSharpLint finding fail format and lint",
	async () => {
		// Arrange
		const root = await adopted({
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
			"src/Shout/Shout.fs":
				"module Shout\n\n/// The text in upper case.\nlet Loud_Text (text:string) =   text.ToUpperInvariant()\n"
		});
		const context = contextIn(root);

		// Act
		const formatted = await dotnetFormat.run(context, ["--check"]);
		const linted = await dotnetLint.run(context, []);

		// Assert
		expect(formatted).toBe(1);
		expect(linted).toBe(1);
	},
	600_000
);

/** The test project on xUnit v3, a Microsoft.Testing.Platform application. */
const XUNIT_V3 = `<Project Sdk="Microsoft.NET.Sdk">
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
`;

test.skipIf(!hasDotnet).each([
	["Hello", 0],
	["Hi", 1]
])(
	"xUnit v3 tests run on the testing platform adopt opts into: %s exits %p",
	async (greeting, expected) => {
		// Arrange
		const root = await adopted({
			"tests/Greet.Tests/Greet.Tests.csproj": XUNIT_V3,
			"src/Greet/Greeting.cs": SOURCES["src/Greet/Greeting.cs"].replace(
				"Hello",
				greeting
			)
		});

		// Act
		const tested = await dotnetTest.run(contextIn(root), []);

		// Assert
		expect(tested).toBe(expected);
	},
	600_000
);

test.skipIf(!hasDotnet)(
	"an xUnit v3 project with no test yet passes, as VSTest passes one",
	async () => {
		// Arrange
		const root = await adopted({
			"tests/Greet.Tests/Greet.Tests.csproj": XUNIT_V3,
			"tests/Greet.Tests/GreetingTests.cs":
				"namespace Greet.Tests;\n\ninternal static class GreetingTests;\n"
		});

		// Act
		const tested = await dotnetTest.run(contextIn(root), []);

		// Assert
		expect(tested).toBe(0);
	},
	600_000
);

test.skipIf(!hasDotnet)(
	"a failing test fails the test command",
	async () => {
		// Arrange
		const root = await adopted({
			"src/Greet/Greeting.cs": SOURCES["src/Greet/Greeting.cs"].replace(
				"Hello",
				"Hi"
			)
		});

		// Act
		const tested = await dotnetTest.run(contextIn(root), []);

		// Assert
		expect(tested).toBe(1);
	},
	600_000
);
