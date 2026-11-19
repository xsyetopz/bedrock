import { join, relative } from "node:path";

import type { Context, Finding } from "../../core/language.ts";
import { pending } from "../../core/steps.ts";
import { planRust } from "./plan.ts";
import { rustFiles } from "./project.ts";
import { repairRust } from "./repair.ts";
import { suppressionProblems } from "./suppressions.ts";
import {
	inlineTests,
	pathTests,
	testOnlyFiles,
	testsFile
} from "./test-modules.ts";

/** One finding per test module that keeps its body in the file or sets its file with `#[path]`. */
const misplaced = async (
	root: string,
	file: string,
	source: string
): Promise<Finding[]> => {
	const destination = async (name: string): Promise<string> =>
		relative(root, await testsFile(join(root, file), name));
	const inline = await Promise.all(
		inlineTests(source).map(
			async ({ line, name }) =>
				`${file}:${line}: mod ${name} keeps its tests in this file; move them to ${await destination(name)} and declare \`#[cfg(test)] mod ${name};\` (just lint-fix does it)`
		)
	);
	const pathed = await Promise.all(
		pathTests(source).map(
			async ({ line, name }) =>
				`${file}:${line}: mod ${name} sets its file with #[path]; drop the attribute and keep the tests in ${await destination(name)} (just lint-fix does it)`
		)
	);
	return [...inline, ...pathed];
};

/** The findings for one source file; a test-only file may hold test modules. */
const checkFile = async (
	root: string,
	file: string,
	testOnly: boolean
): Promise<Finding[]> => {
	const source = await Bun.file(join(root, file)).text();
	return [
		...(testOnly ? [] : await misplaced(root, file, source)),
		...suppressionProblems(source).map(
			({ line, message }) => `${file}:${line}: ${message}`
		)
	];
};

/** The `file:line` a finding is about, or the whole finding when it names no line. */
const site = (finding: Finding): string =>
	/^.*?:\d+(?=:)/u.exec(finding)?.[0] ?? finding;

/** Renames, manifests, toolchain, and test gates that the plan would still change. */
const layout = async (context: Context): Promise<Finding[]> => {
	const plan = await planRust(context);
	const own = new Set(plan.projectFiles);
	const undone = await pending(context.root, plan.steps);
	const open = undone.filter((step) => own.has(step.path));
	// A rename that rewrites paths is a write of `foo.rs` plus a removal of `foo/mod.rs`.
	const renamed = (path: string): string | undefined => {
		const target = `${path.slice(0, -"/mod.rs".length)}.rs`;
		return path.endsWith("/mod.rs") &&
			plan.steps.some((step) => step.kind === "write" && step.path === target)
			? target
			: undefined;
	};
	const targets = new Set(
		open.flatMap((step) => (step.kind === "remove" ? [renamed(step.path)] : []))
	);
	return open
		.filter((step) => !targets.has(step.path))
		.map((step) => {
			const to = step.kind === "move" ? step.path : renamed(step.path);
			const from = step.kind === "move" ? step.from : step.path;
			return to !== undefined &&
				(step.kind === "move" || step.kind === "remove")
				? `${from}: name it ${to}, beside its directory (bedrock update does it)`
				: `${step.path}: differs from what bedrock writes; run bedrock update`;
		});
};

/**
 * What Clippy cannot see: unit tests that sit in the file they test or behind
 * `#[path]`, lint suppressions that skip `#[expect(…, reason)]`, and files
 * that differ from what adopt writes, such as a member that skips the
 * workspace lints, `foo/mod.rs`, or an integration test without
 * `#![cfg(test)]`. With `fix`, moves test modules to their own files and gates
 * integration tests first.
 */
export const guardRust = async (
	context: Context,
	fix: boolean
): Promise<Finding[]> => {
	const { root } = context;
	const unrepaired = fix ? await repairRust(root, await rustFiles(root)) : [];
	const current = await rustFiles(root);
	const testOnly = await testOnlyFiles(root, current);
	const checked = await Promise.all(
		current.map((file) => checkFile(root, file, testOnly.has(file)))
	);
	const explained = new Set(unrepaired.map((finding) => site(finding)));
	return [
		...unrepaired,
		...checked.flat().filter((finding) => !explained.has(site(finding))),
		...(await layout(context))
	];
};
