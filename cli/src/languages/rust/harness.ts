import type { LanguageOptions } from "../../core/config.ts";
import type { Context } from "../../core/language.ts";
import {
	buildPlan,
	contextFor,
	type Plan,
	type Selected,
} from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { rust } from "./index.ts";

/** The context the Rust checks and plan run with in tests. */
export const contextIn = (
	root: string,
	init = false,
	options: LanguageOptions = {},
): Context => {
	const selected: Selected[] = [{ language: rust, options }];
	return contextFor(root, selected, rust, options, init);
};

/** Plans and applies an adopt (or init) of Rust into `root`, as the commands do without their prompts. */
export const adoptRust = async (
	root: string,
	init = false,
	options: LanguageOptions = {},
): Promise<Plan> => {
	const plan = await buildPlan(root, [{ language: rust, options }], { init });
	await applySteps(root, plan.steps);
	return plan;
};
