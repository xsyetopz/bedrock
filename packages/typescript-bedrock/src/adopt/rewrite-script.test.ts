import { describe, expect, test } from "bun:test";

import { rewriteScript } from "./rewrite-script.ts";

describe("rewriteScript", () => {
	test.each([
		["npm run build", "bun run build"],
		["npm test", "bun run test"],
		["npm ci", "bun install --frozen-lockfile"],
		["yarn install --immutable", "bun install --frozen-lockfile"],
		["pnpm install", "bun install"],
		["pnpm build", "bun run build"],
		["yarn add -D zod", "bun add -D zod"],
		["npm publish --access public", "bun publish --access public"],
		["npx --yes cowsay hi", "bunx cowsay hi"],
		["pnpm dlx prettier --check .", "bun fmt --check"],
		["prettier --write .", "bun fmt"],
		["prettier --write src/**/*.ts", "bun fmt src/**/*.ts"],
		["oxfmt --check", "bun fmt --check"],
		["eslint . --ext .ts", "typescript-bedrock lint"],
		["eslint --fix .", "typescript-bedrock lint --fix"],
		["biome check --write .", "typescript-bedrock lint --fix"],
		["biome format .", "bun fmt --check"],
		["biome format --write .", "bun fmt"],
		["tsc --noEmit", "bun check"],
		["tsc --noEmit -p tsconfig.app.json", "bun check -p tsconfig.app.json"],
		["tsc -p tsconfig.build.json", "tsc -p tsconfig.build.json"],
		["tsx watch src/main.ts", "bun --watch src/main.ts"],
		["ts-node src/main.ts", "bun src/main.ts"],
		["node dist/index.js", "bun dist/index.js"],
		["jest --coverage", "bun test --coverage"],
		["vitest run", "bun test"],
		["vitest --watch", "bun test --watch"],
		[
			"cross-env NODE_ENV=production tsx src/main.ts",
			"NODE_ENV=production bun src/main.ts",
		],
		["rimraf dist", "rm -rf dist"],
		["npm whoami", "npm whoami"],
		["vite build", "vite build"],
	])("%p → %p", (input, output) => {
		expect(rewriteScript(input)).toBe(output);
	});

	test("rewrites each command of a chain and keeps the separators", () => {
		expect(
			rewriteScript("rimraf dist && tsc --noEmit || echo failed; npm run x"),
		).toBe("rm -rf dist && bun check || echo failed; bun run x");
	});

	test("leaves an environment-only command as it is", () => {
		expect(rewriteScript("FOO=1")).toBe("FOO=1");
	});
});
