import { dirname, join } from "node:path";

import { Glob } from "bun";

import { exists, listFiles, readText } from "../../core/fs.ts";
import { at, isTable, parseToml } from "./manifests.ts";

/** One `Cargo.toml` of the repository. */
export interface Manifest {
	/** Path relative to the repository root. */
	readonly path: string;
	/** Directory of the manifest, "." at the root. */
	readonly dir: string;
	readonly text: string;
	/** Whether it has a `[package]` table. */
	readonly hasPackage: boolean;
	/** Whether it has a `[workspace]` table. */
	readonly hasWorkspace: boolean;
}

/** A file bedrock renames: a `foo/mod.rs` becomes the `foo.rs` beside its directory. */
export interface Move {
	readonly from: string;
	readonly to: string;
}

const MANIFESTS = new Glob("**/Cargo.toml");
const RUST = new Glob("**/*.rs");
const MOD_FILES = new Glob("**/mod.rs");

/** The Rust files git tracks or would track. */
export const rustFiles = (root: string): Promise<string[]> =>
	listFiles(root, RUST);

/**
 * The manifests the repository's own Cargo workspace covers. A directory with
 * its own `[workspace]` below the root is another project, so it and
 * everything under it stay out.
 */
export const manifests = async (root: string): Promise<Manifest[]> => {
	const paths = await listFiles(root, MANIFESTS);
	const found = await Promise.all(
		paths.map(async (path): Promise<Manifest> => {
			const text = (await readText(join(root, path))) ?? "";
			const doc = parseToml(text);
			return {
				path,
				dir: dirname(path),
				text,
				hasPackage: isTable(at(doc, "package")),
				hasWorkspace: isTable(at(doc, "workspace")),
			};
		}),
	);
	const separate = found
		.filter((manifest) => manifest.hasWorkspace && manifest.dir !== ".")
		.map((manifest) => manifest.dir);
	return found.filter(
		(manifest) =>
			!separate.some(
				(dir) => manifest.dir === dir || manifest.dir.startsWith(`${dir}/`),
			),
	);
};

/** Every `foo/mod.rs` under a `src/` directory whose `foo.rs` does not exist yet. */
export const moduleMoves = async (
	root: string,
	found: readonly Manifest[],
): Promise<Move[]> => {
	const sources = found.map((manifest) =>
		manifest.dir === "." ? "src/" : `${manifest.dir}/src/`,
	);
	const listed = await listFiles(root, MOD_FILES);
	const candidates = listed.filter((path) =>
		sources.some(
			(source) => path.startsWith(source) && path !== `${source}mod.rs`,
		),
	);
	const moves = candidates.map((from) => ({
		from,
		to: `${from.slice(0, -"/mod.rs".length)}.rs`,
	}));
	const taken = await Promise.all(
		moves.map((move) => exists(join(root, move.to))),
	);
	return moves.filter((_, index) => taken[index] !== true);
};
