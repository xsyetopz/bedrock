import type { Language } from "../../core/language.ts";
import { migrateEditions } from "./editions.ts";
import { guardRust } from "./guard.ts";
import { crateVariables, KINDS, packageName } from "./kinds.ts";
import { planRust } from "./plan.ts";

/** Rust projects: Cargo workspaces on strict Clippy and rustdoc lints. */
export const rust: Language = {
	id: "rust",
	block: "rust-bedrock",
	aliases: ["rust"],
	markers: ["Cargo.toml"],
	tools: {
		cargo: "rustup, from https://rustup.rs, such as brew install rustup",
	},
	sources: [
		"**/*.rs",
		"**/Cargo.toml",
		"Cargo.lock",
		"rust-toolchain.toml",
		"rustfmt.toml",
		"clippy.toml",
		"deny.toml",
		".cargo/**",
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
	guard: guardRust,
};
