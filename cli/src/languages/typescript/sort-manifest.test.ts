import { describe, expect, test } from "bun:test";

import { asRecord } from "./json.ts";
import { sortManifest } from "./sort-manifest.ts";
import { BUN_VERSION } from "./variables.ts";

describe("sortManifest", () => {
	test("orders top-level keys as the formatter does", () => {
		const sorted = sortManifest({
			scripts: {},
			zeta: 1,
			name: "x",
			alpha: 2,
			version: "1.0.0"
		});

		expect(Object.keys(sorted)).toEqual([
			"name",
			"version",
			"scripts",
			"alpha",
			"zeta"
		]);
	});

	test("sorts script and dependency names", () => {
		const sorted = sortManifest({
			devDependencies: { zed: "1", "@types/bun": "1", abc: "1" }
		});

		expect(JSON.stringify(sorted)).toBe(
			'{"devDependencies":{"@types/bun":"1","abc":"1","zed":"1"}}'
		);
	});
});

const textOf = (path: string): Promise<string> =>
	Bun.file(new URL(path, import.meta.url)).text();

const catalogOf = (text: string): unknown =>
	asRecord(asRecord(JSON.parse(text))["workspaces"])["catalog"];

describe("templates against the repository", () => {
	test("the pinned Bun equals .bun-version", async () => {
		const version = await textOf("../../../../.bun-version");

		expect(BUN_VERSION).toBe(version.trim());
	});

	test("the template catalog equals the root catalog", async () => {
		const root = await textOf("../../../../package.json");
		const template = await textOf(
			"../../../../templates/typescript/manifest/package.json.tmpl"
		);

		expect(catalogOf(template)).toEqual(catalogOf(root));
	});
});
