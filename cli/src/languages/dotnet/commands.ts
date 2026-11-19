import { join } from "node:path";

import { Glob } from "bun";

import { listFiles } from "../../core/fs.ts";
import type { Subcommand } from "../../core/language.ts";
import { passes } from "../swift/shell.ts";
import { asRecord, readJsonc } from "../typescript/json.ts";
import { GLOBAL_JSON } from "./manifests.ts";
import { dotnetJobs, msbuildBounds } from "./msbuild.ts";
import {
	buildEntries,
	CSHARP_GLOB,
	FSHARP_GLOB,
	projectFiles,
} from "./projects.ts";
import { testProjects } from "./test-projects.ts";

/** A `dotnet <verb>` MSBuild run on one solution or project, within {@link msbuildBounds}. */
const msbuild = (
	verb: string,
	entry: string,
	flags: readonly string[] = [],
): string[] => ["dotnet", verb, entry, ...flags, ...msbuildBounds()];

/**
 * Whether every command passes, run one after another, since each build takes
 * much of the machine. All run even after one fails, so a run reports them all.
 */
const everyPasses = async (
	root: string,
	commands: readonly (readonly string[])[],
): Promise<boolean> => {
	const [first, ...rest] = commands;
	if (first === undefined) {
		return true;
	}
	const passed = await passes(root, first);
	return (await everyPasses(root, rest)) && passed;
};

/** The C# sources under `root` that git tracks or would track. */
const csharpFiles = (root: string): Promise<string[]> =>
	listFiles(root, new Glob(CSHARP_GLOB));

/** The F# sources under `root` that git tracks or would track. */
const fsharpFiles = (root: string): Promise<string[]> =>
	listFiles(root, new Glob(FSHARP_GLOB));

/** `bedrock dotnet restore`: restores the NuGet packages of each solution or project. */
export const dotnetRestore: Subcommand = {
	summary: "Restore the NuGet packages of each solution or project.",
	run: async ({ root }) => {
		const entries = await buildEntries(root);
		const commands = entries.map((entry) => msbuild("restore", entry));
		return (await everyPasses(root, commands)) ? 0 : 1;
	},
};

/**
 * The formatter runs for `bedrock dotnet format`: CSharpier over the root,
 * which takes the C# and MSBuild files and skips what `.gitignore` lists, and
 * Fantomas over the tracked F# files, since it reads only `.fantomasignore`
 * and would otherwise format the generated files under `obj/`.
 */
export const formatCommands = (
	check: boolean,
	hasCsharp: boolean,
	fsharp: readonly string[],
): string[][] => [
	...(hasCsharp
		? [["dotnet", "csharpier", check ? "check" : "format", "."]]
		: []),
	...(fsharp.length > 0
		? [["dotnet", "fantomas", ...(check ? ["check"] : []), ...fsharp]]
		: []),
];

/** Runs the formatters, or with `check` fails when a file is not formatted. */
const format = async (root: string, check: boolean): Promise<number> => {
	const [csharp, projects, fsharp] = await Promise.all([
		csharpFiles(root),
		projectFiles(root),
		fsharpFiles(root),
	]);
	const commands = formatCommands(
		check,
		csharp.length > 0 || projects.length > 0,
		fsharp,
	);
	return (await everyPasses(root, commands)) ? 0 : 1;
};

/**
 * `bedrock dotnet format [--check]`: CSharpier for C# and the MSBuild files,
 * Fantomas for F#; both read their layout from `.editorconfig`.
 */
export const dotnetFormat: Subcommand = {
	args: "[--check]",
	summary:
		"Format the C#, F#, and MSBuild files, or with --check fail if one is not.",
	run: ({ root }, args) => format(root, args.includes("--check")),
};

/**
 * `bedrock dotnet typecheck`: builds each solution or project with the
 * analyzers off, so it reports what the compilers find, with each warning an
 * error.
 */
export const dotnetTypecheck: Subcommand = {
	summary:
		"Build each solution or project without analyzers, warnings as errors.",
	run: async ({ root }) => {
		const entries = await buildEntries(root);
		const commands = entries.map((entry) =>
			msbuild("build", entry, ["-warnaserror", "-p:RunAnalyzers=false"]),
		);
		return (await everyPasses(root, commands)) ? 0 : 1;
	},
};

/**
 * The lint commands for `entries`: a full rebuild of each, since the analyzers
 * run only on what compiles, with the `.editorconfig` code style enforced and
 * each warning an error; then FSharpLint over each F# project, which reads a
 * `fsharplint.json` beside it when the project has one.
 */
export const lintCommands = (
	entries: readonly string[],
	fsprojs: readonly string[],
): string[][] => [
	...entries.map((entry) =>
		msbuild("build", entry, [
			"--no-incremental",
			"-warnaserror",
			"-p:EnforceCodeStyleInBuild=true",
		]),
	),
	...fsprojs.map((project) => ["dotnet", "fsharplint", "lint", project]),
];

