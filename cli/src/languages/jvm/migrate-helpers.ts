import type { Migration } from "./model.ts";
import { render } from "./render.ts";

/** The rendered build scripts and catalog of a migration, keyed by path. */
export const rendered = async (
	root: string,
	migration: Migration,
): Promise<Record<string, string>> => {
	const steps = await render(root, migration);
	return Object.fromEntries(
		steps.flatMap((step) =>
			step.kind === "write" && typeof step.content === "string"
				? [[step.path, step.content]]
				: [],
		),
	);
};

/** The removed paths of a migration, sorted. */
export const removed = (migration: Migration): string[] =>
	migration.steps
		.filter((step) => step.kind === "remove")
		.map((step) => step.path)
		.toSorted();
