import { join } from "node:path";

import { readText } from "../../core/fs.ts";
import type { Context } from "../../core/language.ts";
import { mergeBunfig } from "./bunfig.ts";
import { asRecord, readJsonc } from "./json.ts";
import { jsonWrite, type Part } from "./part.ts";
import { migrateTsconfig } from "./tsconfig.ts";
import { variables } from "./variables.ts";

/** Inlines the template's compiler options into the repo's tsconfig.json, or writes the template. */
export const planTsconfig = async (context: Context): Promise<Part> => {
	const { root, templates } = context;
	const text = await templates("merge").read("tsconfig.json");
	const current = await readJsonc(join(root, "tsconfig.json"));
	if (current === undefined) {
		return {
			steps: [{ kind: "write", path: "tsconfig.json", content: text ?? "" }],
			notes: []
		};
	}
	const migrated = migrateTsconfig(
		current,
		asRecord(Bun.JSONC.parse(text ?? "{}"))
	);
	return {
		steps: await jsonWrite(root, "tsconfig.json", migrated.tsconfig),
		notes: migrated.notes
	};
};

/** The template's bunfig.toml plus the settings the repo's copy adds. */
export const planBunfig = async (context: Context): Promise<Part> => {
	const { root, templates } = context;
	const merged = mergeBunfig(
		(await templates("merge").read("bunfig.toml", await variables(context))) ??
			"",
		await readText(join(root, "bunfig.toml"))
	);
	return {
		steps: [{ kind: "write", path: "bunfig.toml", content: merged.content }],
		notes: merged.notes
	};
};
