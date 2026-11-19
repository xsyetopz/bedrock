import { mkdir } from "node:fs/promises";

import { $ } from "bun";

import { exists } from "../core/fs.ts";
import { isRepository } from "../core/git.ts";
import { parseNamed, resolveRoot } from "./select.ts";

/** Starts `root` for `init`: it must be empty, and becomes a git repository. */
const prepareInit = async (root: string): Promise<void> => {
	if (await exists(root)) {
		const entries = await Array.fromAsync(
			new Bun.Glob("*").scan({ cwd: root, dot: true, onlyFiles: false })
		);
		if (entries.length > 0) {
			throw new Error(`${root} is not empty. Run bedrock adopt there instead.`);
		}
	}
	await mkdir(root, { recursive: true });
	await $`git init --quiet`.cwd(root);
};

/** Makes `root` ready: a fresh repository for init, an existing one otherwise. */
export const prepare = async (
	mode: Mode,
	root: string,
	dryRun: boolean
): Promise<void> => {
	if (mode === "init") {
		if (!dryRun) {
			await prepareInit(root);
		}
		return;
	}
	if (!(await isRepository(root))) {
		throw new Error(
			`${root} is not a git repository. bedrock changes files in place and relies on git to review and undo them; run git init and commit first.`
		);
	}
};

/** Which entry point runs: they differ in where the language list comes from. */
export type Mode = "adopt" | "init" | "update";

const OPTIONS = `
Options:
  --cxx          Add C and C++ (same as naming cpp).
  --dry-run      Print the plan. Change nothing.
  --group <pkg>  The JVM group and package, such as com.example.app; kept in .editorconfig.
  --yes          Apply without asking (agents, CI).
  --no-run       Skip the installs that follow, such as bun install.
  --allow-dirty  Apply even where a changed file holds uncommitted work.
  -h, --help     Show this help.`;

/** Help text for each mode. */
export const USAGE: Readonly<Record<Mode, string>> = {
	adopt: `Usage: bedrock adopt [<dir>] [<language>[:<kind>]...] [options]

Moves an existing repo onto bedrock: writes the shared justfile, hooks, and
configs, migrates each language's tooling, and removes the tools it replaces.
Without a language, it adopts the ones .editorconfig lists or the repo has.${OPTIONS}`,
	init: `Usage: bedrock init <dir> <language>[:<kind>]... [options]

Starts a repo in <dir>, which must be empty or missing, then adopts it.${OPTIONS}`,
	update: `Usage: bedrock update [<dir>] [options]

Rewrites every managed file for the languages .editorconfig lists, with the
templates of this bedrock release.${OPTIONS}`
};

/** Whether `arg` names a language rather than an existing folder. */
const isLanguageName = async (arg: string): Promise<boolean> => {
	try {
		parseNamed(arg);
	} catch {
		return false;
	}
	return !(await exists(resolveRoot(arg)));
};

/**
 * Splits the positionals into the repository and the named languages. A
 * first positional that names a language, not a folder, means the current
 * folder.
 */
export const target = async (
	mode: Mode,
	positionals: readonly string[]
): Promise<{ root: string; names: readonly string[] }> => {
	const [first, ...rest] = positionals;
	const firstIsTarget =
		mode === "init" || (first !== undefined && !(await isLanguageName(first)));
	const names = firstIsTarget ? rest : positionals;
	if (mode === "update" && names.length > 0) {
		throw new Error(
			"bedrock update takes no languages. Use bedrock adopt to add one."
		);
	}
	return { root: resolveRoot(firstIsTarget ? first : undefined), names };
};
