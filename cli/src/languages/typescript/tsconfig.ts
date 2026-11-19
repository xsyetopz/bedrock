import { join } from "node:path";

import { asRecord, asStrings, type Json } from "./json.ts";

/** TypeScript 7 rejects these (TS5023, TS5102, TS5108) or makes them the only behavior. */
const REMOVED_OPTIONS = new Set([
	"allowSyntheticDefaultImports",
	"alwaysStrict",
	"charset",
	"downlevelIteration",
	"esModuleInterop",
	"importsNotUsedAsValues",
	"keyofStringsOnly",
	"noImplicitUseStrict",
	"noStrictGenericChecks",
	"out",
	"outFile",
	"prepend",
	"preserveValueImports",
	"suppressExcessPropertyErrors",
	"suppressImplicitAnyIndexErrors"
]);
/** Project facts the base config cannot know. */
const PROJECT_OPTIONS = new Set(["jsx", "lib", "types"]);
const BASE_FILE = "bedrock's tsconfig.json";
/** What older releases extended: the file from the `typescript-bedrock` package. */
const LEGACY_BASE = /(?:^|\/)typescript-bedrock\/tsconfig\.base\.json$/u;
const EMIT_OPTIONS = [
	"composite",
	"declaration",
	"emitDeclarationOnly",
	"outDir"
];

const rebasePaths = (paths: unknown, baseUrl: string): Json => {
	const entries = Object.entries(asRecord(paths));
	const rebase = (target: string): string => `./${join(baseUrl, target)}`;
	return entries.length === 0
		? { "*": [rebase("*")] }
		: Object.fromEntries(
				entries.map(([key, targets]) => [
					key,
					asStrings(targets).map((target) => rebase(target))
				])
			);
};

const migrateOptions = (
	compilerOptions: Json,
	policy: Json
): { options: Json; notes: string[] } => {
	const notes: string[] = [];
	const options: Json = {};
	for (const [key, value] of Object.entries(compilerOptions)) {
		if (REMOVED_OPTIONS.has(key)) {
			notes.push(
				`tsconfig: removed "${key}"; TypeScript 7 rejects it or always behaves as if it were on.`
			);
		} else if (key === "baseUrl" && typeof value === "string") {
			notes.push(
				`tsconfig: replaced "baseUrl" with "paths"; TypeScript 7 removed it.`
			);
		} else if (key in policy && !PROJECT_OPTIONS.has(key)) {
			if (JSON.stringify(policy[key]) !== JSON.stringify(value)) {
				notes.push(
					`tsconfig: "${key}": ${JSON.stringify(value)} is now ${JSON.stringify(policy[key])}, as in ${BASE_FILE}.`
				);
			}
		} else {
			options[key] = value;
		}
	}
	const baseUrl = compilerOptions["baseUrl"];
	if (typeof baseUrl === "string") {
		options["paths"] = rebasePaths(options["paths"], baseUrl);
	}
	// A project `types` replaces the base one, so it must name `bun` itself.
	if ("types" in options) {
		options["types"] = [
			...new Set([
				...asStrings(options["types"]),
				...asStrings(policy["types"])
			])
		];
	}
	if (EMIT_OPTIONS.some((key) => key in options)) {
		options["noEmit"] = false;
		options["allowImportingTsExtensions"] = false;
		notes.push(
			`tsconfig: the project emits with tsc, so "noEmit" is off. Check "module" and "target" from ${BASE_FILE}.`
		);
	}
	return { options, notes };
};

/** The template's options, with the project's own `jsx`, `lib`, `types` and extras on top. */
const inlineOptions = (options: Json, policy: Json): Json => ({
	...policy,
	...options
});

/**
 * Inlines the template's compiler options into a project tsconfig: its options
 * win, the project's `jsx`, `lib`, `types` and unrelated options stay, and
 * an `extends` of the old `typescript-bedrock` base file goes.
 */
export const migrateTsconfig = (
	tsconfig: Json,
	base: Json
): { tsconfig: Json; notes: string[] } => {
	const policy = asRecord(base["compilerOptions"]);
	const migrated = migrateOptions(
		asRecord(tsconfig["compilerOptions"]),
		policy
	);
	const options = inlineOptions(migrated.options, policy);
	const notes = migrated.notes;
	const own =
		typeof tsconfig["extends"] === "string"
			? [tsconfig["extends"]]
			: asStrings(tsconfig["extends"]);
	// An earlier run left the template's base file here; replace it rather than stack it.
	const parents = own.filter((path) => !LEGACY_BASE.test(path));
	if (parents.length > 0) {
		notes.push(
			`tsconfig: still extends ${parents.join(", ")}. If one sets an option TypeScript 7 removed, \`bun check\` names it.`
		);
	}
	return {
		tsconfig: {
			$schema: "https://json.schemastore.org/tsconfig.json",
			...Object.fromEntries(
				Object.entries(tsconfig).filter(([key]) => key !== "extends")
			),
			...(parents.length > 0 ? { extends: parents } : {}),
			compilerOptions: options,
			exclude: [
				...new Set([
					...asStrings(tsconfig["exclude"]),
					...asStrings(base["exclude"])
				])
			]
		},
		notes
	};
};
