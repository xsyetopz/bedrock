import { join } from "node:path";

import { Glob } from "bun";

import { BLOCK_ORDER, removeBlock } from "../../core/blocks.ts";
import { existing, listFiles, readText } from "../../core/fs.ts";
import type { LanguagePlan } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";

/** Files of tools that swift-format, SwiftLint, and SwiftPM replaced. */
const REPLACED = [".swiftformat", ".swiftlint.yaml", "Mintfile"];

/** Files the project owns that an old bedrock pointed at BuildTools. */
const PROJECT_FILES = [
	"project.just",
	".gitignore",
	".just/*.just",
	".github/**/*.{yml,yaml}",
];

/** Whether the old bedrock wrote BuildTools: its package depends on swift-bedrock. */
const isBedrockBuildTools = async (root: string): Promise<boolean> => {
	const manifest = await readText(join(root, "BuildTools/Package.swift"));
	return manifest?.includes("swift-bedrock") ?? false;
};

/** The text of a file without the blocks bedrock keeps up to date. */
const outsideBlocks = (text: string): string => {
	let rest = text;
	for (const name of BLOCK_ORDER) {
		rest = removeBlock(rest, "hash", name);
	}
	return rest;
};

const referenceNotes = async (root: string): Promise<string[]> => {
	const listed = await Promise.all(
		PROJECT_FILES.map((pattern) => listFiles(root, new Glob(pattern))),
	);
	const paths = listed
		.flat()
		.filter(
			(path) =>
				!/^\.just\/(?:swift|cpp)\.just$|^\.github\/workflows\/swift\.yml$/u.test(
					path,
				),
		);
	const notes = await Promise.all(
		paths.map(async (path) => {
			const text = await readText(join(root, path));
			return text !== undefined && outsideBlocks(text).includes("BuildTools")
				? [
						`${path}: mentions BuildTools, which bedrock no longer uses; remove the reference`,
					]
				: [];
		}),
	);
	return notes.flat();
};

/**
 * What an old swift-bedrock left behind: the BuildTools package it wrote, the
 * files of tools it replaced, and the project files that still point at it.
 */
export const legacyPlan = async (root: string): Promise<LanguagePlan> => {
	const [legacy, replaced] = await Promise.all([
		isBedrockBuildTools(root),
		existing(root, REPLACED),
	]);
	const steps: Step[] = replaced.map((path) => ({ kind: "remove", path }));
	if (!legacy) {
		return { steps, notes: [], projectFiles: [] };
	}
	steps.push({ kind: "remove", path: "BuildTools" });
	return { steps, notes: await referenceNotes(root), projectFiles: [] };
};
