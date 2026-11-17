# Bedrock

Strict, shared setups for TypeScript (on Bun), Rust, Swift, C++20, and Java and Kotlin repositories. Bedrock is one CLI, `bedrock`, and a Claude Code plugin that runs it. It starts a repo, or moves an existing or legacy one onto its tooling, fully, so no old tool is left running beside it. People and AI agents run it the same way, and the repo it sets up depends on no bedrock package.

| Language | Kinds | Tooling |
| --- | --- | --- |
| `typescript` | `library`, `cli`, `plugin` | Bun 1.4.4, TypeScript 7 through `bun check`, `bun fmt`, oxlint with type-aware rules, `bun test` |
| `rust` | `lib`, `bin`, `no-std`, `ffi`, `wasm`, `bun-ffi` | Rust 1.99.1, edition 2024, workspace lints, Clippy, rustdoc lints, cargo-deny, kache, inline unit tests |
| `swift` | `library`, `executable`, `app` | Swift 6.4 for macOS 12 and newer, swift-format, SwiftLint, Swift Testing, an Xcode config |
| `cpp` | | C++20 beside Swift, clang-format and clang-tidy, xmake |
| `jvm` | `java-library`, `java-application`, `java-native`, `kotlin-library`, `kotlin-application`, `kotlin-native`, `kmp`, `cmp`, `android` | Gradle 9.9, GraalVM Community 25, Java 8 bytecode, Android 5 and newer, Error Prone and NullAway, google-java-format and ktfmt, ProGuard and R8, JUnit 6, J2K |

## What Every Repo Gets

Every language writes the same base, so a repo with several languages has one of each:

- a `justfile` that runs each task for every language, through `.just/<language>.just`, with the project's own recipes in `project.just`;
- `.githooks`: `pre-commit` runs the checks for the languages a commit touches, `pre-push` runs `just verify`, and `commit-msg` checks Conventional Commits;
- `AGENTS.md`, linked as `CLAUDE.md` and `GEMINI.md`, with a block of rules from each language;
- `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml`, with a block from each language;
- `.bedrock.json`, which pins the bedrock release and lists the languages;
- markdownlint, and a CI workflow per language.

Bedrock rewrites only the blocks it owns, between `# >>> name` and `# <<< name` comments (`<!-- name -->` and `<!-- /name -->` in Markdown), keeps the rest of each file, and lists the blocks in the same order whatever the languages. Git is the backup: bedrock leaves no `.bak` files, and refuses to change a file with uncommitted work unless you pass `--allow-dirty`. A second run on the same repo plans no steps.

```bash
just            # list the recipes
just setup      # install bedrock, the pinned tools, and the hooks
just verify     # every check CI runs
just rust::test # one language's recipe
```

`just setup` downloads the bedrock release `.bedrock.json` pins, checks it against the release's `SHA256SUMS`, and keeps one copy per release in the user cache, shared by every checkout and worktree.

## Adopt a Repo

From a Claude Code session with the plugin installed:

```text
/adopt                  # detect the languages, print the plan
/adopt ts rust --yes    # adopt these languages and apply
/bedrock guard          # any bedrock command
```

`/adopt`, and `/bedrock adopt`, `init`, or `update`, print the plan and change nothing until you add `--yes`.

