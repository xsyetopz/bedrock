import { join } from "node:path";

import { $ } from "bun";

import { manifests } from "./project.ts";

/** Each edition `cargo fix --edition` migrates code from, with the one it migrates to. */
const NEXT: Readonly<Record<string, string>> = {
	"2015": "2018",
	"2018": "2021",
	"2021": "2024",
};

/** An `edition = "<year>"` line older than 2024, at a package, workspace package, or target. */
const OLD_EDITION = /^(\s*edition\s*=\s*)"(2015|2018|2021)"/gmu;

/** The manifest with every declared edition older than 2024 raised by one step. */
export const raisedEdition = (text: string): string =>
	text.replaceAll(
		OLD_EDITION,
		(_, key: string, year: string) => `${key}"${NEXT[year] ?? year}"`,
	);

/**
 * Migrates the workspace to edition 2024 one edition at a time: `cargo fix
 * --edition` rewrites the code for the next edition, then the manifests
 * declare it. Three rounds take 2015 to 2024. A package that declares no
 * edition stays out, since Cargo reads that as 2015 and the plan notes it.
 */
export const migrateEditions = async (
	root: string,
	round = 1,
): Promise<void> => {
	const found = await manifests(root);
	const old = found.filter(
		(manifest) => raisedEdition(manifest.text) !== manifest.text,
	);
	if (old.length === 0 || round > 3) {
		return;
	}
	const fix =
		await $`cargo fix --edition --workspace --all-targets --allow-dirty`
			.cwd(root)
			.nothrow();
	if (fix.exitCode !== 0) {
		throw new Error(
			`cargo fix --edition failed in round ${round}; fix the errors it printed, then run bedrock adopt again.`,
		);
	}
	await Promise.all(
		old.map((manifest) =>
			Bun.write(join(root, manifest.path), raisedEdition(manifest.text)),
		),
	);
	// Each round depends on the code the previous one rewrote, so they run in turn.
	await migrateEditions(root, round + 1);
};
