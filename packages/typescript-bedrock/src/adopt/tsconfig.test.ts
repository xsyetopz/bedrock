import { describe, expect, test } from "bun:test";

import { migrateTsconfig } from "./tsconfig.ts";

const base = {
	compilerOptions: {
		module: "Preserve",
		strict: true,
		target: "ESNext",
		types: ["bun"],
	},
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
			extends: [
				"@docusaurus/tsconfig",
				"typescript-bedrock/tsconfig.base.json",
			],
			compilerOptions: {
				paths: { "@site/*": ["./*"] },
				jsx: "react-jsx",
				types: ["node", "bun"],
			},
			include: ["src"],
			exclude: ["fixtures", "**/dist", "**/.build", "**/coverage", "**/target"],
		});
		expect(notes).toHaveLength(5);
	});

	test("maps a lone baseUrl to a catch-all path", () => {
		const { tsconfig } = migrateTsconfig(
			{ compilerOptions: { baseUrl: "src" } },
			base,
		);
		expect(tsconfig["extends"]).toBe("typescript-bedrock/tsconfig.base.json");
		expect(tsconfig["compilerOptions"]).toEqual({
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

	test("is idempotent: the template's base files are replaced, not stacked", () => {
		const first = migrateTsconfig({ compilerOptions: {} }, base);
		const second = migrateTsconfig(first.tsconfig, base);
		expect(second.tsconfig["extends"]).toBe(
			"typescript-bedrock/tsconfig.base.json",
		);
		expect(second.notes).toEqual([]);
	});
});
