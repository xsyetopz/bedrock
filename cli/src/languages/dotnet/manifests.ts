import { join } from "node:path";

import { Glob } from "bun";

import { listFiles, readText } from "../../core/fs.ts";
import type { Step } from "../../core/steps.ts";
import { asRecord, type Json, toJson } from "../typescript/json.ts";

/** The .NET release every adopted repo builds on: the current LTS. */
export const DOTNET_MAJOR = 10;

/** The SDK `global.json` pins; `latestFeature` rolls forward to any later 10.0 band and patch. */
export const SDK_VERSION = `${String(DOTNET_MAJOR)}.0.100`;

/** The `dotnet` local tools the recipes run, with their pinned NuGet versions. */
export const TOOLS: Readonly<
	Record<string, { version: string; command: string }>
> = {
	csharpier: { version: "1.4.0", command: "csharpier" },
	fantomas: { version: "8.1.3", command: "fantomas" },
	"dotnet-fsharplint": { version: "0.27.2", command: "dotnet-fsharplint" },
};

/** The file that pins the SDK a repo builds with. */
export const GLOBAL_JSON = "global.json";

/**
 * The places `dotnet tool restore` reads a local tool manifest from at the
 * root: `.config/` first, where bedrock writes one, and the root itself, where
 * `dotnet new tool-manifest` puts one since .NET 10.
 */
export const TOOL_MANIFESTS = [
	".config/dotnet-tools.json",
	"dotnet-tools.json",
];

/** The tool manifest a repo already has, or the `.config/` one bedrock writes. */
export const toolManifestPath = async (root: string): Promise<string> => {
	const found = await Promise.all(
		TOOL_MANIFESTS.map((path) => Bun.file(join(root, path)).exists()),
	);
	return TOOL_MANIFESTS[found.indexOf(true)] ?? ".config/dotnet-tools.json";
};

/** The major version of an SDK version string such as `8.0.100`, or undefined. */
const majorOf = (version: unknown): number | undefined => {
	const match = typeof version === "string" ? /^(\d+)\./u.exec(version) : null;
	return match === null ? undefined : Number(match[1]);
};

/** The `global.json` `test.runner` that opts `dotnet test` into Microsoft.Testing.Platform. */
export const TESTING_PLATFORM = "Microsoft.Testing.Platform";

/**
 * `global.json` pinning the .NET 10 SDK, and with `testingPlatform` the
 * Microsoft.Testing.Platform runner, which the .NET 10 SDK needs to run such
 * a test project at all. A repo's own file keeps its other keys, such as
 * `msbuild-sdks`; only an `sdk` older than 10 moves, and a newer one stays
 * the project's choice.
 */
export const globalJsonStep = async (
	root: string,
	testingPlatform = false,
): Promise<Step[]> => {
	const text = await readText(join(root, GLOBAL_JSON));
	const current: Json =
		text === undefined ? {} : asRecord(Bun.JSONC.parse(text));
	const sdk = asRecord(current["sdk"]);
	const test = asRecord(current["test"]);
	const major = majorOf(sdk["version"]);
	const sdkPinned = major !== undefined && major >= DOTNET_MAJOR;
	const runnerSet = !testingPlatform || test["runner"] === TESTING_PLATFORM;
	if (text !== undefined && sdkPinned && runnerSet) {
		return [];
	}
	const next = {
		...current,
		sdk: sdkPinned
			? sdk
			: { ...sdk, version: SDK_VERSION, rollForward: "latestFeature" },
		...(runnerSet ? {} : { test: { ...test, runner: TESTING_PLATFORM } }),
	};
	return [{ kind: "write", path: GLOBAL_JSON, content: toJson(next) }];
};

/**
 * The repo's tool manifest, or a new `.config/dotnet-tools.json`, with each
 * of {@link TOOLS} at its pinned version. Tools the project added stay;
 * bedrock's own move to its pins, so `dotnet tool restore` installs what the
 * recipes run.
 */
