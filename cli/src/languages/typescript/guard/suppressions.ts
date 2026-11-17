import { join } from "node:path";

import { Glob } from "bun";

import type { Finding } from "../../../core/language.ts";
import { listSources } from "../ignores.ts";

/** Every JavaScript and TypeScript source the checks read. */
export const SOURCE = new Glob(
	"**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts,vue,svelte,astro}",
);
// Written as alternations so this file does not match itself.
const SUPPRESSION =
	/(?:eslint|oxlint)-disable|biome-(?:ignore)|@ts-(?:ignore|expect-error|nocheck)/u;

/** The lines of `paths` that switch a linter or the type checker off. */
export const findSuppressions = async (
	root: string,
	paths: readonly string[],
): Promise<Finding[]> => {
	const texts = await Promise.all(
		paths.map((path) => Bun.file(join(root, path)).text()),
	);
	return paths.flatMap((path, index) =>
		(texts[index] ?? "")
			.split("\n")
			.flatMap((line, row) =>
				SUPPRESSION.test(line)
					? [`${path}:${row + 1}: suppression comment`]
					: [],
			),
	);
};

/** The sources under `root`, minus the ones the repo's oxlint ignores. */
export const sourcesOf = (
	root: string,
	ignore: readonly string[],
): Promise<string[]> => listSources(root, SOURCE, ignore);
