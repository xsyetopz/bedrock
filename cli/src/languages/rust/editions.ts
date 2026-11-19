import { join } from "node:path";

import { $ } from "bun";

import { editionsOf } from "./manifests.ts";
import { type Manifest, manifests } from "./project.ts";

/** Each edition `cargo fix --edition` migrates code from, with the one it migrates to. */
const NEXT: Readonly<Record<string, string>> = {
	"2015": "2018",
	"2018": "2021",
	"2021": "2024"
};

/** An `edition = "<year>"` line older than 2024, at a package, workspace package, or target. */
const OLD_EDITION = /^(\s*edition\s*=\s*)"(2015|2018|2021)"/gmu;

/** The manifest with every declared edition older than 2024 raised by one step. */
export const raisedEdition = (text: string): string =>
	text.replaceAll(
		OLD_EDITION,
		(_, key: string, year: string) => `${key}"${NEXT[year] ?? year}"`
	);

/** The manifests that still declare an edition older than 2024. */
const oldManifests = async (root: string): Promise<Manifest[]> => {
	const found = await manifests(root);
	return found.filter(
		(manifest) => raisedEdition(manifest.text) !== manifest.text
	);
};

/** Whether a manifest still declares an edition older than 2024, as a skipped or failed migration leaves it. */
export const editionPending = async (root: string): Promise<boolean> => {
	const old = await oldManifests(root);
	return old.length > 0;
};

/**
 * Migrates the workspace to edition 2024 one edition at a time: `cargo fix
 * --edition` rewrites the code for the next edition, then the manifests
 * declare it. Three rounds take 2015 to 2024. The plan has already named
 * the edition of each package that declared none, so those move as 2015.
 */
export const migrateEditions = async (
	root: string,
	round = 1
): Promise<void> => {
	const old = await oldManifests(root);
	if (old.length === 0 || round > 3) {
		return;
	}
	const fix =
		await $`cargo fix --edition --workspace --all-targets --allow-dirty`
			.cwd(root)
			.nothrow();
	if (fix.exitCode !== 0) {
		const declared = new Set(
			old.flatMap((manifest) => editionsOf(manifest.text))
		);
		declared.delete("2024");
		throw new Error(
			[
				`cargo fix --edition failed moving from edition ${[...declared].join(", ")}, which the manifests still declare.`,
				"Fix the errors it printed, then run bedrock update to go on from there; git checkout undoes the rounds before it."
			].join("\n")
		);
	}
	await Promise.all(
		old.map((manifest) =>
			Bun.write(join(root, manifest.path), raisedEdition(manifest.text))
		)
	);
	// Each round depends on the code the previous one rewrote, so they run in turn.
	await migrateEditions(root, round + 1);
};
