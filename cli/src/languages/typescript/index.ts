import type { Language } from "../../core/language.ts";
import { guardTypescript } from "./guard/index.ts";
import { installDependencies } from "./install.ts";
import { planTypescript } from "./plan.ts";
import { variables } from "./variables.ts";

/** TypeScript on Bun: oxlint, `bun fmt`, `bun check`, `bun test`, and markdownlint. */
export const typescript: Language = {
	id: "typescript",
	block: "typescript-bedrock",
	aliases: ["typescript", "ts", "bun"],
	markers: [
		"bun.lock",
		"bun.lockb",
		"bunfig.toml",
		"package.json",
		"tsconfig.json",
	],
	tools: {
		bun: "curl -fsSL https://bun.sh/install | bash, or brew install oven-sh/bun/bun",
	},
	sources: [
		"**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts}",
		"**/package.json",
		"bunfig.toml",
		"**/tsconfig.json",
	],
	kinds: ["library", "cli", "plugin"],
	variables,
	plan: planTypescript,
	afterApply: installDependencies,
	runs: "bun install, then bun fmt",
	guard: (context) => guardTypescript(context),
};
