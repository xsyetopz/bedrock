import { parseArgs } from "node:util";

import {
	type LanguageOptions,
	readConfig,
	SOURCE_VERSION,
} from "../core/config.ts";
import { exists } from "../core/fs.ts";
import { uncommitted } from "../core/git.ts";
import { inOrder } from "../core/order.ts";
import {
	buildPlan,
	contextFor,
	type Plan,
	type Selected,
} from "../core/plan.ts";
import { applySteps, describe } from "../core/steps.ts";
import { removeBackups, reportCleanup } from "./clean.ts";
import { detect, dropped, missingTools, parseNamed, select } from "./select.ts";
import { type Mode, prepare, target, USAGE } from "./target.ts";

const confirm = async (yes: boolean): Promise<boolean> => {
	if (yes) {
		return true;
	}
	if (!process.stdin.isTTY) {
		console.error(
			"No terminal to confirm. Pass --yes to apply, or --dry-run to preview.",
		);
		return false;
	}
	process.stdout.write("Apply these changes? [y/N] ");
	for await (const line of console) {
		return line.trim().toLowerCase() === "y";
	}
	return false;
};

const printPlan = async (root: string, plan: Plan): Promise<void> => {
	const lines = await Promise.all(
		plan.steps.map((step) => describe(root, step)),
	);
	console.info(`Plan for ${root}:`);
	for (const line of [
		...lines,
		...plan.notes.map((note) => `note    ${note}`),
	]) {
		console.info(`  ${line}`);
	}
	if (plan.steps.length === 0) {
		console.info("  nothing to change");
	}
};

const selection = async (
	mode: Mode,
	root: string,
	names: readonly string[],
	flags: LanguageOptions,
): Promise<Selected[]> => {
	const config = await readConfig(root);
	if (mode === "update" && config === undefined) {
		throw new Error(`${root} has no .bedrock.json. Run bedrock adopt first.`);
	}
	const named = names.map((name) => parseNamed(name));
	const found =
		named.length === 0 && config === undefined ? await detect(root) : [];
	const detected = found.map((language) => ({ language }));
	const selected = select(config, [...named, ...detected], flags);
	if (selected.length === 0) {
		throw new Error(
			`Found no language in ${root}. Name one, such as bedrock ${mode} ${mode === "init" ? "<dir> " : ""}typescript.`,
		);
	}
	const missing = missingTools(selected);
	if (missing.length > 0) {
		throw new Error(
			[
				"Install these, then run bedrock again:",
				...missing.map((line) => `  ${line}`),
			].join("\n"),
		);
	}
	return selected;
};

/** A repo that runs bedrock from its own source tree keeps that pin; others get this release. */
const versionFor = async (root: string): Promise<string | undefined> => {
	const config = await readConfig(root);
	return config?.version === SOURCE_VERSION ? SOURCE_VERSION : undefined;
};

/** Refuses to overwrite files whose current content git does not hold. */
const checkClean = async (root: string, plan: Plan): Promise<void> => {
	const present = await Promise.all(
		plan.steps.map((step) => exists(`${root}/${step.path}`)),
	);
	const touched = plan.steps
		.filter((_, index) => present[index] === true)
		.map((step) => step.path);
	const dirty = await uncommitted(root, touched);
	if (dirty.length > 0) {
		throw new Error(
			[
				"Commit or stash these first, so git can show and undo what bedrock changes (or pass --allow-dirty):",
				...dirty.map((path) => `  ${path}`),
			].join("\n"),
		);
	}
};

/**
 * The languages whose after-run goes with this apply: all of them when there
 * are steps, else those that a skipped or failed run left work for.
 */
const toRun = async (
	root: string,
	selected: readonly Selected[],
	plan: Plan,
	init: boolean,
): Promise<Selected[]> => {
	const runnable = selected.filter(
		({ language }) => language.afterApply !== undefined,
	);
	if (plan.steps.length > 0) {
		return runnable;
	}
	const pending = await Promise.all(
		runnable.map(
			({ language, options }) =>
				language.runPending?.(
					contextFor(root, selected, language, options, init),
				) ?? Promise.resolve(false),
		),
	);
	return runnable.filter((_, index) => pending[index] === true);
};

/** One plan line per after-run, which `--no-run` turns into a reminder of what it skipped. */
const printRuns = (runs: readonly Selected[], skipped: boolean): void => {
	for (const { language } of runs) {
		const what = language.runs ?? "its setup";
		console.info(
			skipped
				? `  skip    ${language.id}: ${what} (--no-run)`
				: `  run     ${language.id}: ${what}`,
		);
	}
};

const OPTIONS = {
	"allow-dirty": { type: "boolean", default: false },
	cxx: { type: "boolean", default: false },
	"dry-run": { type: "boolean", default: false },
	group: { type: "string" },
	help: { type: "boolean", short: "h", default: false },
	"no-run": { type: "boolean", default: false },
	yes: { type: "boolean", default: false },
} as const;

/** The plan for `selected`, which also removes what dropped languages left. */
const planFor = async (
	root: string,
	selected: readonly Selected[],
	init: boolean,
): Promise<Plan> =>
	buildPlan(root, selected, {
		init,
		version: await versionFor(root),
		dropped: await dropped(root, selected),
	});

/** `bedrock adopt`, `bedrock init`, and `bedrock update`. */
export const adopt = async (
	mode: Mode,
	args: readonly string[],
): Promise<number> => {
	const { values, positionals } = parseArgs({
		args: [...args],
		allowPositionals: true,
		options: OPTIONS,
	});
	if (values.help || (mode === "init" && positionals.length < 2)) {
		console.info(USAGE[mode]);
		return values.help ? 0 : 2;
	}
	const { root, names } = await target(mode, positionals);
	await prepare(mode, root, values["dry-run"]);
	const selected = await selection(mode, root, names, {
		cxx: values.cxx,
		...(values.group === undefined ? {} : { group: values.group }),
	});
	const init = mode === "init";
	const plan = await planFor(root, selected, init);
	await printPlan(root, plan);
	if (values["dry-run"]) {
		return 0;
	}
	const runs = await toRun(root, selected, plan, init);
	printRuns(runs, values["no-run"]);
	if (plan.steps.length > 0 || (runs.length > 0 && !values["no-run"])) {
		if (plan.steps.length > 0 && !init && !values["allow-dirty"]) {
			await checkClean(root, plan);
		}
		if (!(await confirm(values.yes))) {
			return 1;
		}
		await applySteps(root, plan.steps);
		if (!values["no-run"]) {
			await inOrder(runs, async ({ language, options }) => {
				await language.afterApply?.(
					contextFor(root, selected, language, options, init),
				);
			});
		}
	}
	reportCleanup(await removeBackups(root, false));
	console.info(
		"Done. Review the change with git diff, then run just setup and just verify, and fix what they report.",
	);
	return 0;
};
