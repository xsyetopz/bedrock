# Contributing

## Setup

You need `just`, Bun 1.4.4, rustup, and Swift 6.4 (Xcode on macOS 14 or newer runs the Swift tests).

```bash
bun install
just setup
```

`just setup` installs the pinned tools, including cargo-deny and kache, and points git at `.githooks`.

## Before You Open a Pull Request

```bash
just verify
```

This runs each language's format check, type check, lints, guard, and tests. `just` lists the narrower recipes, such as `just rust::test`.

## Rules

- Use `bun` for packages and scripts, and `bunx` for one-off tools.
- Fix the cause of each lint or type error. The guards reject suppression comments.
- Pin dependency versions in the `catalog` of `package.json`.
- Change a shared file in `base/`; each bedrock's embedded copy must match it.
- Add or update a test for each behavior change. Bun coverage must stay at or above 90% for lines and functions.
- Update the docs in `docs/` and the READMEs when a config or CLI behavior changes.

## Commits

Use Conventional Commits. The `commit-msg` hook checks the subject, the blank line before a body, and `BREAKING CHANGE:` footers.

```text
feat(cli): add --dry-run
docs: explain legacy migration
```

Allowed types: `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`, `revert`, `style`, `test`.

## Releases

Bump a bedrock's version in `packages/typescript-bedrock/package.json`, `packages/rust-bedrock/Cargo.toml`, or `packages/swift-bedrock/Sources/SwiftBedrock/Files.swift`. Once every check passes on `main`, the Release workflow publishes a new `typescript-bedrock` version to npm, then tags that commit `<bedrock>-v<version>` and creates its GitHub release. Adopted TypeScript repos install from npm; Rust and Swift repos pin the tags. `gh workflow run Release` releases the head of `main` by hand, once its checks have passed. npm trusts the Release workflow to publish `typescript-bedrock` through trusted publishing, so the repository holds no npm token.

## Agents

AI agents follow `AGENTS.md`.

## License

Contributions are licensed under the MIT license in `LICENSE`.
