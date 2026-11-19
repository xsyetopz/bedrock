import type { Language } from "../../core/language.ts";
import { editionPending, migrateEditions } from "./editions.ts";
import { guardRust } from "./guard.ts";
import { crateVariables, KINDS, packageName } from "./kinds.ts";
import { planRust } from "./plan.ts";
import { rustTestFiles } from "./test-files.ts";

/** Rust projects: Cargo workspaces on strict Clippy and rustdoc lints. */
export const rust: Language = {
	id: "rust",
	block: "rust-bedrock",
	aliases: ["rust"],
	markers: ["Cargo.toml"],
	tools: {
		cargo: "rustup, from https://rustup.rs, such as brew install rustup"
	},
	sources: [
		"**/*.rs",
		"**/Cargo.toml",
		"Cargo.lock",
		"rust-toolchain.toml",
		"rustfmt.toml",
		"clippy.toml",
		"deny.toml",
		".cargo/**"
	],
	kinds: KINDS,
	variables: (context) =>
		Promise.resolve(crateVariables(packageName(context.root))),
	plan: planRust,
	afterApply: async (context) => {
		if (Bun.which("cargo") !== null) {
			await migrateEditions(context.root);
			await Bun.$`cargo fetch`.cwd(context.root).nothrow();
		}
	},
	runs: "cargo fix --edition for each edition older than 2024, then cargo fetch",
	runPending: async (context) =>
		Bun.which("cargo") !== null && (await editionPending(context.root)),
	guard: guardRust,
	// Clippy has function lints (too_many_lines, too_many_arguments, cognitive_complexity) but no file-length lint.
	lizard: ["rust"],
	testFiles: rustTestFiles
};
