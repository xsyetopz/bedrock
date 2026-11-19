import { describe, expect, test } from "bun:test";
import { realpath } from "node:fs/promises";
import { join } from "node:path";

import { tempRepo } from "../../core/testing.ts";
import {
	buildDir,
	eachProject,
	projectsOf,
	repoPaths,
	topProjects,
} from "./projects.ts";

describe("topProjects", () => {
	test("keeps the projects no other project contains", () => {
		expect(topProjects(["xmake.lua", "a/xmake.lua", "b/c/xmake.lua"])).toEqual([
			"",
		]);
	});

	test("keeps sibling projects", () => {
		expect(
			topProjects(["a/xmake.lua", "b/xmake.lua", "a/x/xmake.lua"]),
		).toEqual(["a", "b"]);
	});
});

test("projectsOf finds a project whose build file is below the root", async () => {
	// Arrange
	const root = await tempRepo({
		"port/CMakeLists.txt": "project(p)\n",
		"port/lib/CMakeLists.txt": "add_library(l l.c)\n",
	});

	// Act
	const projects = await projectsOf(root, "CMakeLists.txt");

	// Assert
	expect(projects).toEqual(["port"]);
});

test("projectsOf leaves out a dependency fetched into a build directory", async () => {
	// Arrange
	const root = await tempRepo({
		"port/CMakeLists.txt": "project(p)\n",
		".cache/bedrock/cmake/port/_deps/x-src/CMakeLists.txt": "project(x)\n",
	});

	// Act
	const projects = await projectsOf(root, "CMakeLists.txt");

	// Assert
	expect(projects).toEqual(["port"]);
});

test("buildDir keeps the root apart from any subproject", () => {
	expect([buildDir("/r", "cmake", ""), buildDir("/r", "cmake", "a/b")]).toEqual(
		["/r/.cache/bedrock/cmake/+", "/r/.cache/bedrock/cmake/a+b"],
	);
});

/** Gives each project's name, except sdk/template, whose configure fails. */
const configureAll = (project: string): Promise<string[]> =>
	project === "sdk/template"
		? Promise.reject(new Error("configure failed"))
		: Promise.resolve([project]);

describe("eachProject", () => {
	test("skips a subproject that fails and keeps the rest", async () => {
		// Arrange
		const projects = ["port", "sdk/template"];

		// Act
		const items = await eachProject(projects, configureAll);

		// Assert
		expect(items).toEqual(["port"]);
	});

	test("fails when the root project fails", async () => {
		// Arrange
		const root = [""];

		// Act
		const outcome = await eachProject(root, () =>
			configureAll("sdk/template"),
		).then(
			() => "resolved",
			(error: unknown) => (error instanceof Error ? error.message : "thrown"),
		);

		// Assert
		expect(outcome).toBe("configure failed");
	});
});

describe("repoPaths", () => {
	test("maps a resolved path to one relative to the root", async () => {
		// Arrange
		const root = await tempRepo({ "src/a.c": "" });
		const inRepo = await repoPaths(root);

		// Act
		const path = inRepo(join(await realpath(root), "src/a.c"));

		// Assert
		expect(path).toBe("src/a.c");
	});

	test("leaves out a path outside the root", async () => {
		// Arrange
		const root = await tempRepo();
		const inRepo = await repoPaths(root);

		// Act
		const path = inRepo("/usr/include/stdio.h");

		// Assert
		expect(path).toBeUndefined();
	});
});
