import { join } from "node:path";

import { existing } from "../../../core/fs.ts";
import type { Finding } from "../../../core/language.ts";
import { asRecord, readJsonc } from "../json.ts";

const OTHER_PACKAGE_MANAGER = /\b(?:npm|npx|pnpm|pnpx|yarn)\b/u;
// `.npmrc` is absent on purpose: Bun reads it for registry auth.
const FOREIGN_FILES = [
	".pnpmfile.cjs",
	".yarnrc",
	".yarnrc.yml",
	"bun.lockb",
	"npm-shrinkwrap.json",
	"package-lock.json",
	"pnpm-lock.yaml",
	"pnpm-workspace.yaml",
	"yarn.lock"
];

/** Files only another package manager writes. */
export const findForeignFiles = async (root: string): Promise<Finding[]> => {
	const present = await existing(root, FOREIGN_FILES);
	return present.map((path) => `${path}: not used in a Bun-only repo`);
};

/** Scripts in the root package.json that call npm, pnpm, or yarn. */
export const findForeignScripts = async (root: string): Promise<Finding[]> => {
	const manifest = await readJsonc(join(root, "package.json"));
	return Object.entries(asRecord(asRecord(manifest)["scripts"]))
		.filter(
			([, command]) =>
				typeof command === "string" && OTHER_PACKAGE_MANAGER.test(command)
		)
		.map(
			([name]) => `package.json: script "${name}" calls another package manager`
		);
};

/** `bun fmt` sorts dependencies but not catalogs, so the guard checks them. */
export const findUnsortedCatalogs = (manifest: unknown): Finding[] => {
	const workspaces = asRecord(asRecord(manifest)["workspaces"]);
	const named = Object.entries(asRecord(workspaces["catalogs"])).map(
		([name, value]): [string, unknown] => [`catalogs.${name}`, value]
	);
	const catalogs: [string, unknown][] = [
		["catalog", workspaces["catalog"]],
		...named
	];
	return catalogs.flatMap(([name, catalog]) => {
		const keys = Object.keys(asRecord(catalog));
		return keys.every(
			(key, index) => index === 0 || (keys[index - 1] ?? "") < key
		)
			? []
			: [`package.json: sort workspaces.${name} by package name`];
	});
};
