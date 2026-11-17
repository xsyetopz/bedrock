import { join } from "node:path";

import { $ } from "bun";

import type { Context } from "../../core/language.ts";
import { buildPlan, contextFor, type Plan } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { typescript } from "./index.ts";

/** The context the TypeScript language plans and checks with in `root`. */
export const contextIn = (root: string, kind = "", init = false): Context =>
	contextFor(
		root,
		[{ language: typescript, options: { kind } }],
		typescript,
		{ kind },
		init,
	);

/** Plans and applies a TypeScript adopt of `root`, as `bedrock adopt` or `init` would. */
export const adoptTypescript = async (
	root: string,
	kind = "",
	init = false,
): Promise<Plan> => {
	const plan = await buildPlan(
		root,
		[{ language: typescript, options: { kind } }],
		{ init },
	);
	await applySteps(root, plan.steps);
	return plan;
};

/** Commits everything in `root`, so tracked-file listings see it. */
export const commitAll = async (root: string): Promise<void> => {
	await $`git add -A`.cwd(root).quiet();
	await $`git -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q --allow-empty -m next --no-verify`
		.cwd(root)
		.quiet();
};

/** The text of `path` in `root`. */
export const textOf = (root: string, path: string): Promise<string> =>
	Bun.file(join(root, path)).text();
