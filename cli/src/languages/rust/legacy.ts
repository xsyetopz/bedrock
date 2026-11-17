import { join } from "node:path";

import { Glob } from "bun";

import { listFiles, readText } from "../../core/fs.ts";
import type { Step } from "../../core/steps.ts";
import { withoutBedrockAlias, withoutBedrockGit } from "./legacy-edits.ts";
import { at, isTable, parseToml } from "./manifests.ts";

/** What a repository adopted by rust-bedrock still carries. */
export interface Legacy {
	readonly steps: readonly Step[];
	readonly notes: readonly string[];
	/** Paths a step edits that stay the project's own. */
	readonly projectFiles: readonly string[];
	/** Whether the xtask crate is bedrock's and goes, so the root manifest drops its member entry. */
	readonly dropXtask: boolean;
}

const XTASK_FILES = new Set(["xtask/Cargo.toml", "xtask/src/main.rs"]);
const DEPENDENCY_TABLES = [
	"dependencies",
	"dev-dependencies",
	"build-dependencies",
];

/** Whether the manifest depends on the rust-bedrock crate, which only bedrock's xtask did. */
const dependsOnBedrock = (text: string): boolean => {
	const doc = parseToml(text);
	return DEPENDENCY_TABLES.some((name) => {
		const table = at(doc, name);
		return isTable(table) && Object.hasOwn(table, "rust-bedrock");
	});
};

/** The xtask crate, if bedrock wrote it: removed whole, with a note for files it did not write. */
const xtaskPlan = async (
	root: string,
): Promise<{ present: boolean; ours: boolean; notes: string[] }> => {
	const manifest = await readText(join(root, "xtask/Cargo.toml"));
	if (manifest === undefined) {
		return { present: false, ours: false, notes: [] };
	}
	if (!dependsOnBedrock(manifest)) {
		return {
			present: true,
			ours: false,
			notes: [
				"xtask does not depend on rust-bedrock, so it stays; remove it by hand if nothing else uses it.",
			],
		};
	}
	const files = await listFiles(root, new Glob("xtask/**"));
	const extra = files.filter((file) => !XTASK_FILES.has(file));
	return {
		present: true,
		ours: true,
		notes: [
			"Cargo.lock still lists xtask and rust-bedrock until cargo runs again (cargo fetch does it).",
			...(extra.length > 0
				? [
						`xtask held files bedrock did not write: ${extra.join(", ")}. They go with the crate; git still has them.`,
					]
				: []),
		],
	};
};

/** `.cargo/config.toml` without the bedrock alias; removed when nothing else is in it. */
const cargoConfigStep = async (
	root: string,
	removable: boolean,
): Promise<Step | undefined> => {
	const text = await readText(join(root, ".cargo/config.toml"));
	if (text === undefined || !removable) {
		return undefined;
	}
	const edited = withoutBedrockAlias(text);
	if (edited === text) {
		return undefined;
	}
	return edited.trim() === ""
		? { kind: "remove", path: ".cargo/config.toml" }
		: { kind: "write", path: ".cargo/config.toml", content: edited };
};

/** `deny.toml` without the `allow-git` entry the xtask dependency needed. */
const denyStep = async (root: string): Promise<Step | undefined> => {
	const text = await readText(join(root, "deny.toml"));
	const edited = text === undefined ? undefined : withoutBedrockGit(text);
	return edited === undefined || edited === text
		? undefined
		: { kind: "write", path: "deny.toml", content: edited };
};

/**
 * What to remove from a repository that rust-bedrock adopted: its xtask crate
 * (only when that depends on rust-bedrock), the `bedrock` cargo alias, and the
 * `deny.toml` entry that allowed the git dependency.
 */
export const legacyPlan = async (root: string): Promise<Legacy> => {
	const xtask = await xtaskPlan(root);
	const [config, deny] = await Promise.all([
		cargoConfigStep(root, xtask.ours || !xtask.present),
		denyStep(root),
	]);
	const edits = [config, deny].filter((step) => step !== undefined);
	const crate: Step[] = xtask.ours ? [{ kind: "remove", path: "xtask" }] : [];
	return {
		steps: [...crate, ...edits],
		notes: xtask.notes,
		projectFiles: edits.map((step) => step.path),
		dropXtask: xtask.ours,
	};
};
