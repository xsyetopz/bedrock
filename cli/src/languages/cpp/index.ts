import { existing } from "../../core/fs.ts";
import type { Language } from "../../core/language.ts";
import { compileCommands } from "./compile-commands.ts";

/** Renamed to the dotted names the tools read; the underscore forms are older. */
const OLD_CONFIGS = ["_clang-format", "_clang-tidy"];

/** C and C++ beside Swift: clang-format, clang-tidy, SwiftPM C++ interop, and compile_commands.json. */
export const cpp: Language = {
	id: "cpp",
	block: "swift-bedrock-cxx",
	aliases: ["cpp", "cxx", "c++"],
	markers: [
		".just/cpp.just",
		".clang-format",
		".clang-tidy",
		...OLD_CONFIGS,
		// SwiftPM builds C, C++, and Objective-C targets from the same tree.
		"Sources/**/*.{c,cc,cpp,cxx,m,mm}",
	],
	requires: ["swift"],
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
	subcommands: { "compile-commands": compileCommands },
};
