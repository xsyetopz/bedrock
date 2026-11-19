import { existing } from "../../core/fs.ts";
import type { Language } from "../../core/language.ts";
import { compileCommands } from "./compile-commands.ts";
import { cppTestFiles } from "./test-files.ts";
import { cppTest } from "./test.ts";

/** Renamed to the dotted names the tools read; the underscore forms are older. */
const OLD_CONFIGS = ["_clang-format", "_clang-tidy"];

/**
 * C and C++, alone or beside other languages: clang-format, clang-tidy, and
 * compile_commands.json from each build system the repo uses.
 */
export const cpp: Language = {
	id: "cpp",
	// The block name predates C++ without Swift; adopted repos carry it.
	block: "swift-bedrock-cxx",
	aliases: ["cpp", "cxx", "c++", "c"],
	markers: [
		".just/cpp.just",
		".clang-format",
		".clang-tidy",
		...OLD_CONFIGS,
		"CMakeLists.txt",
		"meson.build",
		"xmake.lua",
		// Sources at the top two levels, such as micropython's py/*.c, mark a
		// Make-built repo; deeper ones may be vendored, such as node_modules.
		"*.{c,cc,cpp,cxx}",
		"*/*.{c,cc,cpp,cxx}",
		// SwiftPM builds C, C++, and Objective-C targets from the same tree.
		"Sources/**/*.{c,cc,cpp,cxx,m,mm}",
	],
	tools: {},
	sources: [
		"**/*.{c,cc,cpp,cxx,h,hh,hpp,hxx,m,mm}",
		".clang-format",
		".clang-tidy",
	],
	kinds: [],
	variables: () => Promise.resolve({}),
	// The swift plan writes the C++ blocks of Package.swift and the starter files,
	// since both change the same files as the Swift ones.
	plan: async (context) => {
		const found = await existing(context.root, OLD_CONFIGS);
		return {
			steps: found.map((path) => ({ kind: "remove", path })),
			notes: [],
			projectFiles: [],
		};
	},
	guard: () => Promise.resolve([]),
	// clang-tidy has function checks (readability-function-size, readability-function-cognitive-complexity) but no file-length check.
	lizard: ["cpp", "objectivec"],
	testFiles: cppTestFiles,
	subcommands: { "compile-commands": compileCommands, test: cppTest },
};
