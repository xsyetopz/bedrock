import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { isTypeAware, withLock } from "./lint.ts";

const roots: string[] = [];
const lockPath = async (): Promise<string> => {
	const root = await mkdtemp(join(tmpdir(), "lint-test-"));
	roots.push(root);
	return join(root, "lock");
};

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

describe("isTypeAware", () => {
	test("is true for a lint run and false for a query", () => {
		expect(isTypeAware(["src"])).toBe(true);
		expect(isTypeAware([])).toBe(true);
		expect(isTypeAware(["--fix", "."])).toBe(true);
		expect(isTypeAware(["--help"])).toBe(false);
		expect(isTypeAware(["-V"])).toBe(false);
		expect(isTypeAware(["--version"])).toBe(false);
		expect(isTypeAware(["--rules"])).toBe(false);
		expect(isTypeAware(["--print-config"])).toBe(false);
	});
});

describe("withLock", () => {
	test("runs two holders one after the other and tells the second to wait", async () => {
		const path = await lockPath();
		const events: string[] = [];
		const messages: string[] = [];
		const hold = (name: string) =>
			withLock(
				path,
				async () => {
					events.push(`${name}:start`);
					await Bun.sleep(150);
					events.push(`${name}:end`);
					return name;
				},
				{ onWait: (message) => messages.push(message), pollMs: 20 },
			);
		const results = await Promise.all([hold("a"), hold("b")]);
		expect(results).toEqual(["a", "b"]);
		expect(events).toEqual(["a:start", "a:end", "b:start", "b:end"]);
		expect(messages).toHaveLength(1);
		expect(messages[0]).toContain("waiting");
	});

	test("releases the lock when the task throws", async () => {
		const path = await lockPath();
		const failure = await withLock(path, () => {
			throw new Error("boom");
		}).catch((error: unknown) => error);
		expect(failure).toBeInstanceOf(Error);
		const result = await withLock(path, () => Promise.resolve("free"), {
			pollMs: 20,
		});
		expect(result).toBe("free");
	});

	test("breaks a lock whose owner is gone", async () => {
		const path = await lockPath();
		// 2 ** 22 exceeds the pid limit of macOS and Linux, so no process owns it.
		await Bun.write(join(path, "pid"), String(2 ** 22));
		const messages: string[] = [];
		const result = await withLock(path, () => Promise.resolve("ran"), {
			onWait: (message) => messages.push(message),
			pollMs: 20,
		});
		expect(result).toBe("ran");
		expect(messages).toEqual([]);
	});
});
