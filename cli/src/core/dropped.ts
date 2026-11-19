import { existing } from "./fs.ts";
import type { Language } from "./language.ts";
import { withBlocks } from "./shared.ts";
import { renderPath, templateDir, type Variables } from "./templates.ts";

/** What a language a repo no longer uses left behind: its shared-file blocks and its managed files. */
export interface Leftovers {
	/** The dropped languages that still have a block in a shared file. */
	readonly blocks: readonly Language[];
	/** The managed files of dropped languages that the repo still has. */
	readonly files: readonly string[];
}

/** The blocks and managed files that `dropped` languages left in `root`. */
export const leftovers = async (
	root: string,
	dropped: readonly Language[],
	variables: Variables,
): Promise<Leftovers> => {
	// Paths only: a dropped language's templates need variables this run does not compute.
	const written = await Promise.all(
		dropped.map((language) => templateDir(`${language.id}/files`).paths()),
	);
	const paths = written.flat().map((path) => renderPath(path, variables));
	return {
		blocks: await withBlocks(root, dropped),
		files: await existing(root, [...new Set(paths)]),
	};
};

/** The notes that tell what removing `found` changes. */
export const leftoverNotes = (found: Leftovers): string[] => {
	const reason =
		"neither .editorconfig nor a marker file at the root names the language";
	return [
		...(found.blocks.length > 0
			? [
					`Removed the blocks of ${found.blocks.map((language) => language.id).join(", ")} from the shared files: ${reason}.`,
				]
			: []),
		...(found.files.length > 0
			? [`Removed ${found.files.join(", ")}: ${reason}. git still has them.`]
			: []),
	];
};
