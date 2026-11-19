import type { Context } from "../../core/language.ts";
import type { Json } from "./json.ts";

/** The package.json fields each kind's starter files need: the entry point other packages import, or the executable. */
const KIND_FIELDS: Readonly<Record<string, (name: string) => Json>> = {
	cli: (name) => ({ bin: { [name]: "./src/cli.ts" } }),
	library: () => ({ exports: { ".": "./src/index.ts" } }),
	plugin: () => ({})
};

/** The fields of the kinds `bedrock init` starts, for fields a manifest lacks. */
export const kindFields = (context: Context, name: string): Json => {
	const kinds =
		typeof context.options["kind"] === "string"
			? context.options["kind"].split(",")
			: [];
	if (!context.init) {
		return {};
	}
	return Object.fromEntries(
		kinds.flatMap((kind) => Object.entries(KIND_FIELDS[kind]?.(name) ?? {}))
	);
};
