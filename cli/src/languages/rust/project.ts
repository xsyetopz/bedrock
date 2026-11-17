import { basename, dirname, join } from "node:path";

import { Glob } from "bun";

import { exists, listFiles, readText } from "../../core/fs.ts";
import { at, isTable, parseToml } from "./manifests.ts";
import { prefixPaths } from "./mod-paths.ts";

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
	/** The new text, when the rename changes what its relative paths mean. */
	readonly content?: string;
}

/** The renames to make and the files left in place because a rename would break them. */
export interface Moves {
	readonly moves: Move[];
	/** Paths of `mod.rs` files whose `#[path]` or `include!` cannot be rewritten safely. */
	readonly kept: string[];
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

/**
 * Every `foo/mod.rs` under a `src/` directory whose `foo.rs` does not exist
 * yet. Moving the file up a directory changes what its relative `#[path]`
 * and `include!` paths reach, so those gain a `foo/` prefix; a file whose
 * paths cannot be rewritten safely stays where it is.
 */
export const moduleMoves = async (
	root: string,
	found: readonly Manifest[],
): Promise<Moves> => {
	const sources = found.map((manifest) =>
		manifest.dir === "." ? "src/" : `${manifest.dir}/src/`,
	);
	const listed = await listFiles(root, MOD_FILES);
	const candidates = listed.filter((path) =>
		sources.some(
			(source) => path.startsWith(source) && path !== `${source}mod.rs`,
		),
	);
	const free = await Promise.all(
		candidates.map(async (from) => {
			const to = `${from.slice(0, -"/mod.rs".length)}.rs`;
			return (await exists(join(root, to))) ? undefined : { from, to };
		}),
	);
	const wanted = free.filter((move) => move !== undefined);
	const planned = await Promise.all(
		wanted.map(async (move): Promise<Move | undefined> => {
			const text = await Bun.file(join(root, move.from)).text();
			const prefixed = prefixPaths(text, `${basename(dirname(move.from))}/`);
			if (prefixed === undefined) {
				return undefined;
			}
			return prefixed === text ? move : { ...move, content: prefixed };
		}),
	);
	return {
		moves: planned.filter((move) => move !== undefined),
		kept: wanted
			.filter((_, index) => planned[index] === undefined)
			.map((move) => move.from),
	};
};
