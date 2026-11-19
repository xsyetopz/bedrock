import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { asArray, asRecord, asString, asStrings } from "../swift/json.ts";
import { firstLine, run } from "../swift/shell.ts";
import { eachProject, inBuildDirs, projectsOf, repoPaths } from "./projects.ts";
import type { Target } from "./targets.ts";

/**
 * Prints the project's targets as JSON, with the ones `xmake test` runs, from
 * the same `get_tests` xmake's test action and project plugin call. A test
 * with `files` runs a clone of its target with those files added, and a
 * binary target that declares tests is test code even when only its clones
 * run, while a library that declares them stays product code. On-off options
 * named for tests, such as `enable_tests`, are turned on first, since they
 * often hide the test targets. Loading targets installs no packages.
 */
const TARGETS_SCRIPT = `import("core.base.json")
import("core.project.config")
import("core.project.project")
import("actions.test.main", {rootdir = os.programdir(), alias = "test_action"})

function main()
    config.load()
    for name, opt in pairs(project.options()) do
        local default = opt:get("default")
        local flag = opt:get("values") == nil and (default == nil or type(default) == "boolean")
        if flag and name:lower():find("test", 1, true) then
            config.set(name, true)
        end
    end
    local tested = {}
    for _, info in pairs(test_action.get_tests()) do
        tested[info.target:name()] = true
    end
    for _, target in ipairs(project.ordertargets()) do
        if target:kind() == "binary" and #table.wrap(target:get("tests")) > 0 then
            tested[target:name()] = true
        end
    end
    local targets = {}
    for _, target in ipairs(project.ordertargets()) do
        local sources = {}
        for _, file in ipairs(target:sourcefiles()) do
            table.insert(sources, path.absolute(file))
        end
        table.insert(targets, {
            id = target:name(),
            sources = sources,
            deps = table.wrap(target:get("deps")),
            test = tested[target:name()] == true,
            installed = target:is_default() and not tested[target:name()]
        })
    end
    print(json.encode(targets))
end
`;

/** The xmake projects of `root`: each top `xmake.lua`'s directory. */
export const xmakeProjects = (root: string): Promise<string[]> =>
	projectsOf(root, "xmake.lua");

/** Calls `use` with a temporary directory, removed after. */
const inTemp = async <T>(use: (dir: string) => Promise<T>): Promise<T> => {
	const dir = await mkdtemp(join(tmpdir(), "bedrock-xmake-"));
	try {
		return await use(dir);
	} finally {
		await rm(dir, { recursive: true, force: true });
	}
};

/** The compile_commands.json entries of each xmake project. */
export const xmakeEntries = async (root: string): Promise<unknown[]> =>
	eachProject(await xmakeProjects(root), (project) =>
		inTemp(async (output) => {
			const result = await run(
				[
					"xmake",
					"project",
					"-q",
					"-y",
					"-P",
					join(root, project),
					"-k",
					"compile_commands",
					output
				],
				root
			);
			if (result.code !== 0) {
				throw new Error(
					`xmake project of ${project || "."} failed: ${firstLine(result.stderr)}`
				);
			}
			return asArray(
				JSON.parse(await Bun.file(join(output, "compile_commands.json")).text())
			);
		})
	);

/** The targets of the xmake project `project`, or none when xmake cannot load it. */
export const xmakeTargets = (
	root: string,
	project: string
): Promise<Target[]> =>
	inTemp(async (dir) => {
		const script = join(dir, "targets.lua");
		await Bun.write(script, TARGETS_SCRIPT);
		const result = await run(
			["xmake", "lua", "-P", join(root, project), script],
			root
		);
		if (result.code !== 0) {
			return [];
		}
		// xmake prints warnings, such as deprecations, around the JSON line.
		const json = result.stdout.split("\n").find((line) => line.startsWith("["));
		const inRepo = await repoPaths(root);
		return asArray(json === undefined ? [] : JSON.parse(json)).map((item) => {
			const target = asRecord(item);
			return {
				id: asString(target["id"]),
				sources: asStrings(target["sources"]).flatMap((source) => {
					const path = inRepo(source);
					return path === undefined || inBuildDirs(path) ? [] : [path];
				}),
				deps: asStrings(target["deps"]),
				test: target["test"] === true,
				installed: target["installed"] === true
			};
		});
	});
