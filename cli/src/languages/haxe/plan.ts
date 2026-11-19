import { join } from "node:path";

import { readText } from "../../core/fs.ts";
import type { Context, LanguagePlan } from "../../core/language.ts";
import { hxmlBuilds, isTestHxml } from "./hxml.ts";

const WORKFLOW = ".github/workflows/haxe.yml";

/**
 * A note when adopt replaces a Haxe workflow bedrock did not write, since its
 * steps, such as checking out sibling repositories or running the project's
 * own test recipe, are the project's and a person moves them.
 */
const workflowNote = async (root: string): Promise<string[]> => {
	const text = await readText(join(root, WORKFLOW));
	if (text === undefined || text.startsWith("# Managed by bedrock.")) {
		return [];
	}
	return [
		`${WORKFLOW}: replaced with bedrock's; git has yours. Move the steps it still needs into .github/actions/project-setup/action.yml and the recipes it ran into project.just.`,
	];
};

/** A note for each test hxml file `bedrock haxe test` will not run, since it says no way to run its tests. */
const testNotes = async (root: string): Promise<string[]> => {
	const found = await hxmlBuilds(root);
	return found
		.filter((build) => isTestHxml(build.path) && !build.runs)
		.map(
			(build) =>
				`${build.path}: names no --interp, --run, or --cmd, so just haxe::test does not run it; a project-test recipe in project.just runs it on its target.`,
		);
};

/** The notes a Haxe adopt leaves for a person; the templates carry every file. */
export const planHaxe = async (context: Context): Promise<LanguagePlan> => ({
	steps: [],
	notes: [
		...(await workflowNote(context.root)),
		...(await testNotes(context.root)),
	],
	projectFiles: [],
});
