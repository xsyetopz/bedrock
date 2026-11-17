import {
	chmod,
	lstat,
	mkdir,
	readlink,
	rename,
	rm,
	symlink,
} from "node:fs/promises";
import { dirname, join } from "node:path";

/** One change to a repository, relative to its root. */
export type Step =
	| { readonly kind: "remove"; readonly path: string }
	| { readonly kind: "link"; readonly path: string; readonly target: string }
	/** Renames `from` to `path`, keeping its content and mode. */
	| { readonly kind: "move"; readonly path: string; readonly from: string }
	| {
			readonly kind: "write";
			readonly path: string;
			/** Text, or the bytes of a binary file such as a jar. */
			readonly content: string | Uint8Array;
			readonly executable?: boolean;
	  };

const stats = async (path: string) => {
	try {
		return await lstat(path);
	} catch {
		return null;
	}
};

/** One plan line: the verb is padded so paths align. */
export const describe = async (root: string, step: Step): Promise<string> => {
	if (step.kind === "link") {
		return `link    ${step.path} -> ${step.target}`;
	}
	if (step.kind === "remove") {
		return `remove  ${step.path}`;
	}
	if (step.kind === "move") {
		return `move    ${step.from} -> ${step.path}`;
	}
	return (await stats(join(root, step.path))) === null
		? `create  ${step.path}`
		: `update  ${step.path}`;
};

/** Whether the file at `path` holds exactly `content`. */
const holds = async (
	path: string,
	content: string | Uint8Array,
): Promise<boolean> => {
	const file = Bun.file(path);
	return typeof content === "string"
		? (await file.text()) === content
		: Bun.deepEquals(await file.bytes(), content);
};

/** Whether `root` already matches the step. */
export const isDone = async (root: string, step: Step): Promise<boolean> => {
	const path = join(root, step.path);
	const meta = await stats(path);
	if (step.kind === "remove") {
		return meta === null;
	}
	if (step.kind === "link") {
		return (
			meta?.isSymbolicLink() === true && (await readlink(path)) === step.target
		);
	}
	if (step.kind === "move") {
		return meta !== null && (await stats(join(root, step.from))) === null;
	}
	return (
		meta?.isFile() === true &&
		(await holds(path, step.content)) &&
		(step.executable !== true || (meta.mode & 0o111) !== 0)
	);
};

/** Drops the steps `root` already matches. */
export const pending = async (
	root: string,
	steps: readonly Step[],
): Promise<Step[]> => {
	const done = await Promise.all(steps.map((step) => isDone(root, step)));
	return steps.filter((_, index) => done[index] !== true);
};

/**
 * Applies a step in place. Nothing is backed up: adopt runs only when git
 * holds every file a step touches, so `git diff` reviews the change and
 * `git checkout` undoes it.
 */
const applyStep = async (root: string, step: Step): Promise<void> => {
	const path = join(root, step.path);
	const meta = await stats(path);
	// A write over a file replaces its content in place, keeping its mode.
	if (step.kind !== "write" || meta?.isFile() !== true) {
		await rm(path, { recursive: true, force: true });
	}
	if (step.kind === "remove") {
		return;
	}
	await mkdir(dirname(path), { recursive: true });
	if (step.kind === "move") {
		await rename(join(root, step.from), path);
		return;
	}
	if (step.kind === "link") {
		await symlink(step.target, path);
		return;
	}
	await Bun.write(path, step.content);
	if (step.executable === true) {
		await chmod(path, 0o755);
	}
};

/**
 * Applies steps that {@link dedupe} left one per path. Removals go first, so
 * a file can take the place of a directory a removal clears.
 */
export const applySteps = async (
	root: string,
	steps: readonly Step[],
): Promise<void> => {
	const removals = steps.filter((step) => step.kind === "remove");
	await Promise.all(removals.map((step) => applyStep(root, step)));
	const writes = steps.filter((step) => step.kind !== "remove");
	await Promise.all(writes.map((step) => applyStep(root, step)));
};

/**
 * Keeps the last step for each path, so a language's file replaces the
 * base file at the same path, and orders the result by path.
 */
export const dedupe = (steps: readonly Step[]): Step[] =>
	[...new Map(steps.map((step) => [step.path, step])).values()].toSorted(
		(left, right) => left.path.localeCompare(right.path),
	);
