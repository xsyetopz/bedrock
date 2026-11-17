import { join } from "node:path";

import { Glob } from "bun";

import { listFiles, readText } from "../../core/fs.ts";
import type { LanguagePlan } from "../../core/language.ts";
import type { Step } from "../../core/steps.ts";
import { importsXCTest, isUITest } from "./suppressions.ts";
import { convertXCTest } from "./xctest/convert.ts";

/** The Swift files outside `*UITests` directories that import XCTest. */
export const xctestFiles = async (root: string): Promise<string[]> => {
	const listed = await listFiles(root, new Glob("**/*.swift"));
	const paths = listed.filter((path) => !isUITest(path));
	const texts = await Promise.all(
		paths.map((path) => readText(join(root, path))),
	);
	return paths.filter((_, index) => importsXCTest(texts[index] ?? ""));
};

/** Steps that convert each XCTest unit-test file to Swift Testing, with what is left for a person. */
export const conversionPlan = async (root: string): Promise<LanguagePlan> => {
	const paths = await xctestFiles(root);
	const converted = await Promise.all(
		paths.map(async (path) => ({
			path,
			result: convertXCTest((await readText(join(root, path))) ?? ""),
		})),
	);
	const steps: Step[] = [];
	const notes: string[] = [];
	for (const { path, result } of converted) {
		if (result !== undefined) {
			steps.push({ kind: "write", path, content: result.text });
			notes.push(...result.notes.map((note) => `${path}: ${note}`));
		}
	}
	return { steps, notes, projectFiles: steps.map((step) => step.path) };
};
