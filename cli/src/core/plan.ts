import { basename } from "node:path";

import { VERSION } from "../version.ts";
import {
	type Config,
	CONFIG_PATH,
	formatConfig,
	type LanguageOptions,
} from "./config.ts";
import { leftoverNotes, leftovers } from "./dropped.ts";
import { existing } from "./fs.ts";
import type { Context, Language } from "./language.ts";
import { copySteps, sharedSteps, starterSteps } from "./shared.ts";
import { dedupe, pending, type Step } from "./steps.ts";
import { templateDir, type Variables } from "./templates.ts";

/**
 * Hook managers and task runners the justfile and `lefthook.yml` replace,
 * with the `.githooks` of releases before 0.5 and the lefthook config names
 * lefthook would read in place of `lefthook.yml`.
 */
export const REPLACED_BY_BASE = [
	".githooks",
	".husky",
	".lefthook.yaml",
	".lefthook.yml",
	".pre-commit-config.yaml",
	".simple-git-hooks.json",
	"lefthook.yaml",
	"Makefile",
];

/** A language picked for this run, with its settings. */
export interface Selected {
	readonly language: Language;
	readonly options: LanguageOptions;
}

/** Everything one adopt, init, or update changes. */
export interface Plan {
	/** Steps the repo does not match yet, ordered by path. */
	readonly steps: readonly Step[];
	readonly notes: readonly string[];
	/** Paths whose content stays the project's own after the first write. */
	readonly projectFiles: ReadonlySet<string>;
	readonly config: Config;
}

/** Builds the context a language plans and checks with. */
export const contextFor = (
	root: string,
	selected: readonly Selected[],
	language: Language,
	options: LanguageOptions,
	init: boolean,
): Context => ({
	root,
	options,
	init,
	languages: selected.map((entry) => entry.language.id),
	templates: (dir) => templateDir(`${language.id}/${dir}`),
});

const kindsOf = (options: LanguageOptions): string[] =>
	typeof options["kind"] === "string"
		? options["kind"].split(",").filter((kind) => kind !== "")
		: [];

/** The steps and notes for one language. */
const languageSteps = async (
	context: Context,
	language: Language,
	base: Variables,
): Promise<{
	steps: Step[];
	starters: Step[];
	notes: readonly string[];
	projectFiles: readonly string[];
}> => {
	const variables = { ...base, ...(await language.variables(context)) };
	const own = await language.plan(context);
	const kinds = context.init
		? await Promise.all(
				kindsOf(context.options).map((kind) =>
					starterSteps(context.root, `${language.id}/kinds/${kind}`, variables),
				),
			)
		: [];
	const starters = [
		...(await starterSteps(context.root, `${language.id}/starter`, variables)),
		...kinds.flat(),
	];
	return {
		steps: [
			...(await copySteps(`${language.id}/files`, variables)),
			...own.steps,
		],
		starters,
		notes: own.notes,
		projectFiles: own.projectFiles,
	};
};

const configOf = (selected: readonly Selected[], version: string): Config => ({
	version,
	languages: Object.fromEntries(
		selected.map(({ language, options }) => [language.id, options]),
	),
});

const removalNotes = (removed: readonly string[]): string[] =>
	removed.length > 0
		? [
				`Move what ${removed.join(", ")} did into project.just; git still has the removed files.`,
			]
		: [];

/** How {@link buildPlan} plans. */
export interface PlanOptions {
	/** Whether the run starts a project, which adds the starter code. */
	readonly init: boolean;
	/** The bedrock release `.bedrock.json` records. */
	readonly version?: string | undefined;
	/** Languages whose shared-file blocks and managed files the plan removes. */
	readonly dropped?: readonly Language[];
}

/** Plans an adopt of `selected` into `root`, which must already exist. */
export const buildPlan = async (
	root: string,
	selected: readonly Selected[],
	{ init, version = VERSION, dropped = [] }: PlanOptions,
): Promise<Plan> => {
	const base: Variables = { name: basename(root), bedrockVersion: version };
	const parts = await Promise.all(
		selected.map(({ language, options }) =>
			languageSteps(
				contextFor(root, selected, language, options, init),
				language,
				base,
			),
		),
	);
	const config = configOf(selected, version);
	const removed = await existing(root, REPLACED_BY_BASE);
	const leftover = await leftovers(root, dropped, base);
	const starters = parts.flatMap((part) => part.starters);
	const steps = dedupe([
		...leftover.files.map((path): Step => ({ kind: "remove", path })),
		...(await copySteps("base/files", base)),
		...parts.flatMap((part) => part.steps),
		...(await sharedSteps(
			root,
			selected.map((entry) => entry.language),
			base,
			leftover.blocks,
		)),
		...removed.map((path): Step => ({ kind: "remove", path })),
		{ kind: "write", path: CONFIG_PATH, content: formatConfig(config) },
		...starters,
	]);
	return {
		steps: await pending(root, steps),
		notes: [
			...removalNotes(removed),
			...leftoverNotes(leftover),
			...parts.flatMap((part) => part.notes),
		],
		projectFiles: new Set([
			...parts.flatMap((part) => part.projectFiles),
			...starters.map((step) => step.path),
		]),
		config,
	};
};
