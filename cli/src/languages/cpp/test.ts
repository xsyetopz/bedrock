import { join } from "node:path";

import type { Subcommand } from "../../core/language.ts";
import { passes } from "../swift/shell.ts";
import { cmakeProjects, configureCmake } from "./cmake.ts";
import { configureMeson, mesonProjects } from "./meson.ts";
import { eachProject } from "./projects.ts";
import { xmakeProjects } from "./xmake.ts";

/**
 * For each project, the commands that build and test it, in the order they
 * run; a project's commands stop at the first that fails.
 */
const testRuns = async (root: string): Promise<string[][][]> => {
	const [cmake, meson, xmake] = await Promise.all([
		cmakeProjects(root),
		mesonProjects(root),
		xmakeProjects(root)
	]);
	const [cmakeRuns, mesonRuns] = await Promise.all([
		eachProject(cmake, async (project) => {
			const build = await configureCmake(root, project);
			return [
				[
					["cmake", "--build", build],
					[
						"ctest",
						"--test-dir",
						build,
						"--output-on-failure",
						"--no-tests=ignore"
					]
				]
			];
		}),
		eachProject(meson, async (project) => {
			const build = await configureMeson(root, project);
			return [[["meson", "test", "-C", build, "--print-errorlogs"]]];
		})
	]);
	return [
		...cmakeRuns,
		...mesonRuns,
		...xmake.map((project) => [
			["xmake", "test", "-y", "-P", join(root, project)]
		])
	];
};

/**
 * Whether every command of `commands` passes, run one after another, since a
 * project's tests need its build; the first that fails ends the run.
 */
const allPass = async (
	root: string,
	commands: readonly (readonly string[])[]
): Promise<boolean> => {
	const [first, ...rest] = commands;
	return (
		first === undefined || ((await passes(root, first)) && allPass(root, rest))
	);
};

/**
 * Whether each project's commands of `runs` pass. The projects run one at a
 * time, so each one's output reads as a block, and all of them run even after
 * one fails.
 */
const everyRunPasses = async (
	root: string,
	runs: readonly (readonly (readonly string[])[])[]
): Promise<boolean> => {
	const [first, ...rest] = runs;
	if (first === undefined) {
		return true;
	}
	const passed = await allPass(root, first);
	return (await everyRunPasses(root, rest)) && passed;
};

/**
 * `bedrock cpp test`: builds each CMake, Meson, and xmake project in its
 * bedrock build directory and runs its tests: CTest's, `meson test`'s, and
 * `xmake test`'s. A subproject that cannot configure is skipped with a
 * warning, as `bedrock cpp compile-commands` skips it, and every project runs
 * even after one fails, so a run reports them all. `swift test` covers
 * SwiftPM's C++ targets.
 */
export const cppTest: Subcommand = {
	summary: "Build and run the tests of each CMake, Meson, and xmake project.",
	run: async ({ root }) => {
		let runs: string[][][];
		try {
			runs = await testRuns(root);
		} catch (error) {
			console.error(error instanceof Error ? error.message : String(error));
			return 1;
		}
		const passed = await everyRunPasses(root, runs);
		return passed ? 0 : 1;
	}
};