export const toolManifestStep = async (root: string): Promise<Step[]> => {
	const path = await toolManifestPath(root);
	const text = await readText(join(root, path));
	const current = text === undefined ? {} : asRecord(Bun.JSONC.parse(text));
	const tools = { ...asRecord(current["tools"]) };
	// `dotnet tool install` rewrites the manifest in its own layout, so a pinned
	// tool counts by its fields, not by the text.
	const pinned = Object.entries(TOOLS).every(([id, { version, command }]) => {
		const tool = asRecord(tools[id]);
		return (
			tool["version"] === version &&
			JSON.stringify(tool["commands"]) === JSON.stringify([command]) &&
			tool["rollForward"] === false
		);
	});
	if (text !== undefined && pinned) {
		return [];
	}
	for (const [id, { version, command }] of Object.entries(TOOLS)) {
		tools[id] = {
			...asRecord(tools[id]),
			version,
			commands: [command],
			rollForward: false,
		};
	}
	const content = toJson({ version: 1, isRoot: true, ...current, tools });
	return [{ kind: "write", path, content }];
};

/**
 * One target framework moved to .NET 10: `net5.0` through `net9.0` and every
 * `netcoreapp`, keeping a platform suffix such as `-windows`. `netstandard`
 * and .NET Framework targets, such as `net48`, stay, since a library keeps
 * them for callers on older runtimes.
 */
export const retarget = (framework: string): string => {
	const match = /^net(coreapp)?(\d+)\.(\d+)(-.+)?$/u.exec(framework.trim());
	if (match === null) {
		return framework.trim();
	}
	const [, coreapp, major] = match;
	if (coreapp === undefined && Number(major) >= DOTNET_MAJOR) {
		return framework.trim();
	}
	return `net${String(DOTNET_MAJOR)}.0${match[4] ?? ""}`;
};

const FRAMEWORK = /<(TargetFrameworks?)>([^<$]*)<\/\1>/gu;

/** `text` with each literal TargetFramework and TargetFrameworks list retargeted; an MSBuild property stays. */
export const retargetProject = (text: string): string =>
	text.replaceAll(FRAMEWORK, (_whole, tag: string, list: string) => {
		const frameworks = [
			...new Set(
				list
					.split(";")
					.filter((item) => item.trim() !== "")
					.map((item) => retarget(item)),
			),
		];
		return `<${tag}>${frameworks.join(";")}</${tag}>`;
	});

/** The project and props files whose target frameworks move to .NET 10. */
export const retargetSteps = async (root: string): Promise<Step[]> => {
	const paths = await listFiles(
		root,
		new Glob("**/{*.csproj,*.fsproj,Directory.Build.props}"),
	);
	const texts = await Promise.all(
		paths.map((path) => readText(join(root, path))),
	);
	return paths.flatMap((path, index): Step[] => {
		const text = texts[index];
		if (text === undefined) {
			return [];
		}
		const next = retargetProject(text);
		return next === text ? [] : [{ kind: "write", path, content: next }];
	});
};

/**
 * A note for each project that pins a C# `LangVersion` older than the one
 * .NET 10 defaults to, since editorconfig rules such as
 * `csharp_prefer_system_threading_lock` need C# 13 and later.
 */
export const langVersionNotes = async (root: string): Promise<string[]> => {
	const paths = await listFiles(
		root,
		new Glob("**/{*.csproj,Directory.Build.props}"),
	);
	const texts = await Promise.all(
		paths.map((path) => readText(join(root, path))),
	);
	return paths.flatMap((path, index) => {
		const match = /<LangVersion>\s*(\d+)(?:\.\d+)?\s*<\/LangVersion>/u.exec(
			texts[index] ?? "",
		);
		return match !== null && Number(match[1]) < 14
			? [
					`${path}: LangVersion ${match[1] ?? ""} holds C# below the C# 14 that .NET 10 defaults to; delete it unless a consumer needs the older language.`,
				]
			: [];
	});
};
