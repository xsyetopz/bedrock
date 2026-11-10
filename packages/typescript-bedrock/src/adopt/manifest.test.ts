import { describe, expect, test } from "bun:test";

import { asRecord } from "./json.ts";
import { migrateManifest } from "./manifest.ts";

const template = {
	scripts: { lint: "oxlint", test: "bun test", typecheck: "bun check" },
	devDependencies: { oxlint: "catalog:" },
	workspaces: { packages: [], catalog: { oxlint: "1.90.0" } },
	overrides: { "@types/node": "26.6.6" },
	packageManager: "bun@1.4.4",
	engines: { bun: ">=1.4.4" },
};

describe("migrateManifest", () => {
	test("keeps a package the template ships, even one the removals match", () => {
		const { manifest, notes } = migrateManifest(
			{ devDependencies: { "eslint-plugin-jsdoc": "catalog:" } },
			{ ...template, devDependencies: { "eslint-plugin-jsdoc": "catalog:" } },
		);
		expect(manifest["devDependencies"]).toEqual({
			"eslint-plugin-jsdoc": "catalog:",
		});
		expect(notes.join("\n")).not.toContain("Removed packages");
	});

	test("replaces tools, scripts, engines, and package manager", () => {
		const { manifest, notes } = migrateManifest(
			{
				name: "app",
				scripts: {
					check: "tsc --noEmit",
					fmt: "prettier --write .",
					lint: "eslint .",
					test: "jest",
					typecheck: "bun check --cwd packages/app",
				},
				dependencies: { zod: "^3.0.0" },
				devDependencies: {
					eslint: "^9.0.0",
					"@typescript-eslint/parser": "^8.0.0",
					husky: "^9.0.0",
					vite: "^6.0.0",
				},
				engines: { node: ">=20", npm: ">=10" },
				packageManager: "npm@10.0.0",
				"lint-staged": { "*.ts": "eslint" },
			},
			template,
		);
		expect(manifest).toEqual({
			name: "app",
			scripts: {
				"check:project": "bun check",
				"fmt:project": "bun fmt",
				lint: "oxlint",
				test: "bun test",
				typecheck: "bun check",
			},
			dependencies: { zod: "^3.0.0" },
			devDependencies: { vite: "^6.0.0", oxlint: "catalog:" },
			workspaces: { packages: [], catalog: { oxlint: "1.90.0" } },
			engines: { node: ">=20", bun: ">=1.4.4" },
			overrides: { "@types/node": "26.6.6" },
			packageManager: "bun@1.4.4",
		});
		expect(notes.join("\n")).toContain("@typescript-eslint/parser, husky");
		expect(notes.join("\n")).toContain("Jest or Vitest");
		expect(notes.join("\n")).toContain('"lint-staged"');
		expect(notes.join("\n")).toContain(
			'Replaced script "typecheck" ("bun check --cwd packages/app")',
		);
	});

	test("merges pnpm-workspace.yaml into workspaces", () => {
		const { manifest } = migrateManifest(
			{ name: "mono", workspaces: ["apps/*"] },
			template,
			{
				packages: ["apps/*", "packages/*"],
				catalog: { react: "19.0.0" },
				catalogs: { legacy: { react: "18.3.1" } },
				onlyBuiltDependencies: ["esbuild"],
			},
		);
		expect(manifest["workspaces"]).toEqual({
			packages: ["apps/*", "packages/*"],
			catalog: { react: "19.0.0", oxlint: "1.90.0" },
			catalogs: { legacy: { react: "18.3.1" } },
		});
		expect(
			Object.keys(asRecord(asRecord(manifest["workspaces"])["catalog"])),
		).toEqual(["oxlint", "react"]);
		expect(manifest["trustedDependencies"]).toEqual(["esbuild"]);
	});

	test("drops trusted entries for the packages it removed", () => {
		const { manifest } = migrateManifest(
			{
				name: "app",
				devDependencies: { "@biomejs/biome": "2.0.0" },
				trustedDependencies: ["@biomejs/biome", "esbuild"],
			},
			template,
		);
		expect(manifest["trustedDependencies"]).toEqual(["esbuild"]);
		const { manifest: only } = migrateManifest(
			{ name: "app", trustedDependencies: ["@biomejs/biome"] },
			template,
		);
		expect("trustedDependencies" in only).toBe(false);
	});

	test("merges overrides: the template wins a conflict, the target keeps the rest", () => {
		const { manifest } = migrateManifest(
			{
				name: "app",
				overrides: { "@types/node": "20.0.0", lodash: "4.17.21" },
			},
			template,
		);
		expect(manifest["overrides"]).toEqual({
			"@types/node": "26.6.6",
			lodash: "4.17.21",
		});
	});
});
