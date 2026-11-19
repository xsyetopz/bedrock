import type { Language } from "../../core/language.ts";
import { haxeTest, haxeTypecheck } from "./builds.ts";
import { planHaxe } from "./plan.ts";
import { haxeFormat, haxeLint } from "./style.ts";

/**
 * Haxe, alone or beside other languages: the haxe-formatter, haxe-checkstyle
 * with a config that bounds file and function size, a type check of each hxml
 * build, and the test hxml files that run their tests.
 */
export const haxe: Language = {
	id: "haxe",
	block: "haxe-bedrock",
	aliases: ["haxe", "hx"],
	markers: [".just/haxe.just", "haxelib.json", "**/*.hxml", "**/*.hx"],
	tools: {
		haxe: "https://haxe.org/download, such as brew install haxe, which brings haxelib",
	},
	sources: [
		"**/*.hx",
		"**/*.hxml",
		"haxelib.json",
		"checkstyle.json",
		"hxformat.json",
	],
	kinds: [],
	variables: () => Promise.resolve({}),
	plan: planHaxe,
	guard: () => Promise.resolve([]),
	// checkstyle.json bounds file length, method length, complexity, and
	// parameters (FileLength, MethodLength, CyclomaticComplexity,
	// ParameterNumber), and lizard reads no Haxe, so no lizard bound applies.
	subcommands: {
		format: haxeFormat,
		lint: haxeLint,
		typecheck: haxeTypecheck,
		test: haxeTest,
	},
};
