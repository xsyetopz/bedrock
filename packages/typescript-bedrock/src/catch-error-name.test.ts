import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { $ } from "bun";

const oxlint = join(
	dirname(fileURLToPath(import.meta.resolve("oxlint/package.json"))),
	"bin/oxlint",
);
const shared = join(import.meta.dir, "..", "oxlintrc.json");

const roots: string[] = [];
const workspace = async (): Promise<string> => {
	const root = await mkdtemp(join(tmpdir(), "catch-name-test-"));
	roots.push(root);
	await Bun.write(
		join(root, ".oxlintrc.json"),
		JSON.stringify({
			extends: [shared],
			options: { typeAware: false },
		}),
	);
	return root;
};

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

/** Whether the shared config reports `unicorn/catch-error-name` for `source`. */
const isFlagged = async (source: string): Promise<boolean> => {
	const root = await workspace();
	await Bun.write(join(root, "sample.ts"), source);
	const result = await $`bun ${oxlint} -c .oxlintrc.json sample.ts`
		.cwd(root)
		.nothrow()
		.quiet();
	return result.stdout.toString().includes("catch-error-name");
};

const HOOK = "declare const hook: { catch(handler: unknown): void };\n";

describe("unicorn/catch-error-name", () => {
	test("accepts the engine `$` as the first parameter of a hook `.catch`", async () => {
		expect(
			await isFlagged(`${HOOK}hook.catch(($, e, next) => [$, e, next]);\n`),
		).toBe(false);
	});

	test("accepts `_$` as the first parameter of a hook `.catch`", async () => {
		expect(
			await isFlagged(`${HOOK}hook.catch((_$, e, next) => [e, next]);\n`),
		).toBe(false);
	});

	test("still rejects `e` as the name of a caught error", async () => {
		expect(await isFlagged(`${HOOK}hook.catch((e) => [e]);\n`)).toBe(true);
	});

	test("still rejects `engine` as the first parameter", async () => {
		expect(await isFlagged(`${HOOK}hook.catch((engine) => [engine]);\n`)).toBe(
			true,
		);
	});
});
