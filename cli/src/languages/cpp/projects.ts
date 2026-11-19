import { realpath } from "node:fs/promises";
import { dirname, isAbsolute, join, relative, resolve } from "node:path";

import { $ } from "bun";

import { toolEnv } from "../swift/shell.ts";

/** Where bedrock configures each build, relative to the root; the cpp gitignore block lists it. */
const BUILD_ROOT = ".cache/bedrock";

/** The projects whose build file no other tracked one of the same name encloses. */
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

/**
 * The top projects of the build file `name`, such as `CMakeLists.txt`: the
 * directories, relative to `root` and `""` for the root, whose file is tracked
 * and in no enclosing project, since the enclosing one includes it.
 */
export const projectsOf = async (
	root: string,
	name: string,
): Promise<string[]> => {
	const listed =
		await $`git ls-files -z --cached --others --exclude-standard -- ${name} ${`*/${name}`}`
			.cwd(root)
			.env(toolEnv())
			.nothrow()
			.quiet();
	return topProjects(
		listed.stdout
			.toString()
			.split("\0")
			// A build directory holds fetched dependencies' build files, such as
			// CMake's _deps, until the gitignore block lists it.
			.filter((path) => path !== "" && !inBuildDirs(path)),
	);
};

/**
 * The items `each` gives for every project, run in parallel. A subproject
 * that fails, such as a starter that needs an SDK fetched first, is skipped
 * with a warning, while the root project's failure stands, since it is the
 * repo's own build.
 */
export const eachProject = async <T>(
	projects: readonly string[],
	each: (project: string) => Promise<readonly T[]>,
): Promise<T[]> => {
	const lists = await Promise.all(
		projects.map(async (project) => {
			try {
				return await each(project);
			} catch (error) {
				if (project === "") {
					throw error;
				}
				console.warn(
					`skipped ${project}: ${error instanceof Error ? error.message : String(error)}`,
				);
				return [];
			}
		}),
	);
	return lists.flat();
};

/**
 * A function from an absolute path a build tool printed to its path relative
 * to `root`, or undefined outside it, such as a generated source in the build
 * directory. Tools print `root` as given or resolved, as /private/tmp for /tmp.
 */
export const repoPaths = async (
	root: string,
): Promise<(path: string) => string | undefined> => {
	const roots = [...new Set([resolve(root), await realpath(root)])];
	return (path) =>
		roots
			.map((base) => relative(base, resolve(base, path)))
			.find(
				(inside) =>
					inside !== "" && !inside.startsWith("..") && !isAbsolute(inside),
			);
};

/** Whether `path`, relative to the root, is in a build directory bedrock configured, as generated sources are. */
export const inBuildDirs = (path: string): boolean =>
	path.startsWith(`${BUILD_ROOT}/`);

/**
 * The build directory bedrock configures `project` in, under `.cache/bedrock`,
 * which the cpp gitignore block lists. `bedrock cpp compile-commands`
 * configures it, and the test recipe and the guard read it.
 */
export const buildDir = (
	root: string,
	system: string,
	project: string,
): string =>
	join(
		root,
		BUILD_ROOT,
		system,
		// `+` for the root, so no subproject's name can collide with it.
		project === "" ? "+" : project.replaceAll("/", "+"),
	);
