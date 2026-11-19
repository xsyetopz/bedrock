import { join } from "node:path";

import { $ } from "bun";

import type { Finding } from "../../../core/language.ts";
import { asRecord, readJsonc } from "../json.ts";

/** Reports an installed `typescript` that differs from the one `bun check` runs. */
export const findTypescriptDrift = async (root: string): Promise<Finding[]> => {
	const manifest = await readJsonc(
		join(root, "node_modules/typescript/package.json")
	);
	if (manifest === undefined) {
		return [];
	}
	const installed = asRecord(manifest)["version"];
	// The compiled bedrock binary carries its own Bun; `bun check` is the repo's.
	const run = await $`bun -e "console.log(process.versions.typescript)"`
		.cwd(root)
		.nothrow()
		.quiet();
	const builtIn = run.exitCode === 0 ? run.stdout.toString().trim() : undefined;
	if (builtIn === undefined) {
		return [];
	}
	return installed === builtIn
		? []
		: [
				`typescript: ${String(installed)} is installed but \`bun check\` runs ${builtIn}; set the catalog to ${builtIn}`
			];
};
