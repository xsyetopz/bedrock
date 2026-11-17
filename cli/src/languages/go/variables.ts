import { basename } from "node:path";

import type { Context } from "../../core/language.ts";
import type { Variables } from "../../core/templates.ts";
import { moduleFor, packageOf } from "./module.ts";

/** Placeholder values for the Go templates: the module path and a package name for the starter code. */
export const variables = async (context: Context): Promise<Variables> => {
	const name = basename(context.root);
	const { path } = await moduleFor(context, name);
	return {
		module: path,
		package: packageOf(name),
	};
};
