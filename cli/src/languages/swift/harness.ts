import { join } from "node:path";

import { $ } from "bun";

import type { Context } from "../../core/language.ts";
import { buildPlan, contextFor, type Plan } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { cpp } from "../cpp/index.ts";
import { swift } from "./index.ts";

const selection = (kind: string, cxx: boolean) => [
	{ language: swift, options: { kind } },
	...(cxx ? [{ language: cpp, options: {} }] : []),
];

/** The context the swift language plans and checks with in `root`. */
export const contextIn = (root: string, kind = "", init = false): Context =>
	contextFor(root, selection(kind, false), swift, { kind }, init);

/** Plans and applies a swift adopt of `root`, as `bedrock adopt` or `init` would. */
export const adoptSwift = async (
	root: string,
	{ kind = "", init = false, cxx = false } = {},
): Promise<Plan> => {
	const plan = await buildPlan(root, selection(kind, cxx), { init });
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

/** `value`, or a thrown error naming what a test expected to exist. */
export const required = <T>(value: T | undefined, what = "value"): T => {
	if (value === undefined) {
		throw new Error(`expected ${what}`);
	}
	return value;
};

/** The content the plan writes to `path`. */
export const written = (
	steps: readonly { kind: string; path: string; content?: string }[],
	path: string,
): string =>
	required(
		steps.find((step) => step.kind === "write" && step.path === path)?.content,
		path,
	);
