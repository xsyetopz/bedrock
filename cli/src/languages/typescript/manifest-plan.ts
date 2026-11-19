import { basename, join } from "node:path";

import { readText } from "../../core/fs.ts";
import type { Context } from "../../core/language.ts";
import { asRecord, readJsonc } from "./json.ts";
import { kindFields } from "./kinds.ts";
import { migrateManifest } from "./manifest.ts";
import { jsonWrite, type Part } from "./part.ts";
import { sortManifest } from "./sort-manifest.ts";
import { variables } from "./variables.ts";

/** Migrates the repo's package.json onto the template and the bun toolchain. */
export const planManifest = async (context: Context): Promise<Part> => {
	const { root, templates } = context;
	const template = await templates("manifest").read(
		"package.json",
		await variables(context)
	);
	const name = basename(root);
	const manifest = {
		...kindFields(context, name),
		...((await readJsonc(join(root, "package.json"))) ?? {
			name,
			version: "0.0.0",
			private: true,
			type: "module"
		})
	};
	const pnpmText = await readText(join(root, "pnpm-workspace.yaml"));
	const migrated = migrateManifest(
		manifest,
		asRecord(Bun.JSONC.parse(template ?? "{}")),
		pnpmText === undefined ? undefined : asRecord(Bun.YAML.parse(pnpmText))
	);
	return {
		steps: await jsonWrite(
			root,
			"package.json",
			sortManifest(migrated.manifest)
		),
		notes: migrated.notes
	};
};
