import { join } from "node:path";

import { type Config, SOURCE_VERSION } from "../core/config.ts";
import { existing, linkTarget } from "../core/fs.ts";
import { hooksOn } from "../core/git.ts";
import type { Finding } from "../core/language.ts";
import { buildPlan, REPLACED_BY_BASE, type Selected } from "../core/plan.ts";
import { VERSION } from "../version.ts";
import { findBackups } from "./clean.ts";
import { dropped } from "./select.ts";

const AGENT_LINKS = ["CLAUDE.md", "GEMINI.md"];

/** Managed files that drifted from the templates this repo pins. */
const drift = async (
	root: string,
	config: Config,
	selected: readonly Selected[]
): Promise<Finding[]> => {
	const plan = await buildPlan(root, selected, {
		init: false,
		version: config.version,
		dropped: await dropped(root, selected)
	});
	return plan.steps
		.filter((step) => !plan.projectFiles.has(step.path))
		.map(
			(step) =>
				`${step.path}: differs from what bedrock writes; run \`bedrock update\``
		);
};

/** The rules every bedrock repo follows, whatever its languages. */
export const guardBase = async (
	root: string,
	config: Config,
	selected: readonly Selected[]
): Promise<Finding[]> => {
	const [targets, replaced, hooks, backups, drifted] = await Promise.all([
		Promise.all(AGENT_LINKS.map((link) => linkTarget(join(root, link)))),
		existing(root, REPLACED_BY_BASE),
		hooksOn(root),
		findBackups(root),
		drift(root, config, selected)
	]);
	return [
		...AGENT_LINKS.filter((_, index) => targets[index] !== "AGENTS.md").map(
			(link) => `${link}: make it a symlink to \`AGENTS.md\``
		),
		...replaced.map(
			(path) =>
				`${path}: replaced by the \`justfile\` and \`lefthook.yml\`; move what it still does into \`project.just\``
		),
		...(process.env["CI"] === undefined && !hooks
			? ["git hooks are off: run just setup"]
			: []),
		...backups.map(
			(path) => `${path}: a backup from an older bedrock; run bedrock clean`
		),
		...(config.version !== SOURCE_VERSION && config.version !== VERSION
			? [
					`.editorconfig: \`bedrock_version\` pins ${config.version}, but ${VERSION} runs; run \`just setup\``
				]
			: []),
		...drifted
	];
};
