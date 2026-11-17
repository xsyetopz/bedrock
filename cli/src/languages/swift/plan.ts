import { basename, join } from "node:path";

import { exists, readText } from "../../core/fs.ts";
import type { Context, LanguagePlan } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import { conversionPlan } from "./conversion.ts";
import { kindsOf, kitSteps } from "./kit.ts";
import { legacyPlan } from "./legacy.ts";
import { adoptedManifest } from "./manifest.ts";
import { cxxNote, manifestBlocks, optionalSteps } from "./optional.ts";
import { wired, xcodeProjects } from "./xcode.ts";

const xcodeSteps = async (
	root: string,
	projects: readonly string[],
): Promise<Step[]> => {
	const steps = await Promise.all(
		projects.map(async (project): Promise<Step[]> => {
			const path = `${project}/project.pbxproj`;
			const text = await readText(join(root, path));
			const next = text === undefined ? undefined : wired(text);
			return next === undefined ? [] : [{ kind: "write", path, content: next }];
		}),
	);
	return steps.flat();
};

/** Plans the Swift part of an adopt: the strict manifest, Xcode wiring, optional configs, XCTest conversion, and legacy cleanup. */
export const planSwift = async (context: Context): Promise<LanguagePlan> => {
	const { root } = context;
	const cxx = context.languages.includes("cpp");
	const [projects, manifest, tests, blocks] = await Promise.all([
		xcodeProjects(root),
		readText(join(root, "Package.swift")),
		exists(join(root, "Tests")),
		manifestBlocks(cxx),
	]);
	const hasXcode =
		projects.length > 0 || (context.init && kindsOf(context).has("app"));
	const [optional, xcode, legacy, conversion, kit, cxxHint] = await Promise.all(
		[
			optionalSteps(context, cxx, hasXcode, tests || context.init),
			xcodeSteps(root, projects),
			legacyPlan(root),
			conversionPlan(root),
			context.init
				? kitSteps(context, cxx, blocks, manifest !== undefined, basename(root))
				: [],
			cxxNote(root, cxx),
		],
	);
	const notes = [...legacy.notes, ...conversion.notes, ...cxxHint];
	const steps = [...optional, ...xcode, ...legacy.steps, ...conversion.steps];
	const projectFiles = [
		...xcode.map((step) => step.path),
		...conversion.projectFiles,
		...kit.map((step) => step.path),
	];
	if (manifest !== undefined) {
		steps.push({
			kind: "write",
			path: "Package.swift",
			content: adoptedManifest(manifest, blocks),
		});
		projectFiles.push("Package.swift");
	} else if (!context.init && projects.length === 0) {
		notes.push(
			"Package.swift and an .xcodeproj are both missing; run bedrock init to start a package.",
		);
	}
	return { steps: [...steps, ...kit], notes, projectFiles };
};
