import { join } from "node:path";

import type { Context, Finding } from "../../../core/language.ts";
import { readOxlintIgnores } from "../ignores.ts";
import { readJsonc } from "../json.ts";
import { findUndocumentedApi } from "./api.ts";
import {
	findForeignFiles,
	findForeignScripts,
	findUnsortedCatalogs,
} from "./package-managers.ts";
import { findSuppressions, sourcesOf } from "./suppressions.ts";
import { findTypescriptDrift } from "./typescript-drift.ts";

/**
 * What the linters cannot see: suppression comments, other package managers,
 * undocumented package API, TypeScript drift, and unsorted catalogs. Managed
 * files that differ from the templates are core's drift check.
 */
export const guardTypescript = async (context: Context): Promise<Finding[]> => {
	const { root } = context;
	const ignore = await readOxlintIgnores(root);
	const manifest = await readJsonc(join(root, "package.json"));
	const results = await Promise.all([
		sourcesOf(root, ignore).then((paths) => findSuppressions(root, paths)),
		findUndocumentedApi(context, ignore),
		findForeignFiles(root),
		findForeignScripts(root),
		findTypescriptDrift(root),
	]);
	return [...results.flat(), ...findUnsortedCatalogs(manifest)];
};
