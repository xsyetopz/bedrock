import type { Subcommand } from "../../core/language.ts";
import { passes } from "../swift/shell.ts";
import { hxmlBuilds, isTestHxml } from "./hxml.ts";

/**
 * Whether every command of `commands` passes, run one after another, since
 * each compilation can take much of the machine's memory. All of them run even
 * after one fails, so a run reports them all.
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

/**
 * `bedrock haxe typecheck`: compiles each hxml file that names its target,
 * from the root, with `--no-output`, which `--each` applies to every
 * compilation `--next` separates. An hxml file without a target is a part the
 * command line completes, such as one a recipe adds `-js <file>` to, or a list
 * of libraries; its target decides which packages it can use, so it is left to
 * the recipe that completes it.
 */
export const haxeTypecheck: Subcommand = {
	summary: "Type-check each hxml file that names its target.",
	run: async ({ root }) => {
		const found = await hxmlBuilds(root);
		const commands = found
			.filter((build) => build.target)
			.map((build) => ["haxe", "--no-output", "--each", build.path]);
		return (await everyPasses(root, commands)) ? 0 : 1;
	},
};

/**
 * `bedrock haxe test`: runs each test hxml file, such as `test.hxml` or
 * `test-native.hxml`, that says how its tests run: `--interp`, `--run`, or a
 * `--cmd` after the compilation. One that does not is named, since the target
 * it runs on is the project's choice, and a recipe in project.just runs it.
 */
export const haxeTest: Subcommand = {
	summary: "Run each test hxml file that runs its tests.",
	run: async ({ root }) => {
		const found = await hxmlBuilds(root);
		const tests = found.filter((build) => isTestHxml(build.path));
		for (const build of tests.filter((test) => !test.runs)) {
			console.error(
				`not run ${build.path}: it names no --interp, --run, or --cmd, so a recipe in project.just runs it`,
			);
		}
		const commands = tests
			.filter((build) => build.runs)
			.map((build) => ["haxe", build.path]);
		return (await everyPasses(root, commands)) ? 0 : 1;
	},
};
