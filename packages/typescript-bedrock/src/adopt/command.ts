import { resolve } from "node:path";
import { parseArgs } from "node:util";

import { $ } from "bun";

import { type Plan, planAdoption } from "./adopt.ts";
import { applySteps, describe } from "./steps.ts";
import type { Step } from "./steps.ts";

const USAGE = `Usage: typescript-bedrock adopt <target> [options]

Sets up a new, existing, or legacy repo with typescript-bedrock: Bun only,
TypeScript 7, bun fmt, bun check, bun test, and oxlint, all at full strictness.

Options:
  --package <spec>  Install typescript-bedrock from <spec>, such as file:/path/to/typescript-bedrock.
  --dry-run         Print the plan. Change nothing.
  --yes             Apply without asking (agents, CI).
  --no-install      Skip bun install and the steps that need it.
  -h, --help        Show this help.`;

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

/** `bun fmt --doctor` knows which formatter packages no file needs anymore. */
const removeUnusedFormatters = async (target: string): Promise<void> => {
	const doctor = await $`bun fmt --doctor`.cwd(target).nothrow().quiet();
	const line = /^You can remove:\s*(.+)$/mu.exec(doctor.stdout.toString())?.[1];
	const packages = line?.split(",").map((name) => name.trim()) ?? [];
	if (packages.length > 0) {
		await $`bun remove ${packages}`.cwd(target);
	}
};

const printPlan = async (plan: Plan, target: string): Promise<void> => {
	console.info(`Plan for ${target}:`);
	const lines = await Promise.all(
		plan.steps.map((step) => describe(target, step)),
	);
	for (const line of lines) {
		console.info(`  ${line}`);
	}
	for (const lockfile of plan.lockfiles) {
		console.info(`  migrate ${lockfile} into bun.lock, then remove it`);
	}
	for (const note of plan.notes) {
		console.info(`  note    ${note}`);
	}
};

const install = async (target: string, plan: Plan): Promise<void> => {
	await $`bun install`.cwd(target);
	await applySteps(
		target,
		plan.lockfiles.map((path): Step => ({ kind: "remove", path })),
	);
	await removeUnusedFormatters(target);
	// The code takes the bedrock style here rather than failing the first `just format-check`.
	await $`bun fmt`.cwd(target).nothrow().quiet();
};

export const adopt = async (args: string[]): Promise<number> => {
	const { values, positionals } = parseArgs({
		args,
		allowPositionals: true,
		options: {
			"dry-run": { type: "boolean", default: false },
			help: { type: "boolean", short: "h", default: false },
			"no-install": { type: "boolean", default: false },
			package: { type: "string" },
			yes: { type: "boolean", default: false },
		},
	});
	const [targetArg] = positionals;
	if (values.help || targetArg === undefined) {
		console.info(USAGE);
		return values.help ? 0 : 2;
	}
	const target = resolve(targetArg);
	const plan = await planAdoption(resolve(import.meta.dir, "../.."), target, {
		packageSpec: values.package,
	});

	await printPlan(plan, target);
	if (values["dry-run"]) {
		return 0;
	}
	if (!(await confirm(values.yes))) {
		return 1;
	}

	await applySteps(target, plan.steps);
	if (values["no-install"]) {
		console.info("Skipped bun install. Run `just setup`, then `just verify`.");
		return 0;
	}
	await install(target, plan);
	console.info(
		"Done. Run `just setup` and `just verify`, and fix what they report. Then compare each *.bak file with its replacement and delete it.",
	);
	return 0;
};
