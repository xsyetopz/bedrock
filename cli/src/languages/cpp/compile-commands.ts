import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, extname, join, resolve } from "node:path";

import { $ } from "bun";

import type { Subcommand } from "../../core/language.ts";
import { asArray, asRecord, asString } from "../swift/json.ts";
import { firstLine, run, toolEnv } from "../swift/shell.ts";

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
	inputs: Inputs,
): CompileCommand[] => {
	const resources = asString(
		asRecord(inputs.info["paths"])["runtimeResourcePath"],
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
		`${root}/${asString(target["path"])}/include`,
	]);
	const common = [
		...(inputs.sdk === "" ? [] : ["-isysroot", inputs.sdk]),
		"-isystem",
		toolchainInclude,
		"-mmacosx-version-min=12.0",
		...includes,
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
					file,
				],
				directory: root,
				file,
			};
		}),
	);
};

/** The xmake projects that no other tracked xmake.lua includes. */
export const topProjects = (lists: readonly string[]): string[] => {
	const projects = lists
		.map((path) => dirname(path))
		.map((dir) => (dir === "." ? "" : dir));
	return projects.filter(
		(project) =>
			!projects.some(
				(other) =>
					other !== project &&
					(other === "" || project.startsWith(`${other}/`)),
			),
	);
};

const xmakeEntries = async (root: string): Promise<unknown[]> => {
	const listed =
		await $`git ls-files -z --cached --others --exclude-standard -- xmake.lua '*/xmake.lua'`
			.cwd(root)
			.env(toolEnv())
			.nothrow()
			.quiet();
	const lists = listed.stdout
		.toString()
		.split("\0")
		.filter((path) => path !== "");
	const output = await mkdtemp(join(tmpdir(), "bedrock-xmake-"));
	try {
		const entries = await Promise.all(
			topProjects(lists).map(async (project) => {
				const result = await run(
					[
						"xmake",
						"project",
						"-q",
						"-y",
						"-P",
						join(root, project),
						"-k",
						"compile_commands",
						output,
					],
					root,
				);
				if (result.code !== 0) {
					throw new Error(`xmake project failed: ${firstLine(result.stderr)}`);
				}
				return asArray(
					JSON.parse(
						await Bun.file(join(output, "compile_commands.json")).text(),
					),
				);
			}),
		);
		return entries.flat();
	} finally {
		await rm(output, { recursive: true, force: true });
	}
};

const swiftJson = async (
	root: string,
	command: readonly string[],
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
				Object.entries(entry).toSorted(([a], [b]) => a.localeCompare(b)),
			)
		: entry;

/**
 * `bedrock cpp compile-commands`: writes compile_commands.json for clang-tidy and
 * clangd, which SwiftPM does not emit, from the package's Clang targets and each
 * xmake project.
 */
export const compileCommands: Subcommand = {
	summary: "Write compile_commands.json for clang-tidy and clangd.",
	run: async (context) => {
		const { root } = context;
		try {
			const [describe, info, sdk] = await Promise.all([
				swiftJson(root, [
					"swift",
					"package",
					"--disable-sandbox",
					"describe",
					"--type",
					"json",
				]),
				swiftJson(root, ["swift", "-print-target-info"]),
				run(["xcrun", "--show-sdk-path"], root),
			]);
			const entries = [
				...swiftEntries(root, {
					describe,
					info,
					sdk: sdk.code === 0 ? sdk.stdout.trim() : "",
				}),
				...(await xmakeEntries(root)),
			];
			await Bun.write(
				join(root, "compile_commands.json"),
				`${JSON.stringify(
					entries.map((entry) => sorted(entry)),
					undefined,
					2,
				)}\n`,
			);
			return 0;
		} catch (error) {
			console.error(error instanceof Error ? error.message : String(error));
			return 1;
		}
	},
};
