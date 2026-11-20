# Bedrock

Strict, shared setups for TypeScript (on Bun), Rust, Swift, C and C++, Java and Kotlin, Go, Haxe, and C# and F# (.NET) repositories, alone or mixed in one repo. Bedrock is one CLI, `bedrock`, and a Claude Code plugin that runs it. It starts a repo, or moves an existing or legacy one onto its tooling, fully, so no old tool is left running beside it. People and AI agents run it the same way, and the repo it sets up depends on no bedrock package.

| Language | Kinds | Tooling |
| --- | --- | --- |
| `typescript` | `library`, `cli`, `plugin`, `server`, `react` | Bun 1.4.5, TypeScript 7 through `bun check`, `bun fmt`, oxlint with type-aware rules, `bun test` |
| `rust` | `lib`, `bin`, `cli`, `service`, `no-std`, `ffi`, `staticlib`, `proc-macro`, `wasm`, `bun-ffi` | Rust 1.100.0, edition 2024, workspace lints, Clippy, rustdoc lints, cargo-deny, kache, unit tests in `tests.rs` files |
| `swift` | `library`, `executable`, `tool`, `macro`, `build-tool-plugin`, `command-plugin`, `app` | Swift 6.4 for macOS 12 and newer, swift-format, SwiftLint, Swift Testing, an Xcode config |
| `cpp` | `library`, `header-only`, `executable` | C and C++ alone or beside any other language, clang-format and clang-tidy over the `compile_commands.json` of SwiftPM, CMake, Meson, and xmake, each build system's tests |
| `jvm` | `java-library`, `java-application`, `java-native`, `java-gradle-plugin`, `kotlin-library`, `kotlin-application`, `kotlin-native`, `kotlin-gradle-plugin`, `scala-library`, `scala-application`, `kmp`, `cmp`, `android` | Gradle 9.9, GraalVM Community 25, Java 8 bytecode, Android 5 and newer, Error Prone and NullAway, google-java-format and ktfmt, ProGuard and R8, JUnit 6, J2K |
| `go` | `command`, `library`, `service` | Go 1.27, golangci-lint 2.15 with a strict config, `go test`, `GOMEMLIMIT`, `-p`, and `run.concurrency` bounds on every recipe |
| `haxe` | `command`, `library` | Haxe 4.3.7 alone or beside any other language, haxe-formatter 1.18.0, haxe-checkstyle 2.9.0 with size bounds, a type check of each hxml build, the test hxml files that run their tests |
| `dotnet` | `console`, `library`, `webapi`, `worker`, `avalonia`, `android`, `apple` | .NET 10 LTS for C# and F#, CSharpier 1.4.0 and Fantomas 8.1.3 reading `.editorconfig`, the SDK analyzers with the `.editorconfig` code style as errors, FSharpLint 0.27.2, `dotnet test` |

## What Every Repo Gets

Every language writes the same base, so a repo with several languages has one of each:

- a `justfile` that runs each task for every language, through `.just/<language>.just`, with the project's own recipes in `project.just`; a `project-<task>` recipe there, such as `project-verify`, runs after the languages' `<task>`;
- `lefthook.yml`, whose hooks each run one `just` recipe: `pre-commit` checks the languages a commit touches, `pre-push` runs `verify` without the tests for the languages the unpushed commits touch, and `commit-msg` checks Conventional Commits;
- `AGENTS.md`, linked as `CLAUDE.md` and `GEMINI.md`, with a block of rules from each language;
- `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml`, with a block from each language;
- a bedrock block in `.editorconfig`, which pins the bedrock release, lists the languages with their options, and sets the size bounds `bedrock guard` checks;
- markdownlint, and a CI workflow per language.

Bedrock rewrites only the blocks it owns, between `# >>> name` and `# <<< name` comments (`<!-- name -->` and `<!-- /name -->` in Markdown), keeps the rest of each file, and lists the blocks in the same order whatever the languages. Git is the backup: bedrock leaves no `.bak` files, and refuses to change a file with uncommitted work unless you pass `--allow-dirty`. A second run on the same repo plans no steps.

```bash
just            # list the recipes
just setup      # install bedrock, the pinned tools, and the lefthook hooks
just verify     # every check CI runs
just rust::test # one language's recipe
```

`just setup` downloads the bedrock release `bedrock_version` in `.editorconfig` pins and the lefthook release the justfile pins, checks each against its release checksums, and keeps one copy per release in the user cache, shared by every checkout and worktree. It then installs the hooks `lefthook.yml` lists and unsets `core.hooksPath`, which releases before 0.5 pointed at `.githooks`, so rerun `just setup` in each clone after updating to 0.5. CI skips the hook install.

