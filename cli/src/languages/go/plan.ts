import { basename, join } from "node:path";

import { exists } from "../../core/fs.ts";
import type { Context, LanguagePlan } from "../../core/language.ts";
import { GO_VERSION, moduleFor } from "./module.ts";

/** Starts `go.mod` when the repo has none, so the starter config and files have a module to name. */
export const planGo = async (context: Context): Promise<LanguagePlan> => {
	if (await exists(join(context.root, "go.mod"))) {
		return { steps: [], notes: [], projectFiles: [] };
	}
	const { path, placeholder } = await moduleFor(
		context,
		basename(context.root)
	);
	return {
		steps: [
			{
				kind: "write",
				path: "go.mod",
				content: `module ${path}\n\ngo ${GO_VERSION}\n`
			}
		],
		notes: placeholder
			? [
					`go.mod names the placeholder module ${path}; change it, and the goimports prefix in .golangci.yml, to the real path.`
				]
			: [],
		projectFiles: ["go.mod"]
	};
};
