import { join, resolve } from "node:path";

import { exists, readText } from "../../core/fs.ts";
import { asArray, asRecord, asString, asStrings } from "../swift/json.ts";
import { firstLine, run } from "../swift/shell.ts";
import {
	buildDir,
	eachProject,
	inBuildDirs,
	projectsOf,
	repoPaths
} from "./projects.ts";
import type { Target } from "./targets.ts";

/** The Meson projects of `root`: each top `meson.build`'s directory. */
export const mesonProjects = (root: string): Promise<string[]> =>
	projectsOf(root, "meson.build");

/**
 * Sets `project` up in its bedrock build directory, or reconfigures the one
 * there, which writes compile_commands.json and the introspection data.
 */
export const configureMeson = async (
	root: string,
	project: string
): Promise<string> => {
	const build = buildDir(root, "meson", project);
	const reconfigure = (await exists(join(build, "meson-private")))
		? ["--reconfigure"]
		: [];
	const result = await run(
		["meson", "setup", ...reconfigure, build, join(root, project)],
		root
	);
	if (result.code !== 0) {
		throw new Error(
			`meson setup of ${project || "."} failed: ${firstLine(result.stdout + result.stderr)}`
		);
	}
	return build;
};

/** The compile_commands.json entries of each Meson project, configured afresh. */
export const mesonEntries = async (root: string): Promise<unknown[]> =>
	eachProject(await mesonProjects(root), async (project) => {
		const build = await configureMeson(root, project);
		return asArray(
			JSON.parse((await readText(join(build, "compile_commands.json"))) ?? "[]")
		);
	});

const introspect = async (
	build: string,
	flag: string
): Promise<readonly unknown[]> => {
	const result = await run(["meson", "introspect", build, flag], build);
	return result.code === 0 ? asArray(JSON.parse(result.stdout)) : [];
};

/** What an introspected target is read against. */
interface Build {
	/** A path Meson printed, relative to the root when inside it. */
	readonly fileOf: (path: string) => string;
	readonly inRepo: (path: string) => string | undefined;
	/** The target that builds each file, keyed by {@link Build.fileOf}. */
	readonly owners: ReadonlyMap<string, string>;
	/** The targets the tests run. */
	readonly tested: ReadonlySet<string>;
}

/**
 * The targets the tests run: the one whose file a test's command is, or, when
 * a test runs a script, the uninstalled targets it depends on.
 */
const testedTargets = (
	tests: readonly Record<string, unknown>[],
	targets: readonly Record<string, unknown>[],
	build: Omit<Build, "tested">
): Set<string> => {
	const installed = new Set(
		targets
			.filter((target) => target["installed"] === true)
			.map((target) => asString(target["id"]))
	);
	return new Set(
		tests.flatMap((test) => {
			const runs = build.owners.get(
				build.fileOf(asStrings(test["cmd"])[0] ?? "")
			);
			return runs === undefined
				? asStrings(test["depends"]).filter((id) => !installed.has(id))
				: [runs];
		})
	);
};

/**
 * The target an introspected target describes. Meson's `depends` leaves out the libraries a target links, so those come
 * from the linker arguments, which name each by its file in the build directory.
 */
const targetOf = (target: Record<string, unknown>, build: Build): Target => {
	const id = asString(target["id"]);
	const groups = asArray(target["target_sources"]).map((group) =>
		asRecord(group)
	);
	const linked = groups
		.filter((group) => "linker" in group)
		.flatMap((group) =>
			asStrings(group["parameters"]).flatMap((argument) => {
				const owner = build.owners.get(build.fileOf(argument));
				return owner === undefined || owner === id ? [] : [owner];
			})
		);
	return {
		id,
		sources: groups.flatMap((group) =>
			asStrings(group["sources"]).flatMap((source) => {
				const path = build.inRepo(source);
				return path === undefined || inBuildDirs(path) ? [] : [path];
			})
		),
		deps: [...new Set([...asStrings(target["depends"]), ...linked])],
		test: build.tested.has(id),
		installed: target["installed"] === true
	};
};

/**
 * The targets of the configured build of `project`, from `meson introspect`.
 * A target whose file a test runs is a test target, installed or not, since
 * installed tests are tests.
 */
export const mesonTargets = async (
	root: string,
	project: string
): Promise<Target[]> => {
	const dir = buildDir(root, "meson", project);
	if (!(await exists(join(dir, "meson-private")))) {
		return [];
	}
	const [targets, tests, inRepo] = await Promise.all([
		introspect(dir, "--targets").then((items) =>
			items.map((item) => asRecord(item))
		),
		introspect(dir, "--tests").then((items) =>
			items.map((item) => asRecord(item))
		),
		repoPaths(root)
	]);
	const fileOf = (path: string): string => inRepo(resolve(dir, path)) ?? path;
	const known = {
		fileOf,
		inRepo,
		owners: new Map(
			targets.flatMap((target) =>
				asStrings(target["filename"]).map((file) => [
					fileOf(file),
					asString(target["id"])
				])
			)
		)
	};
	const build: Build = {
		...known,
		tested: testedTargets(tests, targets, known)
	};
	return targets.map((target) => targetOf(target, build));
};
