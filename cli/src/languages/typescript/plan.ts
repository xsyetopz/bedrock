import type { Context, LanguagePlan } from "../../core/language.ts";
import { planBunfig, planTsconfig } from "./configs.ts";
import { IGNORE_KEYS } from "./ignore-lists.ts";
import {
	legacyMentions,
	removalSteps,
	workspaceManifestSteps,
} from "./legacy.ts";
import { planIgnoreLists } from "./lint-configs.ts";
import { planManifest } from "./manifest-plan.ts";
import { testSteps } from "./tests.ts";

/** Files that stay the project's own after the first write, so drift checks skip them. */
const PROJECT_FILES = [
	"bunfig.toml",
	"package.json",
	// A repo adds its own ignore entries to these two.
	...Object.keys(IGNORE_KEYS),
	"tsconfig.json",
];

/** Plans the TypeScript part of an adopt: the manifest, configs, removed tools, and test files. */
export const planTypescript = async (
	context: Context,
): Promise<LanguagePlan> => {
	const parts = await Promise.all([
		planManifest(context),
		planTsconfig(context),
		planBunfig(context),
		planIgnoreLists(context),
	]);
	const { root } = context;
	return {
		steps: [
			...parts.flatMap((part) => part.steps),
			...(await removalSteps(root)),
			...(await workspaceManifestSteps(root)),
			...(await testSteps(root)),
		],
		notes: [
			...parts.flatMap((part) => part.notes),
			...(await legacyMentions(root)),
		],
		projectFiles: PROJECT_FILES,
	};
};
