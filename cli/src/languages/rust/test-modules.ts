import { join, relative } from "node:path";

import { readText } from "../../core/fs.ts";
import { childFile, type Module, moduleDirectory, modules } from "./modules.ts";

/** `found` without the modules inside another one's body, which move with it. */
export const outermost = (found: readonly Module[]): Module[] =>
	found.filter(
		({ start }) =>
			!found.some(
				({ body }) => body !== undefined && body[0] < start && start < body[1]
			)
	);

/** The `#[cfg(test)] mod name { … }` modules of `source`, outside any other. */
export const inlineTests = (source: string): Module[] =>
	outermost(
		modules(source).filter(
			(found) => found.testOnly && found.body !== undefined
		)
	);

/** The `#[cfg(test)] #[path = "…"] mod name;` modules of `source`. */
export const pathTests = (source: string): Module[] =>
	modules(source).filter(
		(found) =>
			found.testOnly && found.body === undefined && found.path !== undefined
	);

/**
 * Where the test module `name` of `file` belongs: `foo/tests.rs` for
 * `foo.rs`, and `tests.rs` beside a `lib.rs`, `main.rs`, or `mod.rs`.
 */
export const testsFile = async (
	file: string,
	name = "tests"
): Promise<string> => join(await moduleDirectory(file), `${name}.rs`);

/**
 * The files of `files`, relative to `root`, that compile only under test: each
 * one a `#[cfg(test)] mod name;` declares, and every file below its module
 * directory. Their own test modules already sit outside the code they test.
 */
export const testOnlyFiles = async (
	root: string,
	files: readonly string[]
): Promise<Set<string>> => {
	const declared = await Promise.all(
		files.map(async (file) => {
			const path = join(root, file);
			const source = (await readText(path)) ?? "";
			const children = modules(source).filter(
				(found) => found.testOnly && found.body === undefined
			);
			return Promise.all(children.map((found) => childFile(path, found)));
		})
	);
	const owners = declared
		.flat()
		.filter((path) => path !== undefined)
		.map((path) => relative(root, path));
	const directories = await Promise.all(
		owners.map(
			async (owner) =>
				`${relative(root, await moduleDirectory(join(root, owner)))}/`
		)
	);
	return new Set(
		files.filter(
			(file) =>
				owners.includes(file) ||
				directories.some((directory) => file.startsWith(directory))
		)
	);
};
