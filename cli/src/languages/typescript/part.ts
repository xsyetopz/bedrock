import { join } from "node:path";

import type { Step } from "../../core/steps.ts";
import { type Json, readJsonc, toJson } from "./json.ts";

/** Steps for one file, with what adopt could not carry over. */
export interface Part {
	readonly steps: Step[];
	readonly notes: string[];
}

/** Rewrites a JSON file only when its data changes, so its layout and comments stay. */
export const jsonWrite = async (
	root: string,
	path: string,
	next: Json
): Promise<Step[]> =>
	Bun.deepEquals(await readJsonc(join(root, path)), next)
		? []
		: [{ kind: "write", path, content: toJson(next) }];
