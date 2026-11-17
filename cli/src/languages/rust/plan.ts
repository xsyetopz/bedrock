import { dirname, join } from "node:path";

import { existing, exists, readText } from "../../core/fs.ts";
import type { Context, LanguagePlan } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import { gated, isIntegrationTest } from "./gating.ts";
import { kindsOf, kindSteps, packageName, WASM_TARGET } from "./kinds.ts";
import { legacyPlan } from "./legacy.ts";
import { EMPTY, manifestPart, merge, type Part } from "./manifest-steps.ts";
import { toolchainFile } from "./manifests.ts";
import {
	type Manifest,
	manifests,
	moduleMoves,
	type Move,
	rustFiles,
} from "./project.ts";

/** Files whose job a managed file took over. */
const REPLACED = [
	".cargo/config",
	".clippy.toml",
	".rustfmt.toml",
	"rust-toolchain",
];

/** The toolchain file at the pinned channel; a wasm crate adds its target. */
const toolchainPart = async (
	context: Context,
	kinds: readonly string[],
): Promise<Part> => {
	const template = await context.templates("parts").read("rust-toolchain.toml");
	const current = await readText(join(context.root, "rust-toolchain.toml"));
	const targets = kinds.includes("wasm") ? [WASM_TARGET] : [];
	return {
		steps: [
			{
				kind: "write",
				path: "rust-toolchain.toml",
				content: toolchainFile(template ?? "", current, targets),
			},
		],
		notes: [],
		projectFiles: ["rust-toolchain.toml"],
	};
};

/** Integration tests that lack `#![cfg(test)]`, written with it. */
const testsPart = async (
	root: string,
	found: readonly Manifest[],
): Promise<Part> => {
	const dirs = new Set(found.map((manifest) => manifest.dir));
	const listed = await rustFiles(root);
	const tests = listed.filter(
		(path) => isIntegrationTest(path) && dirs.has(dirname(dirname(path))),
	);
	const steps = await Promise.all(
		tests.map(async (path): Promise<Step | undefined> => {
			const content = gated(await Bun.file(join(root, path)).text());
			return content === undefined
				? undefined
				: { kind: "write", path, content };
		}),
	);
	const written = steps.filter((step) => step !== undefined);
	return {
		steps: written,
		notes: [],
		projectFiles: written.map((step) => step.path),
	};
};

/** `foo/mod.rs` moved to `foo.rs`, as a write and a removal. */
const movesPart = async (
	root: string,
	moves: readonly Move[],
): Promise<Part> => {
	const steps = await Promise.all(
		moves.map(async (move): Promise<Step[]> => [
			{
				kind: "write",
				path: move.to,
				content: await Bun.file(join(root, move.from)).text(),
			},
			{ kind: "remove", path: move.from },
		]),
	);
	return {
		steps: steps.flat(),
		notes: [],
		projectFiles: moves.flatMap((move) => [move.from, move.to]),
	};
};

/** Removes the files whose job a managed file took over. */
const replacedPart = async (root: string): Promise<Part> => {
	const present = await existing(root, REPLACED);
	return present.length === 0
		? EMPTY
		: {
				steps: present.map((path): Step => ({ kind: "remove", path })),
				notes: [
					`Removed ${present.join(", ")}: the toolchain, clippy.toml, rustfmt.toml, and the justfile replace them. Their settings are not carried over; git still has them.`,
				],
				projectFiles: [],
			};
};

/** At init: the crates for each kind, and a README when there is none. */
const initPart = async (
	context: Context,
	kinds: readonly string[],
): Promise<Part> => {
	if (!context.init) {
		return EMPTY;
	}
	const crates = await kindSteps(context, kinds);
	const readme = await context
		.templates("parts")
		.read("README.md", { name: packageName(context.root) });
	const missing = !(await exists(join(context.root, "README.md")));
	return {
		steps: [
			...crates.steps,
			...(readme === undefined || !missing
				? []
				: [{ kind: "write" as const, path: "README.md", content: readme }]),
		],
		notes: crates.notes,
		projectFiles: [],
	};
};

/**
 * Plans the Rust part of an adopt: manifests on the workspace lints, the
 * pinned toolchain, test files and modules in the layout the guard checks,
 * what older rust-bedrock releases left behind, and at init the crates.
 */
export const planRust = async (context: Context): Promise<LanguagePlan> => {
	const kinds = context.init ? kindsOf(context.options) : [];
	const legacy = await legacyPlan(context.root);
	const all = await manifests(context.root);
	const covered = legacy.dropXtask
		? all.filter(
				(manifest) =>
					manifest.dir !== "xtask" && !manifest.dir.startsWith("xtask/"),
			)
		: all;
	const moves = await moduleMoves(context.root, covered);
	return merge(
		await Promise.all([
			toolchainPart(context, kinds),
			manifestPart(context, covered, legacy.dropXtask),
			testsPart(context.root, covered),
			movesPart(context.root, moves),
			replacedPart(context.root),
			initPart(context, kinds),
			Promise.resolve(legacy),
		]),
	);
};
