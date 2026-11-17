# Contributing

## Setup

You need `just` and Bun 1.4.4. The end-to-end tests for each language run its real toolchain, and skip when it is missing: rustup for Rust, Swift 6.4 for Swift and C++ (Xcode on macOS 14 or newer), a JDK 17 or newer for the JVM, whose Gradle downloads GraalVM Community 25, and Go 1.27 with golangci-lint 2.15 for Go.

```bash
bun install
just setup
```

`just setup` installs the pinned tools and points git at `.githooks`. This repo pins `"source"` in `.bedrock.json`, so it runs the CLI from its own tree and downloads no release.

## Before You Open a Pull Request

```bash
just verify
```

This runs the format check, type check, lints, guard, and tests. Run the narrowest check while you work: `bun test <file>`, `bun check`, or `just typescript::lint <path>`. `just` lists the recipes.

## Rules

- Use `bun` for packages and scripts, and `bunx` for one-off tools.
- Fix the cause of each lint or type error. The guards reject suppression comments.
- Pin dependency versions in the `catalog` of `package.json`.
- Change a file every repo gets in `templates/base`, and one a language adds in `templates/<language>`. This repo's own configs are copies that `bedrock update` rewrites.
- Write each managed file through a template, and keep the plan idempotent: a second `bedrock adopt` on the same repo plans no steps.
- Add or update a test for each behavior change, and give each language kind an end-to-end test that inits it and runs its toolchain. `bun test` prints coverage but sets no minimum, because Bun fails the run when any one file falls under the threshold.
- Update `README.md` and `docs/` when a config or CLI behavior changes.

## Commits

Use Conventional Commits. The `commit-msg` hook checks the subject, the blank line before a body, and `BREAKING CHANGE:` footers.

```text
feat(cli): add --dry-run
docs: explain legacy migration
```

Allowed types: `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`, `revert`, `style`, `test`.

## Releases

Bump `version` in `package.json` and in `plugins/bedrock/.claude-plugin/plugin.json`; a test fails when they differ. Once every check passes on `main`, the Release workflow compiles the CLI with `bun cli/build.ts`, one binary per platform with the templates embedded, and creates the `v<version>` GitHub release with those binaries and their `SHA256SUMS`. Adopted repos pin that version in `.bedrock.json`, and `just setup` downloads it. `gh workflow run Release` releases the head of `main` by hand, once its checks have passed.

## Agents

AI agents follow `AGENTS.md`.

## License

Contributions are licensed under the MIT license in `LICENSE`.
