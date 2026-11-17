import { join } from "node:path";

import type { Context } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import type { Variables } from "../../core/templates.ts";

/** The kinds `bedrock init` starts, each a module tree under `templates/jvm/modules`. */
export const KINDS = [
	"android",
	"cmp",
	"java-application",
	"java-library",
	"java-native",
	"kmp",
	"kotlin-application",
	"kotlin-library",
	"kotlin-native",
] as const;

/** The chosen kinds, from the comma-separated `kind` option. */
export const chosenKinds = (context: Context): string[] =>
	typeof context.options["kind"] === "string"
		? context.options["kind"].split(",").filter((kind) => kind !== "")
		: [];

/** A package name from a directory name: lowercase letters and digits, starting with a letter. */
export const packageName = (name: string): string => {
	const cleaned = name
		.toLowerCase()
		.replaceAll(/[^a-z0-9]/gu, "")
		.replace(/^\d+/u, "");
	return cleaned === "" ? "app" : cleaned;
};

/** The module directories a kind creates, such as `lib` or `shared`. */
const modulesOf = (paths: readonly string[]): string[] => [
	...new Set(paths.map((path) => path.split("/")[0] ?? path)),
];

/**
 * The files of each chosen kind that the repo lacks, with `__path__` in
 * directory names filled with the package path, and the modules to include.
 */
export const kindSteps = async (
	context: Context,
	variables: Variables,
): Promise<{ steps: Step[]; modules: string[] }> => {
	const parts = await Promise.all(
		chosenKinds(context).map(async (kind) => {
			const dir = context.templates(`modules/${kind}`);
			const files = await dir.files(variables);
			const steps = files.map((file): Step => ({
				kind: "write",
				path: file.path,
				content: file.content,
			}));
			return { steps, modules: modulesOf(files.map((file) => file.path)) };
		}),
	);
	const steps = parts.flatMap((part) => part.steps);
	const present = await Promise.all(
		steps.map((step) => Bun.file(join(context.root, step.path)).exists()),
	);
	return {
		steps: steps.filter((_, index) => present[index] !== true),
		modules: [...new Set(parts.flatMap((part) => part.modules))],
	};
};
