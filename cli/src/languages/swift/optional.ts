import { Glob } from "bun";

import type { Block } from "../../core/blocks.ts";
import { listFiles } from "../../core/fs.ts";
import type { Context } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import { templateDir } from "../../core/templates.ts";

const read = async (dir: string, path: string): Promise<string> =>
	(await templateDir(dir).read(path)) ?? "";

/** The Package.swift blocks: strict settings, and the C++ interop settings when cpp is adopted. */
export const manifestBlocks = async (cxx: boolean): Promise<Block[]> => {
	const [strict, interop] = await Promise.all([
		read("swift/manifest", "block.swift"),
		cxx ? read("cpp/manifest", "block.swift") : ""
	]);
	return [
		{ name: "swift-bedrock", content: strict },
		...(cxx ? [{ name: "swift-bedrock-cxx", content: interop }] : [])
	];
};

/** Config/Bedrock.xcconfig with the C++ interop switched on and caching off for C++ repositories. */
const xcconfig = (text: string, cxx: boolean): string =>
	cxx
		? text
				.replace(
					"BEDROCK_CXX_FLAGS =\n",
					"BEDROCK_CXX_FLAGS = $(BEDROCK_CXX_INTEROP)\n"
				)
				.replace(
					"COMPILATION_CACHE_ENABLE_CACHING = YES\n",
					"COMPILATION_CACHE_ENABLE_CACHING = NO\n"
				)
		: text;

/** The files that apply only beside an Xcode project or a Tests folder. */
export const optionalSteps = async (
	context: Context,
	cxx: boolean,
	hasXcode: boolean,
	hasTests: boolean
): Promise<Step[]> => {
	const optional = context.templates("optional");
	const wanted: [string, boolean][] = [
		["Config/Bedrock.xcconfig", hasXcode],
		["Tests/.swiftlint.yml", hasTests]
	];
	const steps = await Promise.all(
		wanted.map(async ([path, on]): Promise<Step[]> => {
			const text = on ? await optional.read(path) : undefined;
			return text === undefined
				? []
				: [
						{
							kind: "write",
							path,
							content: xcconfig(text, cxx && path.endsWith(".xcconfig"))
						}
					];
		})
	);
	return steps.flat();
};

/** C and C++ sources, whose presence suggests the C++ setup. */
const CXX_GLOB = "**/*.{c,cc,cpp,cxx,h,hh,hpp,hxx,mm}";

const cxxFiles = (root: string): Promise<string[]> =>
	listFiles(root, new Glob(CXX_GLOB));

/** A note to adopt with `--cxx` when C or C++ files exist but cpp is not adopted. */
export const cxxNote = async (
	root: string,
	cxx: boolean
): Promise<string[]> => {
	const files = cxx ? [] : await cxxFiles(root);
	return files.length > 0
		? [
				"C or C++ files found; adopt with --cxx to add the C++ setup (clang-format, clang-tidy, SwiftPM interop)."
			]
		: [];
};
