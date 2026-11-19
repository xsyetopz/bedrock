import type { Language } from "../../core/language.ts";
import {
	dotnetFormat,
	dotnetLint,
	dotnetRestore,
	dotnetTest,
	dotnetTypecheck,
} from "./commands.ts";
import { guardDotnet } from "./guard.ts";
import { planDotnet } from "./plan.ts";
import { dotnetTestFiles } from "./test-projects.ts";

/**
 * C# and F# on .NET 10, alone or beside other languages: CSharpier and
 * Fantomas for layout, the SDK analyzers with the `.editorconfig` code style
 * and FSharpLint for lint, a build of each solution or project, and
 * `dotnet test`.
 */
export const dotnet: Language = {
	id: "dotnet",
	block: "dotnet-bedrock",
	aliases: ["dotnet", ".net", "csharp", "c#", "cs", "fsharp", "f#"],
	markers: [
		".just/dotnet.just",
		"global.json",
		"*.{sln,slnx}",
		"**/*.{csproj,fsproj}",
	],
	tools: {
		dotnet:
			"the .NET 10 SDK from https://dotnet.microsoft.com/download, such as brew install dotnet@10",
	},
	sources: [
		"**/*.{cs,csx,fs,fsi,fsx}",
		"**/*.{csproj,fsproj,props,targets}",
		"*.{sln,slnx}",
		"global.json",
		".config/dotnet-tools.json",
		"dotnet-tools.json",
		".editorconfig",
	],
	kinds: [],
	variables: () => Promise.resolve({}),
	plan: planDotnet,
	guard: (context) => guardDotnet(context),
	// The SDK analyzers and FSharpLint, as configured, bound no file, function,
	// complexity, or parameter size, so lizard measures both languages.
	lizard: ["csharp", "fsharp"],
	testFiles: dotnetTestFiles,
	subcommands: {
		restore: dotnetRestore,
		format: dotnetFormat,
		typecheck: dotnetTypecheck,
		lint: dotnetLint,
		test: dotnetTest,
	},
};
