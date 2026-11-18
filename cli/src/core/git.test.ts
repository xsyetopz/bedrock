import { afterAll, describe, expect, test } from "bun:test";
import { rm } from "node:fs/promises";
import { join } from "node:path";

import { $ } from "bun";

import { hooksOn, unpushedPaths } from "./git.ts";
import { tempRepo } from "./testing.ts";

const roots: string[] = [];

const repo = async (files: Readonly<Record<string, string>> = {}) => {
	const root = await tempRepo(files);
	roots.push(root);
	return root;
};

const commit = async (
	root: string,
	files: Readonly<Record<string, string>>,
) => {
	await Promise.all(
		Object.entries(files).map(([path, content]) =>
			Bun.write(join(root, path), content),
		),
	);
	await $`git add -A`.cwd(root).quiet();
	await $`git -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q -m next --no-verify`
		.cwd(root)
		.quiet();
};

/** Marks HEAD as pushed, as a fetch of `origin/main` would. */
const pushed = async (root: string) => {
	await $`git update-ref refs/remotes/origin/main HEAD`.cwd(root).quiet();
};

const lefthookHook = (hook: string) =>
	`#!/bin/sh\ncall_lefthook()\n{\n  lefthook "$@"\n}\n\ncall_lefthook run "${hook}" "$@"\n`;

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

describe("unpushedPaths", () => {
	test("lists the paths the commits no remote has change", async () => {
		// Arrange
		const root = await repo({ "a.ts": "a", "b.rs": "b" });
		await pushed(root);
		await commit(root, { "b.rs": "changed", "c.go": "c" });

		// Act
		const paths = await unpushedPaths(root);

		// Assert
		expect(paths).toEqual(["b.rs", "c.go"]);
	});

	test("is empty when a remote has every commit", async () => {
		// Arrange
		const root = await repo({ "a.ts": "a" });
		await pushed(root);

		// Act
		const paths = await unpushedPaths(root);

		// Assert
		expect(paths).toEqual([]);
	});

	test("is undefined when no remote has any commit", async () => {
		// Arrange
		const root = await repo({ "a.ts": "a" });

		// Act
		const paths = await unpushedPaths(root);

		// Assert
		expect(paths).toBeUndefined();
	});
});

describe("hooksOn", () => {
	test("is true when lefthook runs every hook", async () => {
		// Arrange
		const root = await repo({
			".git/hooks/pre-commit": lefthookHook("pre-commit"),
			".git/hooks/commit-msg": lefthookHook("commit-msg"),
			".git/hooks/pre-push": lefthookHook("pre-push"),
		});

		// Act
		const on = await hooksOn(root);

		// Assert
		expect(on).toBe(true);
	});

	test("is false when a hook is missing", async () => {
		// Arrange
		const root = await repo({
			".git/hooks/pre-commit": lefthookHook("pre-commit"),
			".git/hooks/commit-msg": lefthookHook("commit-msg"),
		});

		// Act
		const on = await hooksOn(root);

		// Assert
		expect(on).toBe(false);
	});

	test("is false when core.hooksPath still points at .githooks", async () => {
		// Arrange
		const root = await repo({
			".git/hooks/pre-commit": lefthookHook("pre-commit"),
			".git/hooks/commit-msg": lefthookHook("commit-msg"),
			".git/hooks/pre-push": lefthookHook("pre-push"),
		});
		await $`git config core.hooksPath .githooks`.cwd(root).quiet();

		// Act
		const on = await hooksOn(root);

		// Assert
		expect(on).toBe(false);
	});
});
