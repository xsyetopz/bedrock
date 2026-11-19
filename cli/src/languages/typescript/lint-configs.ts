import { join } from "node:path";

import type { Context } from "../../core/language.ts";
import {
	droppedEntries,
	IGNORE_KEYS,
	mergeIgnoreList
} from "./ignore-lists.ts";
import { asRecord, type Json, readJsonc } from "./json.ts";
import { extendsLegacy } from "./legacy.ts";
import type { Part } from "./part.ts";

const isObject = (value: unknown): value is Json =>
	typeof value === "object" && value !== null && !Array.isArray(value);

/** A config value without its `extends`, which the template inlines instead. */
const withoutExtends = (value: unknown): unknown =>
	isObject(value)
		? Object.fromEntries(Object.entries(value).filter(([k]) => k !== "extends"))
		: value;

/** Keys whose values the repo's config sets differently from the template, which the rewrite replaces. */
const droppedKeys = (
	current: Json,
	template: string,
	ignoreKey: string
): string[] => {
	const known = asRecord(Bun.JSONC.parse(template));
	return Object.keys(current).filter(
		(key) =>
			key !== "extends" &&
			key !== ignoreKey &&
			!Bun.deepEquals(withoutExtends(current[key]), withoutExtends(known[key]))
	);
};

export const planIgnoreLists = async (context: Context): Promise<Part> => {
	const { root, templates } = context;
	const parts = await Promise.all(
		Object.entries(IGNORE_KEYS).map(async ([path, key]): Promise<Part> => {
			const template = (await templates("files").read(path)) ?? "";
			const current = await readJsonc(join(root, path));
			const dropped = droppedKeys(asRecord(current), template, key);
			const restored = droppedEntries(template, current, key);
			const notes = [
				...(dropped.length > 0 || extendsLegacy(current)
					? [
							`${path}: now the full bedrock config, without \`extends\`${dropped.length > 0 ? `; its own ${dropped.map((name) => `"${name}"`).join(", ")} were not carried over, git has them` : ""}.`
						]
					: []),
				...(restored.length > 0
					? [
							`${path}: puts back ${restored.map((entry) => `"${entry}"`).join(", ")} in "${key}"; bedrock owns its own entries, and yours go after them.`
						]
					: [])
			];
			return {
				steps: [
					{
						kind: "write",
						path,
						content: mergeIgnoreList(template, current, key)
					}
				],
				notes
			};
		})
	);
	return {
		steps: parts.flatMap((part) => part.steps),
		notes: parts.flatMap((part) => part.notes)
	};
};
