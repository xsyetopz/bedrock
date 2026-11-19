import { describe, expect, test } from "bun:test";

import { asRecord } from "./json.ts";
import { migrateManifest } from "./manifest.ts";

const template = {
	scripts: { lint: "oxlint", test: "bun test", typecheck: "bun check" },
	devDependencies: { oxlint: "catalog:" },
	workspaces: { packages: [], catalog: { oxlint: "1.90.0" } },
	overrides: { "@types/node": "26.6.6" },
	packageManager: "bun@1.4.5",
	engines: { bun: ">=1.4.5" }
};

describe("migrateManifest", () => {
	test("keeps a package the template ships, even one the removals match", () => {
		const { manifest } = migrateManifest(
			{ devDependencies: { "eslint-plugin-jsdoc": "catalog:" } },
			{ ...template, devDependencies: { "eslint-plugin-jsdoc": "catalog:" } }
		);
		expect(manifest["devDependencies"]).toEqual({
			"eslint-plugin-jsdoc": "catalog:"
		});
	});

	test("replaces tools, scripts, engines, and package manager", () => {
		const { manifest } = migrateManifest(
			{
				name: "app",
				scripts: {
					check: "tsc --noEmit",
					fmt: "prettier --write .",
					lint: "eslint .",
					test: "jest",
					typecheck: "bun check --cwd packages/app"
				},
				dependencies: { zod: "^3.0.0" },
				devDependencies: {
					eslint: "^9.0.0",
					"@typescript-eslint/parser": "^8.0.0",
					husky: "^9.0.0",
					vite: "^6.0.0"
				},
				engines: { node: ">=20", npm: ">=10" },
				packageManager: "npm@10.0.0",
				"lint-staged": { "*.ts": "eslint" }
			},
			template
		);
		expect(manifest).toEqual({
			name: "app",
			scripts: {
				"check:project": "bun check",
				"fmt:project": "bun fmt",
				lint: "oxlint",
				test: "bun test",
				typecheck: "bun check"
			},
			dependencies: { zod: "^3.0.0" },
			devDependencies: { vite: "^6.0.0", oxlint: "catalog:" },
			workspaces: { packages: [], catalog: { oxlint: "1.90.0" } },
			engines: { node: ">=20", bun: ">=1.4.5" },
			overrides: { "@types/node": "26.6.6" },
			packageManager: "bun@1.4.5"
		});
	});

	test("merges pnpm-workspace.yaml into workspaces", () => {
		const { manifest } = migrateManifest(
			{ name: "mono", workspaces: ["apps/*"] },
			template,
			{
				packages: ["apps/*", "packages/*"],
				catalog: { react: "19.0.0" },
				catalogs: { legacy: { react: "18.3.1" } },
				onlyBuiltDependencies: ["esbuild"]
			}
		);
		expect(manifest["workspaces"]).toEqual({
			packages: ["apps/*", "packages/*"],
			catalog: { react: "19.0.0", oxlint: "1.90.0" },
			catalogs: { legacy: { react: "18.3.1" } }
		});
		expect(
			Object.keys(asRecord(asRecord(manifest["workspaces"])["catalog"]))
		).toEqual(["oxlint", "react"]);
		expect(manifest["trustedDependencies"]).toEqual(["esbuild"]);
	});

	test("drops trusted entries for the packages it removed", () => {
		// Arrange
		const trusted = {
			name: "app",
			devDependencies: { "@biomejs/biome": "2.0.0" },
			trustedDependencies: ["@biomejs/biome", "esbuild"]
		};
		const only = { name: "app", trustedDependencies: ["@biomejs/biome"] };

		// Act
		const { manifest } = migrateManifest(trusted, template);
		const { manifest: emptied } = migrateManifest(only, template);

		// Assert
		expect(manifest["trustedDependencies"]).toEqual(["esbuild"]);
		expect("trustedDependencies" in emptied).toBe(false);
	});

	test("drops the typescript-bedrock dependency and catalog pin", () => {
		const { manifest } = migrateManifest(
			{
				devDependencies: { "typescript-bedrock": "catalog:", vite: "^6.0.0" },
				workspaces: {
					catalog: { "typescript-bedrock": "1.0.0", react: "19.0.0" }
				}
			},
			template
		);
		expect(manifest["devDependencies"]).toEqual({
			vite: "^6.0.0",
			oxlint: "catalog:"
		});
		expect(
			Object.keys(asRecord(asRecord(manifest["workspaces"])["catalog"]))
		).toEqual(["oxlint", "react"]);
	});

	test("drops a machine-local file: path to typescript-bedrock", () => {
		// Arrange
		const stale =
			"file:/home/dev/.claude/plugins/cache/xsyetopz/bedrock/0.1.4/.bedrock/packages/typescript-bedrock";

		// Act
		const { manifest } = migrateManifest(
			{ name: "app", devDependencies: { "typescript-bedrock": stale } },
			template
		);

		// Assert
		expect(JSON.stringify(manifest)).not.toContain("typescript-bedrock");
	});

	test("merges overrides: the template wins a conflict, the target keeps the rest", () => {
		const { manifest } = migrateManifest(
			{
				name: "app",
				overrides: { "@types/node": "20.0.0", lodash: "4.17.21" }
			},
			template
		);
		expect(manifest["overrides"]).toEqual({
			"@types/node": "26.6.6",
			lodash: "4.17.21"
		});
	});
});
