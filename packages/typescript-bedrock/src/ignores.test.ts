import { afterAll, describe, expect, test } from "bun:test";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import {
	isIgnored,
	readBunfigTestIgnores,
	readOxlintIgnores,
} from "./ignores.ts";

const roots: string[] = [];
const repo = async (files: Record<string, string>): Promise<string> => {
	const root = await mkdtemp(join(tmpdir(), "ignores-test-"));
	roots.push(root);
	await Promise.all(
		Object.entries(files).map(([path, content]) =>
			Bun.write(join(root, path), content),
		),
	);
	return root;
};

afterAll(async () => {
	await Promise.all(
		roots.map((root) => rm(root, { recursive: true, force: true })),
	);
});

describe("isIgnored", () => {
	test("matches globs, bare names at any depth, and directories", () => {
		const patterns = ["openspec/**", "**/*.gen.*", "dist/", "/root.ts", "x.ts"];
		expect(isIgnored("openspec/a/b.ts", patterns)).toBe(true);
		expect(isIgnored("a/b.gen.ts", patterns)).toBe(true);
		expect(isIgnored("dist/a.ts", patterns)).toBe(true);
		expect(isIgnored("pkg/dist/a.ts", patterns)).toBe(true);
		expect(isIgnored("root.ts", patterns)).toBe(true);
		expect(isIgnored("sub/root.ts", patterns)).toBe(false);
		expect(isIgnored("sub/x.ts", patterns)).toBe(true);
		expect(isIgnored("src/a.ts", patterns)).toBe(false);
	});

	test("lets a later negation re-include a path", () => {
		expect(isIgnored("a/keep.ts", ["a/**", "!a/keep.ts"])).toBe(false);
		expect(isIgnored("a/drop.ts", ["a/**", "!a/keep.ts"])).toBe(true);
	});
});

describe("readers", () => {
	test("read the oxlint ignorePatterns, comments allowed", async () => {
		const root = await repo({
			".oxlintrc.json": '{ // c\n "ignorePatterns": ["a/**"] }',
		});
		expect(await readOxlintIgnores(root)).toEqual(["a/**"]);
		expect(await readOxlintIgnores(await repo({}))).toEqual([]);
	});

	test("read the bunfig test pathIgnorePatterns, a string or an array", async () => {
		const array = await repo({
			"bunfig.toml": '[test]\npathIgnorePatterns = ["a/**", "b/**"]\n',
		});
		const string = await repo({
			"bunfig.toml": '[test]\npathIgnorePatterns = "a/**"\n',
		});
		expect(await readBunfigTestIgnores(array)).toEqual(["a/**", "b/**"]);
		expect(await readBunfigTestIgnores(string)).toEqual(["a/**"]);
		expect(await readBunfigTestIgnores(await repo({}))).toEqual([]);
	});
});
