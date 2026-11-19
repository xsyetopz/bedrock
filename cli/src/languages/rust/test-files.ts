import { join } from "node:path";

import { readText } from "../../core/fs.ts";
import { hasGate } from "./gating.ts";
import { manifests } from "./project.ts";
import { testOnlyFiles } from "./test-modules.ts";

/**
 * Whether `file` sits in a crate's `tests/` directory, whose files Cargo
 * builds only as integration tests and their modules.
 */
const inIntegrationTests = (
	file: string,
	crates: ReadonlySet<string>,
): boolean => {
	const parts = file.split("/");
	return parts.some(
		(part, index) =>
			part === "tests" &&
			index < parts.length - 1 &&
			crates.has(index === 0 ? "." : parts.slice(0, index).join("/")),
	);
};

/**
 * The files of `files`, relative to `root`, that rustc compiles only under
 * test: those a `#[cfg(test)] mod` declares and the files below them, those
 * `#![cfg(test)]` gates, and each crate's integration tests. A `#[test]`
 * function alone does not count: rustc still builds the rest of its file.
 */
export const rustTestFiles = async (
	root: string,
	files: readonly string[],
): Promise<Set<string>> => {
	const found = await manifests(root);
	const crates = new Set(found.map(({ dir }) => dir));
	const testOnly = await testOnlyFiles(root, files);
	const picked = await Promise.all(
		files.map(async (file) => {
			if (testOnly.has(file) || inIntegrationTests(file, crates)) {
				return [file];
			}
			const source = await readText(join(root, file));
			return source !== undefined && hasGate(source) ? [file] : [];
		}),
	);
	return new Set(picked.flat());
};
