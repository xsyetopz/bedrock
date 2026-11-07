import { join } from "node:path";

import type { Step } from "./steps.ts";

const TEST_FILES = new Bun.Glob(
	"**/*.{spec,test}.{cjs,cts,js,jsx,mjs,mts,ts,tsx}",
);
const FRAMEWORK =
	/(\bfrom\s*|\b(?:import|require)\s*\(\s*)(["'])(?:@jest\/globals|vitest)\2/gu;
const GLOBALS = [
	"afterAll",
	"afterEach",
	"beforeAll",
	"beforeEach",
	"describe",
	"expect",
	"it",
	"jest",
	"test",
	"vi",
];
// A call or member access that is not itself a member, so `regex.test(…)` is no use.
const usesGlobal = (text: string, name: string): boolean =>
	new RegExp(`(?<![\\w$.])${name}\\s*[(.]`, "u").test(text);

/**
 * Points a Jest or Vitest test file at bun:test, which has the same API: its
 * framework import changes module, and the globals it calls get imported, since
 * the types declare them only in bun:test.
 */
export const migrateTestFile = (text: string): string => {
	const imported = text.replaceAll(FRAMEWORK, "$1$2bun:test$2");
	if (/["']bun:test["']/u.test(imported)) {
		return imported;
	}
	const used = GLOBALS.filter((name) => usesGlobal(text, name));
	return used.length > 0
		? `import { ${used.join(", ")} } from "bun:test";\n\n${text}`
		: text;
};

/** Rewrites each test file outside node_modules that is not on bun:test yet. */
export const testSteps = async (target: string): Promise<Step[]> => {
	const steps: Step[] = [];
	for await (const path of TEST_FILES.scan({ cwd: target, dot: false })) {
		if (path.split("/").includes("node_modules")) {
			continue;
		}
		const text = await Bun.file(join(target, path)).text();
		const content = migrateTestFile(text);
		if (content !== text) {
			steps.push({ kind: "write", path, content });
		}
	}
	return steps;
};
