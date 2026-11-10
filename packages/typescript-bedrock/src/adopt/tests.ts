import { join } from "node:path";

import { listFiles } from "../files.ts";
import { readBunfigTestIgnores } from "../ignores.ts";
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

/**
 * Rewrites each test file that is not on bun:test yet, except those in node_modules,
 * those git ignores, and those bunfig.toml keeps `bun test` from running.
 */
export const testSteps = async (target: string): Promise<Step[]> => {
	const paths = await listFiles(
		target,
		TEST_FILES,
		await readBunfigTestIgnores(target),
	);
	const texts = await Promise.all(
		paths.map((path) => Bun.file(join(target, path)).text()),
	);
	return paths.flatMap((path, index): Step[] => {
		const text = texts[index] ?? "";
		const content = migrateTestFile(text);
		return content === text ? [] : [{ kind: "write", path, content }];
	});
};
