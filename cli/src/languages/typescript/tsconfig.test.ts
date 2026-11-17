import { describe, expect, test } from "bun:test";

import { migrateTsconfig } from "./tsconfig.ts";

const base = {
	compilerOptions: {
		module: "Preserve",
		strict: true,
		target: "ESNext",
		types: ["bun"],
	},
	exclude: ["**/.build", "**/build", "**/coverage", "**/target"],
};

describe("migrateTsconfig", () => {
	test("drops removed and policy options, keeps project options", () => {
		const { tsconfig, notes } = migrateTsconfig(
			{
				extends: "@docusaurus/tsconfig",
				compilerOptions: {
					baseUrl: ".",
					paths: { "@site/*": ["./*"] },
					esModuleInterop: true,
					module: "commonjs",
					strict: false,
					jsx: "react-jsx",
					types: ["node"],
				},
				include: ["src"],
				exclude: ["fixtures", "**/dist"],
			},
			base,
		);
		expect(tsconfig).toEqual({
			$schema: "https://json.schemastore.org/tsconfig.json",
			extends: ["@docusaurus/tsconfig"],
			compilerOptions: {
				module: "Preserve",
				strict: true,
				target: "ESNext",
				paths: { "@site/*": ["./*"] },
				jsx: "react-jsx",
				types: ["node", "bun"],
			},
			include: ["src"],
			exclude: [
				"fixtures",
				"**/dist",
				"**/.build",
				"**/build",
				"**/coverage",
				"**/target",
			],
		});
		expect(notes).toHaveLength(5);
	});

	test("maps a lone baseUrl to a catch-all path", () => {
		const { tsconfig } = migrateTsconfig(
			{ compilerOptions: { baseUrl: "src" } },
			base,
		);
		expect(tsconfig["extends"]).toBeUndefined();
		expect(tsconfig["compilerOptions"]).toEqual({
			...base.compilerOptions,
			paths: { "*": ["./src/*"] },
		});
	});

	test("turns emit back on for projects that build with tsc", () => {
		const { tsconfig } = migrateTsconfig(
			{ compilerOptions: { outDir: "dist", declaration: true } },
			base,
		);
		expect(tsconfig["compilerOptions"]).toMatchObject({
			noEmit: false,
			allowImportingTsExtensions: false,
			outDir: "dist",
		});
	});

	test("drops the extends of the old typescript-bedrock base file", () => {
		const { tsconfig, notes } = migrateTsconfig(
			{ extends: "typescript-bedrock/tsconfig.base.json" },
			base,
		);
		expect(tsconfig["extends"]).toBeUndefined();
		expect(notes).toEqual([]);
	});

	test("is idempotent", () => {
		const first = migrateTsconfig({ compilerOptions: {} }, base);
		const second = migrateTsconfig(first.tsconfig, base);
		expect(second.tsconfig).toEqual(first.tsconfig);
		expect(second.notes).toEqual([]);
	});
});