From a shell, with a `bedrock` binary from the [releases](https://github.com/xsyetopz/bedrock/releases) or from a checkout of this repo (`bun cli/src/main.ts` in place of `bedrock`):

```bash
bedrock adopt <dir> --dry-run        # print the plan
bedrock adopt <dir> ts rust          # print it, then ask before applying
bedrock adopt <dir> swift cpp --yes  # apply without asking
```

Without a language, adopt takes the ones `.bedrock.json` lists or the repo has. Language names are case-insensitive, and `ts` or `bun`, `cxx` or `c++`, and `java` or `kotlin` work too. Then run `just setup` and `just verify` in the target and fix what they report. [docs/adopting.md](docs/adopting.md) lists what changes and the order to fix errors in. A repo adopted by a release before 0.2 has `.bak` files beside the files it replaced; `bedrock clean` deletes the ones git already holds, and lists the rest.

## Start a Repo

```bash
bedrock init <dir> rust:lib                 # or bin, no-std, ffi, wasm, bun-ffi
bedrock init <dir> swift:library cpp        # or executable, app
bedrock init <dir> typescript:cli           # or library, plugin
bedrock init <dir> jvm:kotlin-library       # see the table for every kind
```

`init` takes an empty or missing folder, writes the starter files for each kind, and adopts the result.

## Commands

| Command | What it does |
| --- | --- |
| `bedrock init <dir> <lang>[:<kind>]...` | Start a repo |
| `bedrock adopt [<dir>] [<lang>...]` | Move an existing repo onto bedrock |
| `bedrock update [<dir>]` | Rewrite the managed files with this release |
| `bedrock guard [base\|<lang>...] [--fix]` | Report what the linters cannot see |
| `bedrock clean [<dir>] [--force]` | Delete the `*.bak` files releases before 0.2 left |
| `bedrock languages [--staged]` | List the adopted languages, or those a commit touches |
| `bedrock swift convert-tests [<path>...]` | Convert XCTest unit tests to Swift Testing |
| `bedrock cpp compile-commands` | Write `compile_commands.json` for clang-tidy and clangd |
| `bedrock jvm j2k <path>...` | Convert Java files to Kotlin with IntelliJ IDEA's J2K |

`bedrock <command> --help` lists each command's options.

## Guards

`bedrock guard`, which `just verify` and the pre-commit hook run, catches what the linters can't:

- suppression comments and file-wide disables, where each tool has a narrower, reasoned form;
- files from replaced tools, such as another package manager's lockfile, a Podfile, `.swiftformat`, or `pom.xml`;
- Rust unit tests in a separate `tests.rs`, which `just lint-fix` moves inline;
- XCTest outside UI tests, which `just lint-fix` converts to Swift Testing, and JUnit 4 outside Android tests;
- ProGuard rules that turn shrinking off;
- public API without docs;
- managed files that drifted from what adopt writes.

## Claude Code Plugin

[`plugins/bedrock`](plugins/bedrock) adds `/bedrock` and `/adopt`, and in a repo with `bun.lock` it blocks Bash commands that run npm, npx, pnpm, pnpx, or yarn. This repo is a plugin marketplace:

```text
/plugin install bedrock --marketplace xsyetopz/bedrock
```

The plugin runs the CLI from a checkout of this repo, then a `bedrock` on `PATH` of the plugin's version, then a release binary it downloads and checks against `SHA256SUMS`. To try local changes, run `claude --plugin-dir plugins/bedrock`; `bun run plugin:validate` checks the plugin against Claude Code's plugin API.

## Repo Layout

| Path | Content |
| --- | --- |
| `cli/src/main.ts` | The `bedrock` CLI |
| `cli/src/core` | The plan, the managed blocks, the guard, and the logic every language shares |
| `cli/src/languages/<language>` | What each language plans, migrates, and guards |
| `cli/build.ts` | Compiles the CLI, with the templates embedded, into one binary per platform |
| `templates/base` | The files every repo gets |
| `templates/<language>` | The files, blocks, and starters one language adds |
| `plugins/bedrock` | The Claude Code plugin |

The repo adopts itself from source (`.bedrock.json` pins `"source"`), so `just verify` checks it with the CLI in its own tree.

## Docs

| File | Content |
| --- | --- |
| [docs/adopting.md](docs/adopting.md) | What adopt changes, and how to fix the errors that follow |
| [docs/jvm.md](docs/jvm.md) | How the Java and Kotlin setup works, and Java to Kotlin with J2K |
| [docs/tsconfig.md](docs/tsconfig.md) | Every TypeScript compiler option and why it's set |
| [docs/lint-rules.md](docs/lint-rules.md) | How oxlint and markdownlint are set up |
| [docs/node-apis.md](docs/node-apis.md) | When to use a Bun API and when a `node:` module |
| [AGENTS.md](AGENTS.md) | Instructions for AI agents |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | How to report a vulnerability |

## License

MIT. See [LICENSE](LICENSE).

`templates/base/files/.githooks/commit-msg.tmpl` is ported from [compilerla/conventional-pre-commit](https://github.com/compilerla/conventional-pre-commit) v4.5.0, licensed under [Apache-2.0](https://github.com/compilerla/conventional-pre-commit/blob/v4.5.0/LICENSE). The file header lists the changes.
