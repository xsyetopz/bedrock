# Adopting an Existing Repo

`bedrock adopt` moves a repo onto its strict setup, and works the same way for a fresh folder, an active project, or a legacy codebase. It removes the tools it replaces instead of running beside them. One run adopts every language you name, or, without one, the languages `.bedrock.json` lists or the repo has; the blocks in shared files land in the same order whatever the languages. `/adopt` in the [Claude Code plugin](../README.md#claude-code-plugin) runs it for you.

## Run It

```bash
bedrock adopt <target> --dry-run
bedrock adopt <target> ts rust
```

`bedrock` is a binary from the [releases](https://github.com/xsyetopz/bedrock/releases); from a checkout of this repo, `bun cli/src/main.ts` runs the same CLI. `--dry-run` prints the plan and stops. Without it, adopt prints the plan and asks `Apply these changes? [y/N]`. An agent or CI job, which has no terminal to answer in, passes `--yes`.

The target must be a git repository, and git is the backup: adopt refuses to change a file with uncommitted work unless you pass `--allow-dirty`, and it leaves no `.bak` files. Review the change with `git diff`, and move what a removed Makefile or hook manager did into `project.just`. When it finishes, run `just setup`, then `just verify`, in the target. A repo adopted by a release before 0.2 has `.bak` files beside the files it replaced; `bedrock clean` deletes those whose content git holds, and lists the rest for `bedrock clean --force`.

Project checks that `just verify` should run, such as a build or a repo-specific script, go in `project.just` as recipes named `project-<task>`: `project-verify`, `project-test`, `project-lint`, and so on. The shared `justfile` runs the matching one after each language's `<task>`. `project-pre-typecheck` is the one that runs before a language's task: the TypeScript `typecheck` runs it before `bun check`, so generated files the types import exist in CI and in the pre-commit hook. A tool those checks need on the CI runner, such as a compiler a test calls, goes in a composite action at `.github/actions/project-setup/action.yml`; the TypeScript and Swift workflows run it, when it exists, before their setup steps.

## What Every Language Changes

- It writes the shared `justfile`, `.githooks/`, `.markdownlint.jsonc`, and `.github/workflows/base.yml`, plus its own `.just/<language>.just` and `.github/workflows/<language>.yml`.
- It adds its block to `AGENTS.md`, `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml`, and keeps the rest of each file. An existing `CLAUDE.md` becomes `AGENTS.md`, and `CLAUDE.md` and `GEMINI.md` link to it.
- It records the bedrock release and the languages in `.bedrock.json`, which `just setup` reads to install that release.
- It removes hook managers (husky, lefthook, pre-commit, simple-git-hooks) and the `Makefile`, whose jobs the hooks and `just` take over.

## Bun

`bedrock adopt <target> typescript` (or `ts`, `bun`) edits `package.json` and the config files, and leaves your source code as it is, except for test files that move to `bun:test`. `--no-run` skips the `bun install` that follows, and the removal of the old lockfile after it.

### Package Manager

`bun install` imports `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `npm-shrinkwrap.json`, or `bun.lockb` into `bun.lock`. Adopt deletes the old lockfile only after `bun.lock` exists, so a failed install loses nothing. `package.json` gets `packageManager` and `engines.bun`, and loses any `npm`, `pnpm`, or `yarn` pin in `engines`.

### Scripts

Every adopted repo ends up with these scripts: `format`, `format:check`, `guard`, `lint`, `lint:fix`, `typecheck`, `verify`, and `test`. Your own `test` script stays, with the commands below rewritten; without one, adopt sets `bun test --pass-with-no-tests`. When one of the other names already ran something else, adopt prints the old command so you can move a step it still needs, such as a build or `cargo test`, into the new script. A script that installs a hook manager (`husky`, `lefthook install`, `simple-git-hooks`) is removed, because `just setup` points git at `.githooks`.

Commands inside your other scripts are rewritten:

| Before | After |
| --- | --- |
| `npm`, `pnpm`, `yarn` | `bun` |
| `npx`, `pnpx`, `dlx` | `bunx` |
| `node`, `tsx`, `ts-node`, `ts-node-dev`, `esno` | `bun` |
| `tsc --noEmit` | `bun check` |
| `jest`, `vitest` | `bun test` |
| `prettier`, `oxfmt` | `bun fmt` |
| `eslint`, `biome` | `oxlint` |
| `rimraf` | `rm -rf` |
| `cross-env A=1 cmd` | `A=1 cmd` |
| `typescript-bedrock guard` | `just typescript::guard` |

Scripts named `check` or `fmt` become `check:project` and `fmt:project`, so `bun check` and `bun fmt` keep running Bun's own commands. Test files that import `@jest/globals` or `vitest` import `bun:test` instead, and test files that call the Jest globals without an import get one from `bun:test`. Adopt leaves test files in `node_modules`, in paths that git ignores, and in `[test] pathIgnorePatterns` of `bunfig.toml` as they are. Adopt prints a note for each script that ran Jest or Vitest: move its setup files and other config into `[test]` in `bunfig.toml`, such as `preload`.

### Dependencies

Adopt removes the packages that Bun and bedrock replace: Biome, ESLint and its plugins, typescript-eslint, husky, Jest (with ts-jest, babel-jest, and `@types/jest`), Vitest, lefthook, lint-staged, simple-git-hooks, cross-env, esno, npm-run-all, rimraf, ts-node, tsx, and the `typescript-bedrock` package of releases before 0.2, from workspace packages too. It also removes their config files and their `package.json` fields (`eslintConfig`, `husky`, `jest`, `lint-staged`, `prettier`, `simple-git-hooks`, `pnpm`), plus `.eslintignore`, `jest.config.*`, `vitest.config.*`, `.yarnrc*`, `.yarn`, `.pnp.*`, `.pnpmfile.cjs`, and `pnpm-workspace.yaml`.

It adds the tools bedrock configures: oxlint, oxlint-tsgolint, markdownlint-cli2 with its title case rule, `eslint-plugin-jsdoc` for the JSDoc rule, TypeScript, `@types/bun`, and `@types/node`. Their versions go in `workspaces.catalog`, merged with yours and sorted by name. Where both pin a package, the template version wins, and the template's `overrides` entry for `@types/node` is added. A `pnpm-workspace.yaml` has its `packages`, `catalog`, and `catalogs` moved into `workspaces`, and its `onlyBuiltDependencies` into `trustedDependencies`. Adopt also sorts the keys of `package.json` in the order `bun fmt` writes them.

### TypeScript

`tsconfig.json` gets the template's compiler options written into it, with no `extends` to a bedrock file. Adopt then:

- removes the options TypeScript 7 rejects or always turns on, such as `esModuleInterop`, `downlevelIteration`, `alwaysStrict`, and `outFile`;
- turns `baseUrl` into `paths`;
- replaces any option the template sets with the template's value, so a weaker setting of yours gives way to the strict one, and prints a note when the value differs;
- keeps `jsx`, `lib`, and `types`, and adds `bun` to `types`;
- keeps your other options, your `exclude` entries, and any `extends` other than the old `typescript-bedrock` base file, with a note to check that parent;
- sets `noEmit: false` and `allowImportingTsExtensions: false` when the project emits with `tsc` (`composite`, `declaration`, `emitDeclarationOnly`, or `outDir`). Check `module` and `target` yourself in that case.

### Lint, Format, and Hooks

`.oxlintrc.json` and `.markdownlint-cli2.jsonc` are written as full configs, without `extends`. Adopt carries over the entries of your `ignorePatterns` (oxlint) or `ignores` (markdownlint) that the template lacks. Any other key of yours that differs from the template is dropped, with a note; git has it. Adopt replaces `.bun-version`, `.just/typescript.just`, and `.github/workflows/typescript.yml` with the templates. It removes a `.github/workflows/ci.yml` that runs npm, pnpm, or yarn, or the old `bun run verify`. A workflow or `project.just` that still mentions `typescript-bedrock` gets a note, since a person moves what it ran.

`bunfig.toml` is written as a full copy of the template, with its `[fmt]` style. Adopt keeps what the repo's copy adds to the template, with the comments right above it: each key a table lacks, each entry a template array lacks, and each table or `[[fmt.overrides]]` element the template lacks. Where the repo changes a value the template sets, the template's value wins, and adopt prints a note with the repo's value. A copy that is not valid TOML is replaced, with a note. The test runs print coverage but set no minimum, because Bun fails the run when any one file falls under the threshold. The template also turns off `.env` loading and install-on-run, and runs scripts on Bun.

Every adopted repo uses the one format style: adopt removes Prettier and oxfmt configs and the `prettier` field of `package.json`. After installing, adopt runs `bun fmt`, so expect that first commit to touch most files. `--no-run` skips the install and the format; run `just format` yourself then.

`bunfig.toml`, `package.json`, and `tsconfig.json` stay yours after the first write, so the drift guard skips them. The two lint configs keep only the repo's ignore entries: each adopt rewrites them from the template plus those entries, so the drift guard reports a changed rule but not an added ignore. `.vscode/extensions.json` and `.vscode/settings.json` are written only when the repo has none, so a tuned copy stays.

### Fixing the Errors

A legacy repo goes straight to the full strict config, so the first `just verify` usually reports many errors. The common ones:

- implicit `any`, values that can be `null` or `undefined`, and unchecked index access;
- types imported without `import type`;
- enums, namespaces, and constructor parameter properties;
- a caught `error` used as an `Error`;
- `obj.key` on a type with an index signature;
- CommonJS `require` and `module.exports`.

Work through them on a branch, in this order:

1. Run `just lint-fix`, which rewrites files, so review the diff.
2. Run `bun check` and fix one flag at a time:
   - `strict`: add the missing types, and narrow `null`, `undefined`, and caught values (`instanceof Error`).
   - `noUncheckedIndexedAccess`: an indexed read can be `undefined`, so check it before use.
   - `exactOptionalPropertyTypes`: write `x?: T | undefined` where an explicit `undefined` is meant, or stop passing one.
   - `noPropertyAccessFromIndexSignature`: write `obj["key"]`, or declare the property.
   - `verbatimModuleSyntax`: import types with `import type { T }` or `import { type T }`.
   - `erasableSyntaxOnly`: use `as const` objects and unions for enums, modules for namespaces, and plain fields assigned in the constructor for parameter properties.
   - `noImplicitReturns`, `noImplicitOverride`, `noUnused*`, `noFallthroughCasesInSwitch`: finish or remove what the error names.
   - `checkJs`: `.js` files are type checked too.
3. Convert CommonJS to ESM: `require` to `import`, `module.exports` to named exports, `__dirname` to `import.meta.dir`.
4. Run `just lint` and fix the rest one rule at a time. [lint-rules.md](lint-rules.md) explains the setup.
5. Run `just verify` until it passes. Its guard reports lint and type suppressions, other package managers' files and scripts, undocumented package API, TypeScript version drift, and managed-file drift.

## Rust

`bedrock adopt` needs a `Cargo.toml`; for a new repo, run `bedrock init <dir> rust:lib` (or `rust:bin`, `rust:no-std`, `rust:ffi`, `rust:wasm`, `rust:bun-ffi`). A `bun-ffi` crate is an `ffi` crate plus its Bun side in `src/`, so adopt `typescript` too. Adopt:

- writes `rustfmt.toml` and `clippy.toml`, writes `deny.toml` when the repo has none, and removes `.rustfmt.toml`, `.clippy.toml`, `rust-toolchain`, and `.cargo/config`;
- pins `rust-toolchain.toml` to Rust 1.100.0 with Clippy and rustfmt, keeping the targets the repo lists and adding `wasm32-unknown-unknown` for a `wasm` crate;
- gives the root `Cargo.toml` the managed workspace lints, resolver, and release profile, turning a single package into a workspace, and points each member at the workspace lints in place of its own. They include Cargo's `unused_dependencies`, so a `[dependencies]` or `[build-dependencies]` entry the crate never uses fails the build until you remove it; Cargo leaves `[dev-dependencies]` out;
- removes `cargo-husky`;
- moves each `foo/mod.rs` to `foo.rs`, prefixing its relative `#[path]` and `include!` paths with `foo/` so they reach the same files, and leaving a file whose paths it cannot rewrite safely where it is, with a note; starts each integration test in `tests/` with `#![cfg(test)]`, and gives it a `//!` crate doc when it has none, since `missing_docs` checks each test crate;
- removes the `xtask` crate, its workspace member, the `bedrock` alias in `.cargo/config.toml`, and the `deny.toml` `allow-git` entry that rust-bedrock releases wrote, then runs `cargo fetch`.

After applying, adopt migrates each manifest that declares an edition before 2024: it runs `cargo fix --edition` and raises the edition one step at a time, up to 2024. A package that declares no edition, which Cargo reads as 2015, gets `edition = "2015"` written in first, so it migrates too; a package that inherits its edition from the workspace moves with the workspace manifest. When a step fails, adopt stops with cargo's error, and the manifests keep the edition that step started from, with the earlier steps applied: fix the errors and run `bedrock update` to go on, or `git checkout` the change to undo it. `--no-run` skips the migration, and the plan lists it as skipped. A later `bedrock update` runs it even when nothing else changes, as long as a manifest still declares an older edition.

`just setup` installs the pinned toolchain, cargo-deny, and [kache](https://github.com/kunobi-ninja/kache), and runs `kache init --yes --no-shell`, which makes kache Cargo's compiler wrapper for your user. CI caches through `kache-action`.

To fix the errors, run `just lint-fix`, which moves each inline `#[cfg(test)] mod tests { … }` to a `tests.rs` file beside the file it tests (`bedrock guard rust --fix`), and applies Clippy's fixes to every target, tests included, then work through `just lint` one lint at a time. Silence a lint only with `#[expect(lint, reason = "…")]`, and document each public item, with `# Panics` and `# Errors` sections where they apply.

## Swift and C++

`bedrock adopt` on a Swift repo works with a `Package.swift` or an `.xcodeproj`; with neither, it prints a note, and `bedrock init <dir> swift:library` (or `swift:executable`, `swift:app`) starts a package. Naming `cpp`, or passing `--cxx`, turns on the C++ setup. Adopt never turns it on by itself: when it finds C, C++, or Objective-C++ files and `cpp` is not adopted, it prints a note to adopt `cpp` too. Adopt:

- writes `.swift-version`, `.swift-format`, and `.swiftlint.yml`, `Tests/.swiftlint.yml` when `Tests` exists, and `Config/Bedrock.xcconfig` when the repo has an Xcode project, and removes `.swiftformat`, `.swiftlint.yaml`, and `Mintfile`;
- raises `Package.swift` to tools version 6.4, raises a macOS platform below 12 to `.macOS(.v12)`, and adds the `// >>> swift-bedrock` block, which turns on Swift 6 mode, the upcoming features, strict memory safety, and warnings as errors for each target;
- runs `swift package migrate` for the upcoming features it can rewrite code for. InternalImportsByDefault and ImmutableWeakCaptures need a person;
- sets `Config/Bedrock.xcconfig` as the base configuration of each Xcode project's build configurations;
- removes the `BuildTools` package that swift-bedrock releases before 0.2 wrote, and notes each `project.just`, `.gitignore`, `.just/*.just`, or workflow file that still mentions it;
- with C++, adds the `// >>> swift-bedrock-cxx` block to `Package.swift`, which sets C++20 and turns on C++ interop for each Swift target, writes `.clang-format` and `.clang-tidy` in place of `_clang-format` and `_clang-tidy`, and writes `.just/cpp.just`. Adopt detects C++ from those configs or from a C, C++, or Objective-C file under `Sources/`. clang-tidy reads the compile database that `bedrock cpp compile-commands` writes from the SwiftPM targets and each xmake project, and `just setup` installs xmake when the repo tracks an `xmake.lua`.

The Swift recipes run SwiftPM with `--disable-sandbox`, so a build inside an outer sandbox, such as a coding agent's or a git hook's, does not fail on SwiftPM's nested one. On macOS, outside CI and without C++, they also cache compiler output across checkouts.

To fix the errors, run `just lint-fix`, which also converts XCTest files to Swift Testing outside `*UITests` targets (`bedrock swift convert-tests [<path>...]` does the same on its own), then work through `just lint`. Mark each unsafe expression with `unsafe`, spell existentials `any P`, and document each `public` or `open` declaration.

## Java and Kotlin

`bedrock adopt` writes the Gradle wrapper, `gradle/bedrock/` with the convention plugins, `gradle/bedrock.versions.toml`, and the ProGuard rules, and adds its blocks to `settings.gradle.kts` and `gradle.properties`. When `gradle.properties` names no `group`, adopt takes it from `--group`, then from the migrated build, or uses `com.example.<directory name>` for a new project, and otherwise prints a note asking you to set it; NullAway checks that package. A `--group`, kept in `.bedrock.json`, also replaces a different `group` that `gradle.properties` already names, with a note; the code stays where it is. Scala or Groovy sources get a note to port them first, since bedrock compiles Java and Kotlin only.

It then migrates the build it finds, in this order, to one `build.gradle.kts` per module that applies a `bedrock.*` plugin, with the dependencies in `gradle/libs.versions.toml`:

| Build | What it reads |
| --- | --- |
| Maven `pom.xml` | Modules, `groupId`, dependencies with their scopes, `dependencyManagement` imports and an external parent as platforms, `system` jars, and the main class from a plugin's `mainClass` or `Main-Class`, or the `start-class` or `exec.mainClass` property |
| sbt `build.sbt` | `organization`, each `project`, `dependsOn`, Java `%` dependencies, and `mainClass` |
| Ant `build.xml` and Ivy `ivy.xml` | The javac source folders, which move to `src/main/java` and `src/test/java`, the `Main-Class` attribute, Ivy dependencies by conf, and `lib/*.jar` |
| Gradle, either DSL | `include`s, `group`, the Kotlin plugin, `mainClass`, and each `dependencies` line in string, map, `project()`, `platform()`, or `libs.` form |

Adopt removes each old build file it migrates, so compare each new build with the old one in `git diff`. It prints a note for what it couldn't carry over: Java levels above 8, `%%` Scala dependencies, Maven classifiers, and Gradle lines it didn't parse. sbt and Gradle scripts are code, so read the new builds closely. It leaves Android and multiplatform Gradle modules, which need `bedrock.android`, `bedrock.kmp`, or `bedrock.cmp` applied by hand, and it names Mill and Bazel builds without reading them.

To fix the errors, run `just lint-fix`, then work through `just verify`: code that needs a JDK API newer than Java 8, Error Prone findings, and NullAway's reports, which `@Nullable` from JSpecify answers where a value can be null. [jvm.md](jvm.md) explains the setup, and `just jvm::j2k <path>…` converts Java to Kotlin.

## Commit the Migration

The pre-commit hook runs the checks of each language the commit touches, over the whole repo, so it accepts a commit once every error in those languages is fixed. Markdown is the exception: the hook lints only the staged `.md` files, as they are staged, with the staged Markdown configs, so neither an untracked draft nor an unstaged edit decides the result. Plan the migration as one branch that you commit when `just verify` passes.

When an agent does the work, have it fix each error at its cause and keep every rule, compiler flag, and severity as bedrock sets them. The guards reject suppression comments.
