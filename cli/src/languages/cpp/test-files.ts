import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import { cmakeProjects, cmakeTargets } from "./cmake.ts";
import { mesonProjects, mesonTargets } from "./meson.ts";
import { splitSources, type Target } from "./targets.ts";
import { xmakeProjects, xmakeTargets } from "./xmake.ts";

/**
 * The files of `files` in a SwiftPM package's `Tests/` directory, where the
 * package's test targets live and which SwiftPM builds only for `swift test`.
 */
const swiftTestFiles = async (
	root: string,
	files: readonly string[],
): Promise<string[]> => {
	const packages = new Map<string, Promise<boolean>>();
	const isPackage = (dir: string): Promise<boolean> => {
		let found = packages.get(dir);
		if (found === undefined) {
			found = exists(join(root, dir, "Package.swift"));
			packages.set(dir, found);
		}
		return found;
	};
	const picked = await Promise.all(
		files.map(async (file) => {
			const parts = file.split("/");
			const owners = await Promise.all(
				parts.map((part, index) =>
					part === "Tests" && index < parts.length - 1
						? isPackage(parts.slice(0, index).join("/"))
						: Promise.resolve(false),
				),
			);
			return owners.includes(true) ? [file] : [];
		}),
	);
	return picked.flat();
};

/**
 * The targets of each CMake, Meson, and xmake project of `root`. CMake and
 * Meson are read from the build directories `bedrock cpp compile-commands`
 * configured, since a configure can take minutes; a project not configured
 * yet has none. xmake loads its targets without configuring.
 */
const buildTargets = async (root: string): Promise<Target[][]> => {
	const [cmake, meson, xmake] = await Promise.all([
		cmakeProjects(root),
		mesonProjects(root),
		xmakeProjects(root),
	]);
	return Promise.all([
		...cmake.map((project) => cmakeTargets(root, project)),
		...meson.map((project) => mesonTargets(root, project)),
		...xmake.map((project) => xmakeTargets(root, project)),
	]);
};

/**
 * The files of `files`, relative to `root`, that only test code compiles: a
 * SwiftPM package's `Tests/`, and the sources of the targets each CMake,
 * Meson, or xmake build runs as tests, unless a product target of any build
 * compiles them too.
 */
export const cppTestFiles = async (
	root: string,
	files: readonly string[],
): Promise<Set<string>> => {
	const [swift, builds] = await Promise.all([
		swiftTestFiles(root, files),
		buildTargets(root),
	]);
	const splits = builds.map((targets) => splitSources(targets));
	const other = new Set<string>();
	const tests = new Set(swift);
	for (const split of splits) {
		for (const file of split.other) {
			other.add(file);
		}
		for (const file of split.test) {
			tests.add(file);
		}
	}
	return new Set(files.filter((file) => tests.has(file) && !other.has(file)));
};
