# Bedrock

Strict, shared setups for Bun, Rust, and Swift (with optional C++20) repositories. Each language has a bedrock: a tool that starts a repo or moves an existing or legacy one onto its tooling, fully, so no old tool is left running beside it. People and AI agents run them the same way.

| Bedrock | For | Tooling |
| --- | --- | --- |
| [`bun-bedrock`](packages/bun-bedrock) | TypeScript and JavaScript | Bun 1.4.4, TypeScript 7 through `bun check`, `bun fmt`, oxlint with type-aware rules, `bun test` |
| [`rust-bedrock`](packages/rust-bedrock) | Rust | Rust 1.99.1, edition 2024, workspace lints, Clippy, rustdoc lints, cargo-deny, kache, inline unit tests |
| [`swift-bedrock`](packages/swift-bedrock) | Swift, and C++20 with `--cxx` | Swift 6.4 for macOS 12 and newer, swift-format, SwiftLint, Swift Testing, an Xcode config; clang-format and clang-tidy for C++ |

## What Every Repo Gets

Every bedrock writes the same base, so a repo with several languages has one of each:

- a `justfile` that runs each task for every language, through `.just/<language>.just`, with the project's own recipes in `project.just`;
- `.githooks`: `pre-commit` runs `just pre-commit`, and `commit-msg` checks Conventional Commits;
- `AGENTS.md`, linked as `CLAUDE.md` and `GEMINI.md`, with a block of rules from each bedrock;
- `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml`, with a block from each bedrock;
- markdownlint, and a CI workflow per language.

A bedrock rewrites only the blocks it owns, between `# >>> name` and `# <<< name` comments (`<!-- name -->` and `<!-- /name -->` in Markdown), keeps the rest of each file, and lists the blocks in the same order whichever bedrock runs first. Each replaced file is kept as `<file>.bak`.

```bash
just            # list the recipes
just setup      # install the pinned tools and turn on the hooks
just verify     # every check CI runs
just rust::test # one language's recipe
```

## Adopt a Repo

From a Claude Code session with the plugin installed:

```text
/adopt                  # detect the languages, print the plan
/adopt ts rust --yes    # adopt these languages and apply
/adopt cpp ../app --yes # Swift with C++20, in another folder
```

Language names are case-insensitive: `typescript`, `ts`, `js`, or `bun`; `rust` or `rs`; `swift`; `cpp`, `c++`, `c`, or `objc` for Swift with C++. Without `--yes`, `/adopt` prints the plan and changes nothing. It checks that `git`, `just`, and each language's toolchain are installed first, and names any that are missing.

By hand, from a checkout of this repo:

```bash
bun packages/bun-bedrock/src/cli.ts adopt <dir> --package file:$PWD/packages/bun-bedrock --dry-run
cargo run --manifest-path packages/rust-bedrock/Cargo.toml -- adopt <dir> --dry-run
swift run swift-bedrock adopt <dir> [--cxx] --dry-run
```

Drop `--dry-run` to apply, or add `--yes` to apply without asking. Then run `just setup` and `just verify` in the target, fix what they report, and delete each `.bak` file once you've compared it. [docs/adopting.md](docs/adopting.md) lists what changes and the order to fix errors in.

## Start a Repo

```bash
cargo run --manifest-path packages/rust-bedrock/Cargo.toml -- new <dir> --kind lib   # or bin, no-std, ffi, wasm
swift run swift-bedrock new <dir> --kind library                                   # or executable, app; add --cxx
bun packages/bun-bedrock/src/cli.ts adopt <empty dir> --package file:$PWD/packages/bun-bedrock
```

## Guards

Each bedrock has a `guard` that `just verify` and the pre-commit hook run. It catches what the linters can't:

- suppression comments and file-wide disables, where each tool has a narrower, reasoned form;
- files from replaced tools, such as another package manager's lockfile, a Podfile, or `.swiftformat`;
- Rust unit tests in a separate `tests.rs`, which `just lint-fix` moves inline;
- XCTest outside UI tests, which `just lint-fix` converts to Swift Testing;
- public API without docs;
- managed files that drifted from what adopt writes.

## Claude Code Plugin

[`plugins/bedrock`](plugins/bedrock) adds `/adopt`, and in a repo with `bun.lock` it blocks Bash commands that run npm, npx, pnpm, pnpx, or yarn. This repo is a plugin marketplace:

```text
/plugin install bedrock --marketplace xsyetopz/bedrock
```

Installed from the marketplace, the plugin clones this repo on its first `/adopt`. To try local changes, run `claude --plugin-dir plugins/bedrock`; `bun run plugin:validate` checks the plugin against Claude Code's plugin API.

## Repo Layout

| Path | Content |
| --- | --- |
| `base/` | The files every bedrock writes. Each bedrock embeds a copy, which the tests keep in sync. |
| `packages/bun-bedrock` | The Bun bedrock, a workspace package |
| `packages/rust-bedrock` | The Rust bedrock, a Cargo package with its starter files |
| `packages/swift-bedrock` | The Swift and C++ bedrock, built by the root `Package.swift` |
| `plugins/bedrock` | The Claude Code plugin |

The repo adopts its own bedrocks, so `just verify` checks all of it.

## Docs

| File | Content |
| --- | --- |
| [docs/adopting.md](docs/adopting.md) | What adopt changes, and how to fix the errors that follow |
| [docs/tsconfig.md](docs/tsconfig.md) | Every TypeScript compiler option and why it's set |
| [docs/lint-rules.md](docs/lint-rules.md) | How oxlint and markdownlint are set up |
| [docs/node-apis.md](docs/node-apis.md) | When to use a Bun API and when a `node:` module |
| [AGENTS.md](AGENTS.md) | Instructions for AI agents |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | How to report a vulnerability |

## License

MIT. See [LICENSE](LICENSE).

`base/.githooks/commit-msg` is ported from [compilerla/conventional-pre-commit](https://github.com/compilerla/conventional-pre-commit) v4.5.0, licensed under [Apache-2.0](https://github.com/compilerla/conventional-pre-commit/blob/v4.5.0/LICENSE). The file header lists the changes.
