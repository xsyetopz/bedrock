import { mkdir, realpath, unlink } from "node:fs/promises";
import { dirname, join } from "node:path";

import { exists } from "../../core/fs.ts";
import { prefixPaths } from "./mod-paths.ts";
import { childFile, type Module, moduleDirectory, modules } from "./modules.ts";

/** The `#[cfg(test)] mod name { … }` modules of `source`. */
export const inlineTests = (source: string): Module[] =>
	modules(source).filter((found) => found.testOnly && found.body !== undefined);

/** The `#[cfg(test)] #[path = "…"] mod name;` modules of `source`. */
export const pathTests = (source: string): Module[] =>
	modules(source).filter(
		(found) =>
			found.testOnly && found.body === undefined && found.path !== undefined,
	);

/**
 * Where the tests of `file` belong: `foo/tests.rs` for `foo.rs`, and
 * `tests.rs` beside a `lib.rs`, `main.rs`, or `mod.rs`.
 */
export const testsFile = async (file: string): Promise<string> =>
	join(await moduleDirectory(file), "tests.rs");

/** What moving the test modules of one file needs. */
interface Plan {
	/** Ranges of the parent's text and what replaces them. */
	readonly edits: { from: number; to: number; text: string }[];
	readonly writes: { path: string; text: string }[];
	readonly removals: string[];
	readonly problems: string[];
}

const failure = (operation: string, path: string, error: unknown): string =>
	`${path}: cannot ${operation}: ${error instanceof Error ? error.message : String(error)}`;

const sameFile = async (left: string, right: string): Promise<boolean> => {
	try {
		return (await realpath(left)) === (await realpath(right));
	} catch {
		return false;
	}
};

/** The offset past `end` and the whitespace that follows it. */
const pastSpace = (source: string, end: number): number =>
	end + (/^\s*/u.exec(source.slice(end))?.[0].length ?? 0);

/** Plans the move of a `#[path]` test file to `destination`. */
const planPathModule = async (
	file: string,
	source: string,
	found: Module,
	destination: string,
	plan: Plan,
): Promise<void> => {
	const place = `${file}:${found.line}`;
	const current = await childFile(file, found);
	if (found.pathAttribute === undefined || current === undefined) {
		plan.problems.push(`${place}: cannot find the file named by \`#[path]\``);
		return;
	}
	const [from, to] = found.pathAttribute;
	const dropAttribute = { from, to: pastSpace(source, to), text: "" };
	if (await sameFile(current, destination)) {
		plan.edits.push(dropAttribute);
		return;
	}
	let text: string;
	try {
		text = await Bun.file(current).text();
	} catch (error) {
		plan.problems.push(failure("read", current, error));
		return;
	}
	const taken =
		(await exists(destination)) ||
		plan.writes.some((write) => write.path === destination);
	// A relative path inside the file would resolve from another directory.
	const rewritable = prefixPaths(text, "moved/") === text;
	if (taken) {
		plan.problems.push(
			`${place}: cannot move ${current}: ${destination} exists`,
		);
	} else if (rewritable) {
		plan.edits.push(dropAttribute);
		plan.writes.push({ path: destination, text });
		plan.removals.push(current);
	} else {
		plan.problems.push(
			`${place}: cannot move ${current}: it names files by relative path, which would stop resolving`,
		);
	}
};

/** Plans the move of one top-level test module, or reports why it cannot move. */
const planModule = async (
	file: string,
	source: string,
	found: Module,
	destination: string,
	plan: Plan,
): Promise<void> => {
	const place = `${file}:${found.line}`;
	if (found.depth > 0 || found.name !== "tests") {
		plan.problems.push(
			`${place}: cannot move \`mod ${found.name}\`; declare \`#[cfg(test)] mod tests;\` at the top level of the file`,
		);
		return;
	}
	if (found.body === undefined) {
		await planPathModule(file, source, found, destination, plan);
		return;
	}
	const taken =
		(await exists(destination)) ||
		plan.writes.some((write) => write.path === destination);
	if (taken) {
		plan.problems.push(
			`${place}: cannot move the tests: ${destination} exists`,
		);
		return;
	}
	const [open, close] = found.body;
	plan.writes.push({
		path: destination,
		text: source.slice(open + 1, close).replace(/^[\r\n]+/u, ""),
	});
	plan.edits.push({ from: found.start, to: close + 1, text: "mod tests;" });
};

const planFile = async (file: string, source: string): Promise<Plan> => {
	const plan: Plan = { edits: [], writes: [], removals: [], problems: [] };
	const destination = await testsFile(file);
	const movable = modules(source).filter(
		(found) =>
			found.testOnly && (found.body !== undefined || found.path !== undefined),
	);
	// One at a time: each module's check sees the files the earlier ones claim.
	const planFrom = async (rest: readonly Module[]): Promise<void> => {
		const [head, ...others] = rest;
		if (head !== undefined) {
			await planModule(file, source, head, destination, plan);
			await planFrom(others);
		}
	};
	await planFrom(movable);
	return plan;
};

/** The message for a failed `attempt`, or `undefined` when it succeeded. */
const attempted = async (
	operation: string,
	path: string,
	attempt: () => Promise<unknown>,
): Promise<string | undefined> => {
	try {
		await attempt();
		return undefined;
	} catch (error) {
		return failure(operation, path, error);
	}
};

const present = (messages: (string | undefined)[]): string[] =>
	messages.filter((message) => message !== undefined);

/**
 * Moves each test module of `file` to the `tests.rs` beside it and leaves
 * `#[cfg(test)] mod tests;`, dropping a `#[path]` attribute on the way.
 * Returns one message per module it cannot move, or per I/O failure; the other
 * modules still move.
 */
export const moveOut = async (file: string): Promise<string[]> => {
	let source: string;
	try {
		source = await Bun.file(file).text();
	} catch (error) {
		return [failure("read", file, error)];
	}
	const { edits, writes, removals, problems } = await planFile(file, source);
	if (edits.length === 0) {
		return problems;
	}
	const written = present(
		await Promise.all(
			writes.map((write) =>
				attempted("write", write.path, async () => {
					await mkdir(dirname(write.path), { recursive: true });
					await Bun.write(write.path, write.text);
				}),
			),
		),
	);
	if (written.length > 0) {
		return [...problems, ...written];
	}
	let updated = source;
	for (const { from, to, text } of edits.toSorted((a, b) => b.from - a.from)) {
		updated = updated.slice(0, from) + text + updated.slice(to);
	}
	const parent = await attempted("write", file, () => Bun.write(file, updated));
	if (parent !== undefined) {
		return [...problems, parent];
	}
	const removed = await Promise.all(
		removals.map((old) => attempted("remove", old, () => unlink(old))),
	);
	return [...problems, ...present(removed)];
};
