import { join } from "node:path";
import { parseArgs } from "node:util";

import { type Config, readConfig, SOURCE_VERSION } from "../core/config.ts";
import { existing, linkTarget } from "../core/fs.ts";
import { hooksOn } from "../core/git.ts";
import type { Finding } from "../core/language.ts";
import {
	buildPlan,
	contextFor,
	REPLACED_BY_BASE,
	type Selected,
} from "../core/plan.ts";
import { VERSION } from "../version.ts";
import { findBackups } from "./clean.ts";
import { dropped, select } from "./select.ts";

const USAGE = `Usage: bedrock guard [base|<language>...] [--fix]

Reports what the linters cannot see: suppressions without a reason, files of
replaced tools, managed files that differ from the templates, and each
language's own rules. Without a scope, it checks base and every language
.bedrock.json lists. --fix repairs what it can first.`;

const AGENT_LINKS = ["CLAUDE.md", "GEMINI.md"];

/** Managed files that drifted from the templates this repo pins. */
const drift = async (
	root: string,
	config: Config,
	selected: readonly Selected[],
): Promise<Finding[]> => {
	const plan = await buildPlan(root, selected, {
		init: false,
		version: config.version,
		dropped: await dropped(root, selected),
	});
	return plan.steps
		.filter((step) => !plan.projectFiles.has(step.path))
		.map(
			(step) =>
				`${step.path}: differs from what bedrock writes; run bedrock update`,
		);
};

/** The rules every bedrock repo follows, whatever its languages. */
const guardBase = async (
	root: string,
	config: Config,
	selected: readonly Selected[],
): Promise<Finding[]> => {
	const [targets, replaced, hooks, backups, drifted] = await Promise.all([
		Promise.all(AGENT_LINKS.map((link) => linkTarget(join(root, link)))),
		existing(root, REPLACED_BY_BASE),
		hooksOn(root),
		findBackups(root),
		drift(root, config, selected),
	]);
	return [
		...AGENT_LINKS.filter((_, index) => targets[index] !== "AGENTS.md").map(
			(link) => `${link}: make it a symlink to AGENTS.md`,
		),
		...replaced.map(
			(path) =>
				`${path}: replaced by the justfile and .githooks; move what it still does into project.just`,
		),
		...(process.env["CI"] === undefined && !hooks
			? ["git hooks are off: run just setup"]
			: []),
		...backups.map(
			(path) => `${path}: a backup from an older bedrock; run bedrock clean`,
		),
		...(config.version !== SOURCE_VERSION && config.version !== VERSION
			? [
					`.bedrock.json: pins bedrock ${config.version}, but ${VERSION} runs; run just setup`,
				]
			: []),
		...drifted,
	];
};

/** The scopes to check: those named, or base and every adopted language. */
const scopesOf = (
	positionals: readonly string[],
	selected: readonly Selected[],
): Set<string> => {
	const ids = selected.map((entry) => entry.language.id);
	const scopes = new Set(
		positionals.length === 0 ? ["base", ...ids] : positionals,
	);
	const unknown = [...scopes].filter(
		(scope) => scope !== "base" && !ids.includes(scope),
	);
	if (unknown.length > 0) {
		throw new Error(
			`Not adopted here: ${unknown.join(", ")}. Scopes: base, ${ids.join(", ")}.`,
		);
	}
	return scopes;
};

/** `bedrock guard`. */
export const guard = async (args: readonly string[]): Promise<number> => {
	const { values, positionals } = parseArgs({
		args: [...args],
		allowPositionals: true,
		options: {
			fix: { type: "boolean", default: false },
			help: { type: "boolean", short: "h", default: false },
		},
	});
	if (values.help) {
		console.info(USAGE);
		return 0;
	}
	const root = process.cwd();
	const config = await readConfig(root);
	if (config === undefined) {
		throw new Error(`${root} has no .bedrock.json. Run bedrock adopt first.`);
	}
	const selected = select(config, []);
	const scopes = scopesOf(positionals, selected);
	const findings = await Promise.all([
		scopes.has("base") ? guardBase(root, config, selected) : [],
		...selected
			.filter((entry) => scopes.has(entry.language.id))
			.map(({ language, options }) =>
				language.guard(
					contextFor(root, selected, language, options, false),
					values.fix,
				),
			),
	]);
	for (const finding of findings.flat()) {
		console.error(finding);
	}
	return findings.flat().length === 0 ? 0 : 1;
};
