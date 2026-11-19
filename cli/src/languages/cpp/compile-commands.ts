import { extname, join, resolve } from "node:path";

import type { Subcommand } from "../../core/language.ts";
import { asArray, asRecord, asString } from "../swift/json.ts";
import { firstLine, run } from "../swift/shell.ts";
import { cmakeEntries } from "./cmake.ts";
import { mesonEntries } from "./meson.ts";
import { xmakeEntries } from "./xmake.ts";

/** One entry of compile_commands.json. */
export interface CompileCommand {
	readonly arguments: readonly string[];
	readonly directory: string;
	readonly file: string;
}

/** What SwiftPM and the Swift driver report about the package and the toolchain. */
export interface Inputs {
	readonly describe: Record<string, unknown>;
	readonly info: Record<string, unknown>;
	readonly sdk: string;
}

const languageFlags = (extension: string, standard: string): string[] => {
	switch (extension) {
		case ".c": {
			return ["clang", "-std=c17"];
		}
		case ".m": {
			return ["clang", "-fobjc-arc"];
		}
		case ".mm": {
			return ["clang++", `-std=${standard}`, "-fobjc-arc"];
		}
		default: {
			return ["clang++", `-std=${standard}`];
		}
	}
};

/**
 * One entry per C, C++, and Objective-C source in each Clang target, with the
 * include paths and system roots a build would pass.
 */
export const swiftEntries = (
	root: string,
	inputs: Inputs
): CompileCommand[] => {
	const resources = asString(
		asRecord(inputs.info["paths"])["runtimeResourcePath"]
	);
	// <swift/bridging> lives in the toolchain's usr/include, beside usr/lib/swift.
	const toolchainInclude = resolve(resources, "../../include");
	const standard =
		asString(inputs.describe["cxx_language_standard"]) || "c++20";
	const targets = asArray(inputs.describe["targets"])
		.map((target) => asRecord(target))
		.filter((target) => target["module_type"] === "ClangTarget");
	const includes = targets.flatMap((target) => [
		"-I",
		`${root}/${asString(target["path"])}/include`
	]);
	const common = [
		...(inputs.sdk === "" ? [] : ["-isysroot", inputs.sdk]),
		"-isystem",
		toolchainInclude,
		"-mmacosx-version-min=12.0",
		...includes
	];
	return targets.flatMap((target) =>
		asArray(target["sources"]).map((source) => {
			const relative = asString(source);
			const file = `${root}/${asString(target["path"])}/${relative}`;
			return {
				arguments: [
					...languageFlags(extname(relative), standard),
					...common,
					"-c",
					file
				],
				directory: root,
				file
			};
		})
	);
};

const swiftJson = async (
	root: string,
	command: readonly string[]
): Promise<Record<string, unknown>> => {
	const result = await run(command, root);
	if (result.code !== 0) {
		throw new Error(`${command.join(" ")} failed: ${firstLine(result.stderr)}`);
	}
	return asRecord(JSON.parse(result.stdout));
};

/** Keys in a stable order, so the file only changes when the commands do. */
const sorted = (entry: unknown): unknown =>
	typeof entry === "object" && entry !== null && !Array.isArray(entry)
		? Object.fromEntries(
				Object.entries(entry).toSorted(([a], [b]) => a.localeCompare(b))
			)
		: entry;

/** The entries of the Clang targets of the SwiftPM package at `root`, if any. */
const packageEntries = async (root: string): Promise<CompileCommand[]> => {
	if (!(await Bun.file(join(root, "Package.swift")).exists())) {
		return [];
	}
	const [describe, info, sdk] = await Promise.all([
		swiftJson(root, [
			"swift",
			"package",
			"--disable-sandbox",
			"describe",
			"--type",
			"json"
		]),
		swiftJson(root, ["swift", "-print-target-info"]),
		run(["xcrun", "--show-sdk-path"], root)
	]);
	return swiftEntries(root, {
		describe,
		info,
		sdk: sdk.code === 0 ? sdk.stdout.trim() : ""
	});
};

/**
 * `bedrock cpp compile-commands`: writes compile_commands.json for clang-tidy and
 * clangd from each build system the repo uses: a SwiftPM package's Clang
 * targets, which SwiftPM emits no database for, and each CMake, Meson, and
 * xmake project. CMake and Meson configure in `.cache/bedrock`, where the
 * guard and `bedrock cpp test` find the builds.
 */
export const compileCommands: Subcommand = {
	summary: "Write compile_commands.json for clang-tidy and clangd.",
	run: async (context) => {
		const { root } = context;
		try {
			const lists = await Promise.all([
				packageEntries(root),
				cmakeEntries(root),
				mesonEntries(root),
				xmakeEntries(root)
			]);
			const entries = lists.flat();
			await Bun.write(
				join(root, "compile_commands.json"),
				`${JSON.stringify(
					entries.map((entry) => sorted(entry)),
					undefined,
					2
				)}\n`
			);
			return 0;
		} catch (error) {
			console.error(error instanceof Error ? error.message : String(error));
			return 1;
		}
	}
};
