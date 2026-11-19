import { dirname, join } from "node:path";

import { exists } from "../../core/fs.ts";
import type { Finding } from "../../core/language.ts";
import { inOrder } from "../../core/order.ts";
import { gated, isIntegrationTest } from "./gating.ts";
import { moveOut } from "./move-out.ts";
import { manifests } from "./project.ts";
import { testOnlyFiles } from "./test-modules.ts";

/** What one file needs: its test modules moved out, its integration test gated, or both. */
interface Needs {
	readonly testOnly: boolean;
	readonly integration: boolean;
}

/** Repairs one file; returns what it could not move or gate. */
const repairFile = async (
	root: string,
	file: string,
	{ testOnly, integration }: Needs
): Promise<Finding[]> => {
	const path = join(root, file);
	// Moving an earlier file's tests can remove a later one in the list.
	if (!(await exists(path))) {
		return [];
	}
	// A test-only file may hold test modules, so only the others move.
	const problems = testOnly ? [] : await moveOut(path);
	try {
		if (integration) {
			const content = gated(await Bun.file(path).text());
			if (content !== undefined) {
				await Bun.write(path, content);
			}
		}
	} catch (error) {
		problems.push(`${path}: cannot gate: ${String(error)}`);
	}
	return problems.map((problem) => problem.replaceAll(`${root}/`, ""));
};

/**
 * Moves the test modules of `files`, relative to `root`, out to their own
 * files and gates each crate's integration tests in `tests/`, one file at a
 * time. Returns what it could not move or gate.
 */
export const repairRust = async (
	root: string,
	files: readonly string[]
): Promise<Finding[]> => {
	const testOnly = await testOnlyFiles(root, files);
	const found = await manifests(root);
	const crates = new Set(found.map(({ dir }) => dir));
	const unrepaired: Finding[] = [];
	await inOrder(files.toSorted(), async (file) => {
		// `src/tests/x.rs` under a `tests` module is no integration test.
		const integration =
			isIntegrationTest(file) && crates.has(dirname(dirname(file)));
		const needs = { testOnly: testOnly.has(file), integration };
		unrepaired.push(...(await repairFile(root, file, needs)));
	});
	return unrepaired;
};
