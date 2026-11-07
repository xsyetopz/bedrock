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
	"suppressImplicitAnyIndexErrors",
]);
/** Project facts the base config cannot know. */
const PROJECT_OPTIONS = new Set(["jsx", "lib", "types"]);
const BASE_FILE = "bun-bedrock/tsconfig.base.json";
/** Build output a repo can hold: Bun's, Rust's (`cargo doc` writes JavaScript), and SwiftPM's. */
const GENERATED_DIRS = ["**/.build", "**/coverage", "**/dist", "**/target"];
const EMIT_OPTIONS = [
	"composite",
	"declaration",
	"emitDeclarationOnly",
	"outDir",
];

const rebasePaths = (paths: unknown, baseUrl: string): Json => {
	const entries = Object.entries(asRecord(paths));
	const rebase = (target: string): string => `./${join(baseUrl, target)}`;
	return entries.length === 0
		? { "*": [rebase("*")] }
		: Object.fromEntries(
				entries.map(([key, targets]) => [
					key,
					asStrings(targets).map((target) => rebase(target)),
				]),
			);
};

const migrateOptions = (
	compilerOptions: Json,
	policy: Json,
): { options: Json; notes: string[] } => {
	const notes: string[] = [];
	const options: Json = {};
	for (const [key, value] of Object.entries(compilerOptions)) {
		if (REMOVED_OPTIONS.has(key)) {
			notes.push(
				`tsconfig: removed "${key}"; TypeScript 7 rejects it or always behaves as if it were on.`,
			);
		} else if (key === "baseUrl" && typeof value === "string") {
			notes.push(
				`tsconfig: replaced "baseUrl" with "paths"; TypeScript 7 removed it.`,
			);
		} else if (key in policy && !PROJECT_OPTIONS.has(key)) {
			if (JSON.stringify(policy[key]) !== JSON.stringify(value)) {
				notes.push(
					`tsconfig: "${key}": ${JSON.stringify(value)} now comes from ${BASE_FILE} (${JSON.stringify(policy[key])}).`,
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
				...asStrings(policy["types"]),
			]),
		];
	}
	if (EMIT_OPTIONS.some((key) => key in options)) {
		options["noEmit"] = false;
		options["allowImportingTsExtensions"] = false;
		notes.push(
			`tsconfig: the project emits with tsc, so "noEmit" is off. Check "module" and "target" from ${BASE_FILE}.`,
		);
	}
	return { options, notes };
};

export const migrateTsconfig = (
	tsconfig: Json,
	base: Json,
): { tsconfig: Json; notes: string[] } => {
	const { options, notes } = migrateOptions(
		asRecord(tsconfig["compilerOptions"]),
		asRecord(base["compilerOptions"]),
	);
	const own =
		typeof tsconfig["extends"] === "string"
			? [tsconfig["extends"]]
			: asStrings(tsconfig["extends"]);
	// An earlier run left the template's base file here; replace it rather than stack it.
	const parents = own.filter((path) => path !== BASE_FILE);
	if (parents.length > 0) {
		notes.push(
			`tsconfig: still extends ${parents.join(", ")}. If one sets an option TypeScript 7 removed, \`bun check\` names it.`,
		);
	}
	return {
		tsconfig: {
			$schema: "https://json.schemastore.org/tsconfig.json",
			...tsconfig,
			extends: parents.length > 0 ? [...parents, BASE_FILE] : BASE_FILE,
			compilerOptions: options,
			exclude: [
				...new Set([...asStrings(tsconfig["exclude"]), ...GENERATED_DIRS]),
			],
		},
		notes,
	};
};
