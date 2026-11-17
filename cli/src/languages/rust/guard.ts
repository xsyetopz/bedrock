import { join } from "node:path";

import { exists } from "../../core/fs.ts";
import type { Context, Finding } from "../../core/language.ts";
import { inOrder } from "../../core/order.ts";
import { pending } from "../../core/steps.ts";
import { gated, isIntegrationTest } from "./gating.ts";
import { declarations, inlineTests } from "./modules.ts";
import { planRust } from "./plan.ts";
import { rustFiles } from "./project.ts";
import { suppressionProblems } from "./suppressions.ts";

/** Whether the path is a `tests.rs` or a `tests/` inside `src/`, which holds unit tests apart from the code they test. */
export const isTestFileInSrc = (path: string): boolean => {
	const parts = path.split("/");
	const src = parts.indexOf("src");
	return (
		src !== -1 &&
		parts.slice(src + 1).some((part) => part === "tests" || part === "tests.rs")
	);
};

/** One finding per `mod name;` whose `#[cfg(test)]` body sits in another file. */
const unmoved = (file: string, source: string): Finding[] =>
	declarations(source)
		.filter((found) => found.testOnly)
		.map(
			(found) =>
				`${file}:${found.line}: mod ${found.name} keeps its tests in another file; move them here as \`mod ${found.name} { … }\` (just lint-fix does it)`,
		);

/** With `fix`: moves out-of-line test modules inline and gates integration tests; returns what it could not move. */
const repair = async (root: string, file: string): Promise<Finding[]> => {
	const path = join(root, file);
	// Inlining an earlier file can delete a later one in the list.
	if (!(await exists(path))) {
		return [];
	}
	const problems = await inlineTests(path);
	if (isIntegrationTest(file)) {
		const content = gated(await Bun.file(path).text());
		if (content !== undefined) {
			await Bun.write(path, content);
		}
	}
	return problems.map((problem) => problem.replaceAll(`${root}/`, ""));
};

/** The findings for one source file. */
const checkFile = async (root: string, file: string): Promise<Finding[]> => {
	const source = await Bun.file(join(root, file)).text();
	return [
		...unmoved(file, source),
		...suppressionProblems(source).map(
			({ line, message }) => `${file}:${line}: ${message}`,
		),
		...(isTestFileInSrc(file)
			? [
					`${file}: unit tests belong inline in the file they test, in #[cfg(test)] mod tests { … }`,
				]
			: []),
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
	const renamed = open.filter(
		(step) => step.kind === "remove" && step.path.endsWith("/mod.rs"),
	);
	const targets = new Set(
		renamed.map((step) => step.path.replace(/\/mod\.rs$/u, ".rs")),
	);
	return [
		...renamed.map(
			(step) =>
				`${step.path}: name it ${step.path.replace(/\/mod\.rs$/u, ".rs")}, beside its directory (bedrock update does it)`,
		),
		...open
			.filter((step) => !renamed.includes(step) && !targets.has(step.path))
			.map(
				(step) =>
					`${step.path}: differs from what bedrock writes; run bedrock update`,
			),
	];
};

/**
 * What Clippy cannot see: test modules in other files, unit tests under
 * `src/tests`, lint suppressions that skip `#[expect(…, reason)]`, and files
 * that differ from what adopt writes, such as a member that skips the
 * workspace lints, `foo/mod.rs`, or an integration test without
 * `#![cfg(test)]`. With `fix`, moves test modules inline and gates integration
 * tests first.
 */
export const guardRust = async (
	context: Context,
	fix: boolean,
): Promise<Finding[]> => {
	const { root } = context;
	const files = await rustFiles(root);
	const unrepaired: Finding[] = [];
	if (fix) {
		await inOrder(files.toSorted(), async (file) => {
			unrepaired.push(...(await repair(root, file)));
		});
	}
	const current = await rustFiles(root);
	const checked = await Promise.all(
		current.map((file) => checkFile(root, file)),
	);
	const explained = new Set(unrepaired.map((finding) => site(finding)));
	return [
		...unrepaired,
		...checked.flat().filter((finding) => !explained.has(site(finding))),
		...(await layout(context)),
	];
};
