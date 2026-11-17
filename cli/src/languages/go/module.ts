import { join } from "node:path";

import { readText } from "../../core/fs.ts";
import type { Context } from "../../core/language.ts";

/** The Go language version a new `go.mod` declares. */
export const GO_VERSION = "1.27";

/** The module path on the `module` line of a `go.mod`, or undefined. */
export const moduleOf = (text: string): string | undefined => {
	const found = /^module\s+("?)(\S+)\1\s*$/mu.exec(text);
	return found?.[2];
};

/** A lower-case Go package identifier from a directory name, such as `my-app` to `myapp`. */
export const packageOf = (name: string): string => {
	const word = name.toLowerCase().replaceAll(/[^a-z0-9]/gu, "");
	return /^[a-z]/u.test(word) ? word : `p${word}`;
};

/** The module path a repo's `go.mod` declares, or undefined when it has none. */
const declaredModule = async (
	context: Context,
): Promise<string | undefined> => {
	const text = await readText(join(context.root, "go.mod"));
	return text === undefined ? undefined : moduleOf(text);
};

/** The module path: the repo's `go.mod`, else `module` in `.bedrock.json`, else a placeholder under `example.com`. */
export const moduleFor = async (
	context: Context,
	name: string,
): Promise<{ readonly path: string; readonly placeholder: boolean }> => {
	const declared = await declaredModule(context);
	if (declared !== undefined) {
		return { path: declared, placeholder: false };
	}
	const chosen = context.options["module"];
	return typeof chosen === "string" && chosen !== ""
		? { path: chosen, placeholder: false }
		: { path: `example.com/${name}`, placeholder: true };
};
