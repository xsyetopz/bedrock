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

export type Step =
	| { readonly kind: "remove"; readonly path: string }
	| { readonly kind: "link"; readonly path: string; readonly target: string }
	| {
			readonly kind: "write";
			readonly path: string;
			readonly content: string;
			readonly executable?: boolean;
	  };

const stats = async (path: string) => {
	try {
		return await lstat(path);
	} catch {
		return null;
	}
};

export const describe = async (root: string, step: Step): Promise<string> => {
	if (step.kind === "link") {
		return `link    ${step.path} -> ${step.target}`;
	}
	if (step.kind === "remove") {
		return `remove  ${step.path} (kept as ${step.path}.bak)`;
	}
	return (await stats(join(root, step.path))) === null
		? `create  ${step.path}`
		: `update  ${step.path} (kept as ${step.path}.bak)`;
};

/** Whether `root` already matches the step. */
const isDone = async (root: string, step: Step): Promise<boolean> => {
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
	return (
		meta?.isFile() === true &&
		(await Bun.file(path).text()) === step.content &&
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
 * Applies a step. A file it replaces or removes is kept as `<file>.bak`; an
 * existing backup stays, since it holds the file from before the first adopt.
 */
const applyStep = async (root: string, step: Step): Promise<void> => {
	const path = join(root, step.path);
	if ((await stats(path)) !== null) {
		const backup = `${path}.bak`;
		if ((await stats(backup)) === null) {
			await rename(path, backup);
		} else {
			await rm(path, { recursive: true });
		}
	}
	if (step.kind === "remove") {
		return;
	}
	await mkdir(dirname(path), { recursive: true });
	if (step.kind === "link") {
		await symlink(step.target, path);
		return;
	}
	await Bun.write(path, step.content);
	if (step.executable === true) {
		await chmod(path, 0o755);
	}
};

export const applySteps = async (
	root: string,
	steps: readonly Step[],
): Promise<void> => {
	await Promise.all(steps.map((step) => applyStep(root, step)));
};
