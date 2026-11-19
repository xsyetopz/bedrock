/** A repository shaped like one that rust-bedrock adopted: an xtask crate, the alias, and the git allowance. */
export const OLD_BEDROCK: Readonly<Record<string, string>> = {
	"Cargo.toml": `[workspace]
resolver = "3"
members = ["crates/*", "xtask"]

[workspace.package]
edition = "2024"

[workspace.lints.rust]
unused = "warn"
`,
	"xtask/Cargo.toml": `[package]
name = "xtask"
version = "0.0.0"
edition = "2024"
publish = false

[dependencies]
rust-bedrock = { git = "https://github.com/xsyetopz/bedrock", tag = "rust-bedrock-v0.9.0" }

[lints]
workspace = true
`,
	"xtask/src/main.rs": "fn main() {}\n",
	".cargo/config.toml": `[alias]
bedrock = "run --quiet --package xtask --"

[build]
jobs = 4
`,
	"deny.toml": `[sources]
unknown-git = "deny"
allow-git = ["https://github.com/xsyetopz/bedrock"]
`,
	"crates/a/Cargo.toml": `[package]
name = "a"
version = "0.1.0"
edition.workspace = true

[lints.rust]
unsafe_code = "forbid"
`,
	"crates/a/src/lib.rs": "//! A.\n\npub mod foo;\n",
	"crates/a/src/foo/mod.rs": "//! Foo.\n",
	"crates/a/tests/it.rs": "#[test]\nfn it() {}\n"
};
