import { join } from "node:path";

import { $ } from "bun";

import type { Context } from "../../core/language.ts";
import { buildPlan, contextFor, type Plan } from "../../core/plan.ts";
import { applySteps } from "../../core/steps.ts";
import { jvm } from "./index.ts";

/** The context the jvm language plans and checks with in `root`. */
export const contextIn = (root: string, kind = "", init = false): Context =>
	contextFor(root, [{ language: jvm, options: { kind } }], jvm, { kind }, init);

/** Plans and applies a jvm adopt of `root`, as `bedrock adopt` or `init` would. */
export const adoptJvm = async (
	root: string,
	kind = "",
	init = false
): Promise<Plan> => {
	const plan = await buildPlan(root, [{ language: jvm, options: { kind } }], {
		init
	});
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

/** Runs the Gradle tasks in `root` with the JDK bedrock finds, returning the exit code and output. */
export const gradleIn = async (
	root: string,
	tasks: readonly string[],
	home: string | undefined
): Promise<{ code: number; output: string }> => {
	const env =
		home === undefined ? process.env : { ...process.env, JAVA_HOME: home };
	const child = Bun.spawn(["./gradlew", "--console=plain", ...tasks], {
		cwd: root,
		env,
		stdout: "pipe",
		stderr: "pipe"
	});
	const [out, err, code] = await Promise.all([
		new Response(child.stdout).text(),
		new Response(child.stderr).text(),
		child.exited
	]);
	return { code, output: out + err };
};