/**
 * The fixes `dotnet format` applies for `entries` that hold C#: the
 * `.editorconfig` code style, then the analyzers' code fixes. It reads no F#,
 * so an F# project stays out.
 */
export const fixCommands = (entries: readonly string[]): string[][] =>
	entries.flatMap((entry) =>
		entry.endsWith(".fsproj")
			? []
			: [
					["dotnet", "format", "style", entry, "--severity", "warn"],
					["dotnet", "format", "analyzers", entry, "--severity", "warn"],
				],
	);

/**
 * `bedrock dotnet lint [--fix]`: the analyzer and code-style build, then
 * FSharpLint. With `--fix`, `dotnet format` first applies the fixes it can,
 * and the formatters lay the result out.
 */
export const dotnetLint: Subcommand = {
	args: "[--fix]",
	summary:
		"Build with the analyzers and code style as errors, then run FSharpLint.",
	run: async ({ root }, args) => {
		const [entries, projects, csharp] = await Promise.all([
			buildEntries(root),
			projectFiles(root),
			csharpFiles(root),
		]);
		if (args.includes("--fix")) {
			const fixes = csharp.length > 0 ? fixCommands(entries) : [];
			const fixed = await everyPasses(root, fixes);
			return fixed ? format(root, false) : 1;
		}
		const fsprojs = projects.filter((path) => path.endsWith(".fsproj"));
		return (await everyPasses(root, lintCommands(entries, fsprojs))) ? 0 : 1;
	},
};

/**
 * Whether `global.json` opts `dotnet test` into Microsoft.Testing.Platform,
 * which takes the solution or project as `--solution` or `--project`, where
 * VSTest takes it as an argument.
 */
const usesTestingPlatform = async (root: string): Promise<boolean> => {
	const json = await readJsonc(join(root, GLOBAL_JSON));
	return asRecord(json?.["test"])["runner"] === "Microsoft.Testing.Platform";
};

/**
 * The commands that test one solution or project. VSTest takes the MSBuild
 * switches of {@link msbuildBounds}. Microsoft.Testing.Platform hands unknown
 * switches to each test application, which exits 5 on them, so on it a
 * bounded build runs first and `dotnet test --no-build` then runs at most
 * `BEDROCK_DOTNET_JOBS` test applications at once.
 */
export const testCommands = (entry: string, platform: boolean): string[][] => {
	if (!platform) {
		return [["dotnet", "test", entry, ...msbuildBounds()]];
	}
	const flag = /\.slnx?$/u.test(entry) ? "--solution" : "--project";
	return [
		msbuild("build", entry),
		[
			"dotnet",
			"test",
			flag,
			entry,
			"--no-build",
			"--max-parallel-test-modules",
			String(dotnetJobs()),
			// Exit code 8 is a run with no test. VSTest passes a test project
			// that has none yet, so this runner does too; a failing test
			// still fails.
			"--ignore-exit-code",
			"8",
		],
	];
};

/**
 * The solutions or projects `dotnet test` runs: each solution, or without
 * one, each test project, since `dotnet test` on a product project finds no
 * tests and builds it for nothing. None when no project is a test project.
 */
const testEntries = async (root: string): Promise<string[]> => {
	const [entries, tests] = await Promise.all([
		buildEntries(root),
		testProjects(root),
	]);
	if (tests.length === 0) {
		return [];
	}
	return entries.some((entry) => /\.slnx?$/u.test(entry))
		? entries
		: tests.map(({ path }) => path);
};

/**
 * `bedrock dotnet test`: runs the tests of each solution, or each test
 * project, on the runner `global.json` names. A build failure skips that
 * entry's test run, since its test applications would be stale.
 */
export const dotnetTest: Subcommand = {
	summary: "Run the tests of each solution, or of each test project.",
	run: async ({ root }) => {
		const [entries, platform] = await Promise.all([
			testEntries(root),
			usesTestingPlatform(root),
		]);
		return (await testEach(root, entries, platform)) ? 0 : 1;
	},
};

/** Whether each command passes, stopping at the first that fails. */
const inTurn = async (
	root: string,
	commands: readonly (readonly string[])[],
): Promise<boolean> => {
	const [first, ...rest] = commands;
	if (first === undefined) {
		return true;
	}
	return (await passes(root, first)) && inTurn(root, rest);
};

/** Whether the tests of every entry pass; each runs even after one fails. */
const testEach = async (
	root: string,
	entries: readonly string[],
	platform: boolean,
): Promise<boolean> => {
	const [first, ...rest] = entries;
	if (first === undefined) {
		return true;
	}
	const passed = await inTurn(root, testCommands(first, platform));
	return (await testEach(root, rest, platform)) && passed;
};
