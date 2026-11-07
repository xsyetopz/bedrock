# Rust Bedrock

Shared tooling for strict Rust repos on Rust 1.99.1 and edition 2024. It starts a workspace, moves an existing one onto the setup, and guards what Clippy can't check. It isn't on crates.io yet.

## Commands

| Command | Does |
| --- | --- |
| `rust-bedrock new <dir> [--kind lib\|bin\|no-std\|ffi\|wasm]... [--name <name>]` | Start a workspace with one member per kind. In a repo that also uses Bun, an `ffi` crate gets a Bun `bun:ffi` binding and its test. |
| `rust-bedrock adopt [<dir>] [--dry-run] [--yes] [--no-run]` | Set up an existing workspace or package. `--no-run` skips the Cargo steps after the files are written. |
| `rust-bedrock guard [--fix]` | Fail on test modules in other files, members that skip the workspace lints, and files that differ from what `adopt` writes. `--fix` moves each test module inline. |

`new` and `adopt` take `--bedrock-path <path>` to point `xtask` at a local checkout instead of the tagged release. In an adopted repo, `cargo bedrock` runs the guard through `xtask`, and `just` runs it for you.

## What It Sets Up

- `[workspace.lints]` with the rustc, Clippy, and rustdoc lints, which every member inherits. Public items need docs, with `# Panics` and `# Errors` sections where they apply, and a lint is silenced only with `#[expect(lint, reason = "…")]`.
- `rustfmt.toml`, `clippy.toml`, `deny.toml` for cargo-deny, and `rust-toolchain.toml` with Clippy and rustfmt.
- [kache](https://github.com/kunobi-ninja/kache) as the build cache: `just setup` installs it and runs `kache init`, and CI uses `kache-action`.
- `.just/rust.just`, `.github/workflows/rust.yml`, and its blocks in the files every bedrock shares (`files/base`, a copy of `base/`).

[docs/adopting.md](../../docs/adopting.md#rust) lists what `adopt` changes in an existing repo.

## Inline Tests

Unit tests stay in the file they test, as `#[cfg(test)] mod tests { … }` at its end. The guard reports each `#[cfg(test)] mod name;` whose body sits in another file, such as `foo/tests.rs`, and `guard --fix` moves the body inline. A lint can't do this: rustc and Clippy see the module after it's loaded, not which file it came from.
