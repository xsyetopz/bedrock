import { describe, expect, test } from "bun:test";
import { join } from "node:path";

import { buildPlan } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { tempRepo } from "../../core/testing.ts";
import { typescript } from "../typescript/index.ts";
import { dotnet } from "./index.ts";

const selected = [{ language: dotnet, options: {} }];

const CSPROJ = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <LangVersion>12.0</LangVersion>
  </PropertyGroup>
</Project>
`;

describe("adopt", () => {
	test("writes the recipes, workflow, SDK pin, tool manifest, and starter props", async () => {
		// Arrange
		const root = await tempRepo({ "src/App/App.csproj": CSPROJ });

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).toContain(".just/dotnet.just");
		expect(paths).toContain(".github/workflows/dotnet.yml");
		expect(paths).toContain("global.json");
		expect(paths).toContain(".config/dotnet-tools.json");
		expect(paths).toContain("Directory.Build.props");
		expect(paths).toContain("src/App/App.csproj");
	});

	test("retargets to net10.0 and notes the old LangVersion", async () => {
		// Arrange
		const root = await tempRepo({ "src/App/App.csproj": CSPROJ });

		// Act
		const plan = await buildPlan(root, selected, { init: false });
		await applySteps(root, plan.steps);

		// Assert
		const project = await Bun.file(join(root, "src/App/App.csproj")).text();
		expect(project).toContain("<TargetFramework>net10.0</TargetFramework>");
		expect(plan.notes.join("\n")).toContain(
			"src/App/App.csproj: LangVersion 12",
		);
	});

	test("a second adopt plans no steps", async () => {
		// Arrange
		const root = await tempRepo({ "src/App/App.csproj": CSPROJ });
		const first = await buildPlan(root, selected, { init: false });
		await applySteps(root, first.steps);

		// Act
		const again = await buildPlan(root, selected, { init: false });

		// Assert
		expect(again.steps).toEqual([]);
	});

	test("keeps a root tool manifest that dotnet rewrote in its own layout", async () => {
		// Arrange
		const manifest = {
			version: 1,
			isRoot: true,
			tools: {
				csharpier: {
					version: "1.4.0",
					commands: ["csharpier"],
					rollForward: false,
				},
				fantomas: {
					version: "8.1.3",
					commands: ["fantomas"],
					rollForward: false,
				},
				"dotnet-fsharplint": {
					version: "0.27.2",
					commands: ["dotnet-fsharplint"],
					rollForward: false,
				},
			},
		};
		const root = await tempRepo({
			"src/App/App.fsproj": '<Project Sdk="Microsoft.NET.Sdk" />\n',
			"dotnet-tools.json": `${JSON.stringify(manifest, undefined, 2)}\n`,
		});

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).not.toContain("dotnet-tools.json");
		expect(paths).not.toContain(".config/dotnet-tools.json");
	});

	test("notes the settings a project's own Directory.Build.props lacks", async () => {
		// Arrange
		const root = await tempRepo({
			"src/App/App.csproj": CSPROJ,
			"Directory.Build.props":
				"<Project>\n  <PropertyGroup>\n    <Nullable>enable</Nullable>\n  </PropertyGroup>\n</Project>\n",
		});

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).not.toContain("Directory.Build.props");
		expect(plan.notes.join("\n")).toContain(
			"Directory.Build.props: kept as yours; bedrock's starter also sets <AnalysisLevel>",
		);
	});

	test("notes a hand-written workflow it replaces, and not its own", async () => {
		// Arrange
		const root = await tempRepo({
			"src/App/App.csproj": CSPROJ,
			".github/workflows/dotnet.yml": "name: .NET\n",
		});

		// Act
		const first = await buildPlan(root, selected, { init: false });
		await applySteps(root, first.steps);
		const again = await buildPlan(root, selected, { init: false });

		// Assert
		expect(first.notes.join("\n")).toContain(
			".github/workflows/dotnet.yml: replaced with bedrock's",
		);
		expect(again.notes.join("\n")).not.toContain("dotnet.yml");
	});

	test("notes another workflow that runs dotnet", async () => {
		// Arrange
		const root = await tempRepo({
			"src/App/App.csproj": CSPROJ,
			".github/workflows/ci.yml":
				"jobs:\n  build:\n    steps:\n      - run: dotnet test\n",
		});

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		expect(plan.notes.join("\n")).toContain(
			".github/workflows/ci.yml: sets up .NET or runs dotnet",
		);
	});

	test("opts global.json into the testing platform for xUnit v3 and TUnit tests", async () => {
		// Arrange
		const root = await tempRepo({
			"tests/A/A.csproj":
				'<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="xunit.v3" /></ItemGroup></Project>\n',
			"tests/B/B.csproj":
				'<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="TUnit" /></ItemGroup></Project>\n',
		});

		// Act
		const first = await buildPlan(root, selected, { init: false });
		await applySteps(root, first.steps);
		const again = await buildPlan(root, selected, { init: false });

		// Assert
		const json: unknown = await Bun.file(join(root, "global.json")).json();
		expect(json).toMatchObject({
			test: { runner: "Microsoft.Testing.Platform" },
		});
		expect(again.steps).toEqual([]);
	});

	test("notes test projects split across both runners and keeps VSTest", async () => {
		// Arrange
		const root = await tempRepo({
			"tests/A/A.csproj":
				'<Project Sdk="MSTest.Sdk/4.5.3"><PropertyGroup><TargetFramework>net10.0</TargetFramework></PropertyGroup></Project>\n',
			"tests/B/B.csproj":
				'<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>\n',
		});

		// Act
		const plan = await buildPlan(root, selected, { init: false });

		// Assert
		const globalJson = plan.steps.find((step) => step.path === "global.json");
		expect(
			String(globalJson?.kind === "write" && globalJson.content),
		).not.toContain("Microsoft.Testing.Platform");
		expect(plan.notes.join("\n")).toContain(
			"global.json: tests/A/A.csproj run on Microsoft.Testing.Platform and tests/B/B.csproj on VSTest",
		);
	});

	test("adopts .NET beside TypeScript in one repo", async () => {
		// Arrange
		const root = await tempRepo({
			"package.json": '{ "name": "app" }\n',
			"src/App/App.csproj": CSPROJ,
		});
		const both = [
			{ language: typescript, options: {} },
			{ language: dotnet, options: {} },
		];

		// Act
		const plan = await buildPlan(root, both, { init: false });

		// Assert
		const paths = plan.steps.map((step) => step.path);
		expect(paths).toContain(".just/dotnet.just");
		expect(paths).toContain(".just/typescript.just");
	});
});
