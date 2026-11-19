import { mkdir } from "node:fs/promises";
import { join, resolve } from "node:path";

import { Glob } from "bun";

import { exists, readText } from "../../core/fs.ts";
import { asArray, asRecord, asString } from "../swift/json.ts";
import { firstLine, run } from "../swift/shell.ts";
import {
	buildDir,
	eachProject,
	inBuildDirs,
	projectsOf,
	repoPaths,
} from "./projects.ts";
import type { Target } from "./targets.ts";

/** The CMake projects of `root`: each top `CMakeLists.txt`'s directory. */
export const cmakeProjects = (root: string): Promise<string[]> =>
	projectsOf(root, "CMakeLists.txt");

/**
 * Configures `project` in its bedrock build directory, with the File API
 * codemodel query the guard reads and compile_commands.json. BUILD_TESTING is
 * the CTest module's switch, so the tests' targets are in the model.
 */
export const configureCmake = async (
	root: string,
	project: string,
): Promise<string> => {
	const build = buildDir(root, "cmake", project);
	const query = join(build, ".cmake/api/v1/query");
	await mkdir(query, { recursive: true });
	await Bun.write(join(query, "codemodel-v2"), "");
	const result = await run(
		[
			"cmake",
			"-S",
			join(root, project),
			"-B",
			build,
			"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
			"-DBUILD_TESTING=ON",
		],
		root,
	);
	if (result.code !== 0) {
		throw new Error(
			`cmake configure of ${project || "."} failed: ${firstLine(result.stderr)}`,
		);
	}
	return build;
};

/** The compile_commands.json entries of each CMake project, configured afresh. */
export const cmakeEntries = async (root: string): Promise<unknown[]> =>
	eachProject(await cmakeProjects(root), async (project) => {
		const build = await configureCmake(root, project);
		return asArray(
			JSON.parse(
				(await readText(join(build, "compile_commands.json"))) ?? "[]",
			),
		);
	});

const readJson = async (path: string): Promise<Record<string, unknown>> =>
	asRecord(JSON.parse((await readText(path)) ?? "{}"));

/** The quoted and bracket arguments of each `add_test(...)` line. */
const testCommands = (text: string): string[][] =>
	[...text.matchAll(/^add_test\((.*)\)$/gmu)].map((line) =>
		[
			...(line[1] ?? "").matchAll(
				/"((?:[^"\\]|\\.)*)"|\[(=*)\[([\s\S]*?)\]\2\]/gu,
			),
		].map((arg) => arg[1] ?? arg[3] ?? ""),
	);

/**
 * The paths, relative to the root, of the programs the tests of the build at
 * `build` run, from the CTestTestfile.cmake files configure wrote. Only each
 * test's command counts: a tool passed as an argument, such as one a test
 * checks, stays product code.
 */
const testedPaths = async (
	build: string,
	inRepo: (path: string) => string | undefined,
): Promise<Set<string>> => {
	const tested = new Set<string>();
	for await (const file of new Glob("**/CTestTestfile.cmake").scan({
		cwd: build,
		dot: true,
	})) {
		for (const [, command] of testCommands(
			(await readText(join(build, file))) ?? "",
		)) {
			const path =
				command === undefined ? undefined : inRepo(resolve(build, command));
			if (path !== undefined) {
				tested.add(path);
			}
		}
	}
	return tested;
};

/** The codemodel of the latest File API reply in `reply`, if configure wrote one. */
const latestCodemodel = async (
	reply: string,
): Promise<Record<string, unknown> | undefined> => {
	if (!(await exists(reply))) {
		return undefined;
	}
	const indexes = await Array.fromAsync(
		new Glob("index-*.json").scan({ cwd: reply }),
	);
	const latest = indexes.toSorted().at(-1);
	if (latest === undefined) {
		return undefined;
	}
	const index = await readJson(join(reply, latest));
	return readJson(
		join(
			reply,
			asString(asRecord(asRecord(index["reply"])["codemodel-v2"])["jsonFile"]),
		),
	);
};

/** What a File API target object is read against. */
interface Build {
	/** The build directory, which artifact paths are relative to. */
	readonly dir: string;
	/** The source directory, which source paths are relative to. */
	readonly source: string;
	readonly inRepo: (path: string) => string | undefined;
	/** The programs the tests run, relative to the root. */
	readonly tested: ReadonlySet<string>;
}

/** The target a File API target object describes. */
const targetOf = (target: Record<string, unknown>, build: Build): Target => ({
	id: asString(target["id"]),
	sources: asArray(target["sources"])
		.map((item) => asRecord(item))
		.filter((item) => item["isGenerated"] !== true)
		.flatMap((item) => {
			const path = build.inRepo(resolve(build.source, asString(item["path"])));
			// Generated sources and those outside the repo are no file of it.
			return path === undefined || inBuildDirs(path) ? [] : [path];
		}),
	deps: asArray(target["dependencies"]).map((dep) =>
		asString(asRecord(dep)["id"]),
	),
	test: asArray(target["artifacts"]).some((artifact) =>
		build.tested.has(
			build.inRepo(resolve(build.dir, asString(asRecord(artifact)["path"]))) ??
				"",
		),
	),
	installed: target["install"] !== undefined,
});

/**
 * The targets of the configured build of `project`, from the File API
 * codemodel, with the tests from CTest. A target whose artifact a test runs is
 * a test target; one with an install rule is product code.
 */
export const cmakeTargets = async (
	root: string,
	project: string,
): Promise<Target[]> => {
	const dir = buildDir(root, "cmake", project);
	const reply = join(dir, ".cmake/api/v1/reply");
	const codemodel = await latestCodemodel(reply);
	if (codemodel === undefined) {
		return [];
	}
	const [configuration] = asArray(codemodel["configurations"]);
	const inRepo = await repoPaths(root);
	const build: Build = {
		dir,
		source: asString(asRecord(codemodel["paths"])["source"]),
		inRepo,
		tested: await testedPaths(dir, inRepo),
	};
	return Promise.all(
		asArray(asRecord(configuration)["targets"]).map(async (entry) =>
			targetOf(
				await readJson(join(reply, asString(asRecord(entry)["jsonFile"]))),
				build,
			),
		),
	);
};
