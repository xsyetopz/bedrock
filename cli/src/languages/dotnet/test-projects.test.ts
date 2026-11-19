import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import {
	dotnetTestFiles,
	isTestProject,
	testProjects,
	type TestRunner,
	testRunner
} from "./test-projects.ts";

describe("test projects", () => {
	// Each case matches what `dotnet msbuild -getProperty:IsTestProject
	// -getProperty:IsTestingPlatformApplication` reported for a restored
	// project on the .NET 10 SDK.
	test.each<[string, TestRunner | undefined]>([
		[
			'<PackageReference Include="Microsoft.NET.Test.Sdk" Version="18.0.0" /><PackageReference Include="xunit" />',
			"vstest"
		],
		['<PackageReference Include="xunit.v3" Version="4.0.3" />', "mtp"],
		[
			'<PackageReference Include="xunit.v3" /><PackageReference Include="xunit.runner.visualstudio" /><PackageReference Include="Microsoft.NET.Test.Sdk" />',
			"mtp"
		],
		['<PackageReference Include="xunit.v3.mtp-v2" />', "mtp"],
		[
			'<PackageReference Include="xunit.v3.mtp-off" /><PackageReference Include="xunit.runner.visualstudio" />',
			"vstest"
		],
		['<PackageReference Version="1.87.0" Include="TUnit" />', "mtp"],
		['<PackageReference Include="TUnit.Core" />', undefined],
		['<PackageReference Include="MSTest" Version="4.5.3" />', "vstest"],
		['<Project Sdk="MSTest.Sdk/4.0.0">', "mtp"],
		['<Project Sdk="MSTest.Sdk/4.0.0"><UseVSTest>true</UseVSTest>', "vstest"],
		[
			'<PackageReference Include="NUnit" /><PackageReference Include="NUnit3TestAdapter" />',
			"vstest"
		],
		[
			'<EnableNUnitRunner>true</EnableNUnitRunner><PackageReference Include="NUnit3TestAdapter" />',
			"mtp"
		],
		[
			"<EnableNUnitRunner>true</EnableNUnitRunner><EnableMSTestRunner>true</EnableMSTestRunner>",
			undefined
		],
		[
			'<EnableMSTestRunner>true</EnableMSTestRunner><PackageReference Include="MSTest" />',
			"mtp"
		],
		['<PackageReference Include="YoloDev.Expecto.TestSdk" />', "vstest"],
		["<IsTestProject>true</IsTestProject>", "vstest"],
		['<PackageReference Include="Expecto" />', undefined],
		['<PackageReference Include="xunit" />', undefined]
	])("%s runs on %p", (text, expected) => {
		expect(testRunner(text)).toBe(expected);
		expect(isTestProject(text)).toBe(expected !== undefined);
	});

	test("counts the files under a test project as test files", async () => {
		// Arrange
		const root = await tempRepo({
			"src/App/App.csproj": "<Project />\n",
			"tests/App.Tests/App.Tests.csproj":
				'<Project><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>\n'
		});

		// Act
		const files = await dotnetTestFiles(root, [
			"src/App/Lib.cs",
			"tests/App.Tests/LibTests.cs"
		]);

		// Assert
		expect([...files]).toEqual(["tests/App.Tests/LibTests.cs"]);
	});
});

describe("central test packages", () => {
	test.skipIf(Bun.which("dotnet") === null)(
		"takes a test package and runner property from Directory.Build.props, as MSBuild applies them",
		async () => {
			// Arrange
			const root = await tempRepo({
				"Directory.Build.props": `<Project>
  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <EnableNUnitRunner>true</EnableNUnitRunner>
  </PropertyGroup>
  <ItemGroup Condition="$(MSBuildProjectName.EndsWith('.Tests'))">
    <PackageReference Include="NUnit3TestAdapter" Version="6.3.1" />
  </ItemGroup>
</Project>
`,
				"src/App/App.csproj": '<Project Sdk="Microsoft.NET.Sdk" />\n',
				"tests/App.Tests/App.Tests.csproj":
					'<Project Sdk="Microsoft.NET.Sdk" />\n'
			});

			// Act
			const tests = await testProjects(root);

			// Assert
			expect(tests).toEqual([
				{ path: "tests/App.Tests/App.Tests.csproj", runner: "mtp" }
			]);
		},
		60_000
	);
});