## Adopt a Repo

From a Claude Code session with the plugin installed:

```text
/adopt                  # detect the languages, print the plan
/adopt ts rust --yes    # adopt these languages and apply
/bedrock guard          # any bedrock command
```

`/adopt`, and `/bedrock adopt`, `init`, or `update`, print the plan and change nothing until you add `--yes`.

From a shell, with a `bedrock` binary from the [releases](https://github.com/xsyetopz/bedrock/releases) or built from a checkout of this repo with `go build -o bin/bedrock ./cmd/bedrock` (`bin/bedrock` in place of `bedrock`):

```bash
bedrock adopt <dir> --dry-run        # print the plan
bedrock adopt <dir> ts rust          # print it, then ask before applying
bedrock adopt <dir> swift cpp --yes  # apply without asking
```

Without a language, adopt takes the ones `.editorconfig` lists or the repo has. Language names are case-insensitive, and `ts` or `bun`, `cxx`, `c++`, or `c`, `java` or `kotlin`, `golang`, `hx`, and `.net`, `csharp`, `c#`, `cs`, `fsharp`, or `f#` work too. Then run `just setup` and `just verify` in the target and fix what they report. [docs/adopting.md](docs/adopting.md) lists what changes and the order to fix errors in. A repo adopted by a release before 0.2 has `.bak` files beside the files it replaced; `bedrock clean` deletes the ones git already holds, and lists the rest.

## Start a Repo

```bash
bedrock init <dir> rust:lib@ios,android      # see the table for every kind
bedrock init <dir> swift:library cpp         # a Swift package with C++ beside it
bedrock init <dir> swift:app@ios,macos       # one Xcode app for both platforms
bedrock init <dir> typescript:cli@linux,windows
bedrock init <dir> jvm:kmp@android,ios,wasm
bedrock init <dir> go:command@linux,wasi
bedrock init <dir> dotnet:avalonia@linux,macos,windows,android
```

`init` takes an empty or missing folder, writes the starter files for each kind, and adopts the result. `@` names the platforms a kind builds for, from this list; without it, a kind builds for the host:

| Language | Kind | Platforms |
| --- | --- | --- |
| `typescript` | `cli`, `server` | `linux`, `macos`, `windows`, through `bun build --compile` |
| `rust` | `lib`, `staticlib`, `ffi`, `no-std` | `android`, `ios`, `linux`, `macos`, `wasi`, `wasm`, `windows`, as the targets `rust-toolchain.toml` pins and `just rust::build-targets` builds |
| `rust` | `bin`, `cli` | `linux`, `macos`, `wasi`, `windows` |
| `rust` | `service` | `linux`, `macos`, `windows` |
| `swift` | `app` | `ios`, `macos`, `tvos`, `visionos`, `watchos` |
| `cpp` | `library`, `header-only` | `android`, `freebsd`, `ios`, `linux`, `maccatalyst`, `macos`, `tvos`, `visionos`, `wasi`, `wasm`, `watchos`, `windows`, as CMake presets |
| `cpp` | `executable` | `android`, `freebsd`, `linux`, `macos`, `wasi`, `wasm`, `windows` |
| `jvm` | `kmp` | the JVM and `android`, `ios`, `linux`, `macos`, `wasi`, `wasm`, `windows` |
| `jvm` | `cmp` | the JVM and `android`, `ios`, `linux`, `macos`, `wasm`, `windows` |
| `jvm` | `kotlin-native` | `linux`, `macos`, `windows` |
| `go` | `command` | `freebsd`, `linux`, `macos`, `wasi`, `wasm`, `windows`, through `just go::build` |
| `go` | `library` | `freebsd`, `ios`, `linux`, `macos`, `wasi`, `wasm`, `windows` |
| `go` | `service` | `freebsd`, `linux`, `macos`, `windows` |
| `haxe` | `command` | `linux`, `macos`, `windows`, through hxcpp and `just haxe::build` |
| `dotnet` | `avalonia` | `android`, `ios`, `linux`, `macos`, `wasm`, `windows` |
| `dotnet` | `apple` | `ios`, `maccatalyst`, `macos`, `tvos` |

A kind is cross-platform wherever its toolchain allows. A kind is Apple-only when it uses Apple frameworks, such as `swift:app` and `dotnet:apple`, and Android-only when only Android needs it, such as `jvm:android` and `dotnet:android`. Windows-only desktop frameworks get no kind, since `dotnet:avalonia` covers the same apps on every desktop.

## Commands

| Command | What it does |
| --- | --- |
| `bedrock init <dir> <lang>[:<kind>[@<platform>,...]]...` | Start a repo |
| `bedrock adopt [<dir>] [<lang>...]` | Move an existing repo onto bedrock |
| `bedrock update [<dir>]` | Rewrite the managed files with this release |
| `bedrock guard [base\|<lang>...] [--fix]` | Report what the linters cannot see |
| `bedrock clean [<dir>] [--force]` | Delete the `*.bak` files releases before 0.2 left |
| `bedrock languages [--staged]` | List the adopted languages, or those a commit touches |
| `bedrock swift convert-tests [<path>...]` | Convert XCTest unit tests to Swift Testing |
| `bedrock cpp compile-commands` | Write `compile_commands.json` for clang-tidy and clangd |
| `bedrock cpp test` | Build and run the tests of each CMake, Meson, and xmake project |
| `bedrock jvm j2k <path>...` | Convert Java files to Kotlin with IntelliJ IDEA's J2K |
| `bedrock haxe format [--check]` | Format the Haxe sources, or fail if one is not formatted |
| `bedrock haxe lint` | Run checkstyle over the Haxe sources |
| `bedrock haxe typecheck` | Type-check each hxml build that names its target |
| `bedrock haxe test` | Run each test hxml file that runs its tests |
| `bedrock dotnet restore` | Restore the NuGet packages of each solution or project |
| `bedrock dotnet format [--check]` | Format C#, F#, and MSBuild files with CSharpier and Fantomas, or fail if one is not formatted |
| `bedrock dotnet typecheck` | Build each solution or project without analyzers, warnings as errors |
| `bedrock dotnet lint [--fix]` | Rebuild with the analyzers and code style as errors, then run FSharpLint; `--fix` applies `dotnet format` fixes |
| `bedrock dotnet test` | Run the tests of each solution, or of each test project |

`bedrock <command> --help` lists each command's options.

## Guards

`bedrock guard`, which `just verify` and the pre-commit hook run, catches what the linters can't:

- suppression comments and file-wide disables, where each tool has a narrower, reasoned form;
- files from replaced tools, such as another package manager's lockfile, a Podfile, `.swiftformat`, or `pom.xml`;
- Rust unit tests inside the file they test, which `just lint-fix` moves to a file of the module's name beside it, such as `tests.rs`;
- XCTest outside UI tests, which `just lint-fix` converts to Swift Testing, and JUnit 4 outside Android tests;
- ProGuard rules that turn shrinking off;
- public API without docs;
- managed files that drifted from what adopt writes;
- code size where a language's own linters set no bound.

### Size Bounds

Each language's linters bound what they can: oxlint, golangci-lint, SwiftLint, Clippy, clang-tidy, and haxe-checkstyle check function length and complexity themselves. bedrock-lizard fills only the gaps, running the pinned [lizard](https://github.com/terryyin/lizard) 1.26.1 through `uvx` or `pipx`. `just setup` installs uv, and the CI workflows that run the guard install it too:

| Language | Native bounds | bedrock-lizard checks |
| --- | --- | --- |
| rust | Clippy: function lines, arguments | file code lines |
| cpp | clang-tidy: function size, cognitive complexity | file code lines |
| jvm | none | file code lines; function code lines, cyclomatic complexity, parameters |
| haxe | checkstyle: file and method length, cyclomatic complexity, parameters | none, since lizard reads no Haxe |
| dotnet | none | file code lines; function code lines, cyclomatic complexity, parameters, for C# and F# |

The bounds are [EditorConfig](https://spec.editorconfig.org/) properties in each language's block of `.editorconfig`, so they apply by glob like any other setting, and editors ignore them. A value over `bedrock_<metric>_error` fails the guard; a value over `bedrock_<metric>_warning` only prints. The metrics are `file_code_lines`, `test_file_code_lines`, `function_code_lines`, `function_complexity`, and `function_parameters`. A test file takes `test_file_code_lines` in place of `file_code_lines`. A test file is one the language's compiler, build, or test runner treats as test-only, not one a naming habit marks:

- Rust: a file a `#[cfg(test)] mod` declares and the files below it, a file `#![cfg(test)]` gates, and a crate's `tests/` directory. A `#[test]` function alone does not count, since rustc still builds the rest of its file;
- Java and Kotlin: a Gradle test source set, such as `src/test`, `src/testFixtures`, `src/androidTest`, or `src/commonTest`;
- C# and F#: the files under a test project's directory, where a test project references a test framework's runner or adapter (xUnit v2 or v3, NUnit, MSTest, TUnit, or Expecto), builds on `MSTest.Sdk`, or sets `IsTestProject`;
- C and C++: a SwiftPM package's `Tests/` directory, and a source only test targets compile: a CMake target a CTest test runs, a Meson target a `meson test` test runs, an xmake target `xmake test` runs, and a library only those link. A source a product target also compiles stays product code, as does one a product target compiles only under an option that is off by default.

The other languages' linters set their own test bounds the same way: oxlint by the names `bun test` runs, golangci-lint by `_test.go`, and SwiftLint by the `.swiftlint.yml` in `Tests/`. Haxe has no test-only file, since the compiler, haxelib, and utest treat a test class like any other, so checkstyle bounds tests as sources. The defaults come from studies of defect and maintenance cost: 56 code lines and complexity 15 per function, 7 parameters, and a warning at 603 code lines per file, since no study finds a file-length optimum.

A section after bedrock's blocks overrides a bound for the files it matches, `unset` turns one off, and adopt leaves those sections alone:

```ini
[*.kt]
bedrock_file_code_lines_warning = 800
bedrock_function_complexity_error = unset

[*.rs]
bedrock_test_file_code_lines_warning = 1500

[generated/**]
bedrock_file_code_lines_warning = unset
bedrock_function_code_lines_error = unset
```

`bedrock guard` reads each file's bounds the way an editor does: from the `.editorconfig` in its directory and each one above it, up to the nearest that sets `root = true`, so a nested `.editorconfig` overrides the root one for its subtree. Section globs follow the EditorConfig spec, `{1..12}` ranges included. The guard rejects a bound that is not a positive integer or `unset`, and names the `.editorconfig` that sets it.

## Claude Code Plugin

[`plugins/bedrock`](plugins/bedrock) adds `/bedrock` and `/adopt`, and in a repo with `bun.lock` it blocks Bash commands that run npm, npx, pnpm, pnpx, or yarn. This repo is a plugin marketplace named `bedrock`:

```text
/plugin marketplace add xsyetopz/bedrock
/plugin install bedrock@bedrock
```

The plugin runs the CLI from a checkout of this repo, then a `bedrock` on `PATH` of the plugin's version, then a release binary it downloads and checks against `SHA256SUMS`. To try local changes, run `claude --plugin-dir plugins/bedrock`; `bun run plugin:validate` checks the plugin against Claude Code's plugin API.

## Repo Layout

| Path | Content |
| --- | --- |
| `cmd/bedrock` | The `bedrock` CLI |
| `assets.go` | Embeds the templates and the version in every build |
| `internal/language` | What a language plugs in: a spec plus the capabilities it needs |
| `internal/catalog` | The languages this build knows, in block order |
| `internal/languages/<language>` | What each language plans, migrates, and guards |
| `internal/` | The plan, the managed blocks, the guard, and the logic every language shares |
| `project.just` | `just dist`, which builds one release binary per platform, with the templates embedded |
| `templates/base` | The files every repo gets |
| `templates/<language>` | The files, blocks, and starters one language adds |
| `plugins/bedrock` | The Claude Code plugin |

The repo adopts itself from source (`.editorconfig` pins `bedrock_version = source`), so `just verify` checks it with the CLI in its own tree.

## Docs

| File | Content |
| --- | --- |
| [docs/adopting.md](docs/adopting.md) | What adopt changes, and how to fix the errors that follow |
| [docs/go.md](docs/go.md) | How the Go setup works, and its resource bounds |
| [docs/jvm.md](docs/jvm.md) | How the Java and Kotlin setup works, and Java to Kotlin with J2K |
| [docs/tsconfig.md](docs/tsconfig.md) | Every TypeScript compiler option and why it's set |
| [docs/lint-rules.md](docs/lint-rules.md) | How oxlint and markdownlint are set up |
| [docs/node-apis.md](docs/node-apis.md) | When to use a Bun API and when a `node:` module |
| [AGENTS.md](AGENTS.md) | Instructions for AI agents |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute |
| [SECURITY.md](SECURITY.md) | How to report a vulnerability |

## License

MIT. See [LICENSE](LICENSE).

`internal/commitmsg` is ported from [compilerla/conventional-pre-commit](https://github.com/compilerla/conventional-pre-commit) v4.5.0, licensed under [Apache-2.0](https://github.com/compilerla/conventional-pre-commit/blob/v4.5.0/LICENSE). Its package comment lists the changes.
