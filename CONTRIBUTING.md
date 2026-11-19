# Contributing

## Setup

You need `just`, Go 1.27 with golangci-lint 2.15 for the CLI, and Bun 1.4.5 for the plugin and the shared Markdown and TypeScript checks. The end-to-end tests for each language run its real toolchain, and skip when it is missing: rustup for Rust, Swift 6.4 for Swift and C++ (Xcode on macOS 14 or newer), a JDK 17 or newer for the JVM, whose Gradle downloads GraalVM Community 25, Bun for TypeScript, the Haxe toolchain for Haxe, and the .NET 10 SDK for .NET.

```bash
bun install
just setup
```

`just setup` installs the pinned tools and the hooks in `lefthook.yml`. This repo pins `bedrock_version = source` in `.editorconfig`, so it runs the CLI from its own tree and downloads no release.

## Before You Open a Pull Request

```bash
just verify
```

This runs the format check, type check, lints, guard, and tests. Run the narrowest check while you work: `go test ./internal/<package>/`, `go test -short ./...` to skip the toolchain runs, `just go::lint`, or for the plugin `bun test <file>`. `just` lists the recipes.

## Rules

- Use `bun` for the plugin's packages and scripts, and `bunx` for one-off tools.
- Fix the cause of each lint or type error. The guards reject suppression comments; Go silences a finding only with `//nolint:<linter> // <reason>`.
- Pin JavaScript dependency versions in the `catalog` of `package.json`, and keep `go.mod` tidy.
- Change a file every repo gets in `templates/base`, and one a language adds in `templates/<language>`. This repo's own configs are copies that `bedrock update` rewrites.
- Write each managed file through a template, and keep the plan idempotent: a second `bedrock adopt` on the same repo plans no steps.
- Add or update a test for each behavior change, and give each language kind an end-to-end test that inits it and runs its toolchain. `bun test` prints coverage but sets no minimum, because Bun fails the run when any one file falls under the threshold.
- Update `README.md` and `docs/` when a config or CLI behavior changes.

## Adding a Language

A language is a brick: a package under `internal/languages/<language>` and the files under `templates/<language>`. Nothing in the core changes.

1. Write `templates/<language>`: `files/` copied as they are, `blocks/` merged into the shared files, such as the `.gitignore` block, and the `.just/<language>.just` recipes, with `format`, `lint`, `test`, `guard`, `verify`, and `pre-push`.
2. Give the package a `New()` that returns a `language.Language`: a `Spec` with the ID, block name, markers, kinds, and tools. Add only the capabilities in `internal/language` it needs, such as a `Guarder` for what its linters cannot see.
3. Register `New()` at the end of `internal/catalog`, so existing repos keep their block order.
4. Test the plan and guard through the `internal/bedrocktest` helpers, with a second adopt that plans nothing, and add an `e2e_test.go` that inits each kind and runs the real toolchain, skipped under `-short` and when it is missing. Add the language to the matrix in `.github/workflows/e2e.yml`.

## Commits

Use Conventional Commits. The `commit-msg` hook checks the subject, the blank line before a body, and `BREAKING CHANGE:` footers.

```text
feat(cli): add --dry-run
docs: explain legacy migration
```

Allowed types: `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`, `revert`, `style`, `test`.

## Releases

Bump `version` in `package.json` and in `plugins/bedrock/.claude-plugin/plugin.json`; a test fails when they differ. Once every check passes on `main`, the Release workflow runs `just dist`, which cross-compiles the CLI into one binary per platform with the templates embedded, and creates the `v<version>` GitHub release with those binaries and their `SHA256SUMS`. Adopted repos pin that version as `bedrock_version` in `.editorconfig`, and `just setup` downloads it. `gh workflow run Release` releases the head of `main` by hand, once its checks have passed.

## Agents

AI agents follow `AGENTS.md`.

## License

Contributions are licensed under the MIT license in `LICENSE`.
