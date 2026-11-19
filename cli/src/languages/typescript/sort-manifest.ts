import { asRecord, isRecord, type Json } from "./json.ts";

/** The order `bun fmt` gives package.json keys; other keys follow alphabetically. */
const KEY_ORDER = [
	"name",
	"version",
	"private",
	"description",
	"keywords",
	"homepage",
	"bugs",
	"license",
	"author",
	"repository",
	"bin",
	"workspaces",
	"files",
	"type",
	"sideEffects",
	"main",
	"module",
	"types",
	"exports",
	"publishConfig",
	"scripts",
	"dependencies",
	"devDependencies",
	"peerDependencies",
	"optionalDependencies",
	"overrides",
	"engines",
	"packageManager",
	"trustedDependencies"
];
const SORTED_MAPS = new Set([
	"scripts",
	"dependencies",
	"devDependencies",
	"peerDependencies",
	"optionalDependencies",
	"overrides"
]);

// Code-unit order, the one the formatter uses; localeCompare would reorder "@scope" names.
const byName = ([a]: [string, unknown], [b]: [string, unknown]): number => {
	if (a === b) {
		return 0;
	}
	return a < b ? -1 : 1;
};

const rank = (key: string): number => {
	const index = KEY_ORDER.indexOf(key);
	return index === -1 ? KEY_ORDER.length : index;
};

/**
 * The manifest with its keys in the order `bun fmt` writes, so a fresh adopt
 * passes `bun fmt --check`. Maps of names sort alphabetically.
 */
export const sortManifest = (manifest: Json): Json =>
	Object.fromEntries(
		Object.entries(manifest)
			.toSorted(
				(left, right) => rank(left[0]) - rank(right[0]) || byName(left, right)
			)
			.map(([key, value]): [string, unknown] => [
				key,
				SORTED_MAPS.has(key) && isRecord(value)
					? Object.fromEntries(Object.entries(asRecord(value)).toSorted(byName))
					: value
			])
	);
