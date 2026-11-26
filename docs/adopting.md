# Adopting an Existing Repo

`bedrock adopt` moves a repo onto its strict setup, and works the same way for a fresh folder, an active project, or a legacy codebase. It removes the tools it replaces instead of running beside them. One run adopts every language you name, or, without one, the languages `.editorconfig` lists or the repo has; the blocks in shared files land in the same order whatever the languages. `/adopt` in the [Claude Code plugin](../README.md#claude-code-plugin) runs it for you.

## Run It

```bash
bedrock adopt <target> --dry-run
bedrock adopt <target> ts rust
```

`bedrock` is a binary from the [releases](https://github.com/xsyetopz/bedrock/releases); from a checkout of this repo, `go build -o bin/bedrock ./cmd/bedrock` builds the same CLI. `--dry-run` prints the plan and stops. Without it, adopt prints the plan and asks `Apply these changes? [y/N]`. An agent or CI job, which has no terminal to answer in, passes `--yes`.

The target must be a git repository, and git is the backup: adopt refuses to change a file with uncommitted work unless you pass `--allow-dirty`, and it leaves no `.bak` files. Review the change with `git diff`, and move what a removed Makefile or hook manager did into `project.just`. When it finishes, run `just setup`, then `just verify`, in the target. A repo adopted by a release before 0.2 has `.bak` files beside the files it replaced; `bedrock clean` deletes those whose content git holds, and lists the rest for `bedrock clean --force`.

Project checks that `just verify` should run, such as a build or a repo-specific script, go in `project.just` as recipes named `project-<task>`: `project-verify`, `project-test`, `project-lint`, and so on. The shared `justfile` runs the matching one after each language's `<task>`. `project-pre-typecheck` is the one that runs before a language's task: the TypeScript `typecheck` runs it before `bun check`, so generated files the types import exist in CI and in the pre-commit hook. A tool those checks need on the CI runner, such as a compiler a test calls, goes in a composite action at `.github/actions/project-setup/action.yml`; the TypeScript and Swift workflows run it, when it exists, before their setup steps.

## What Every Language Changes

- It writes the shared `justfile`, `lefthook.yml`, `.markdownlint.jsonc`, and `.github/workflows/base.yml`, plus its own `.just/<language>.just` and `.github/workflows/<language>.yml`.
- It adds its block to `AGENTS.md`, `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml`, and keeps the rest of each file. An existing `CLAUDE.md` becomes `AGENTS.md`, and `CLAUDE.md` and `GEMINI.md` link to it.
- It records the bedrock release, the languages, and their options as `bedrock_*` properties in the bedrock block of `.editorconfig`; `just setup` reads `bedrock_version` to install that release. A `.bedrock.json` from an older release moves there.
- It removes what a language the repo dropped left behind: the language's blocks in the shared files and its managed files, such as `.just/rust.just` and `.github/workflows/rust.yml`. A language counts as dropped when the run does not cover it and none of its marker files sits at the root, such as `Cargo.toml` for Rust or `go.mod` for Go. Markers bedrock writes itself, such as `gradlew` or `.swift-version`, don't count, since they stay behind when the code goes. To drop one, take it out of `bedrock_languages` in `.editorconfig` and run `bedrock update`. `bedrock guard` reports the leftovers as drift.
- It removes other hook setups (husky, pre-commit, simple-git-hooks, the `.githooks` of bedrock releases before 0.5, and lefthook configs under names other than `lefthook.yml`) and the `Makefile`, whose jobs `lefthook.yml` and `just` take over. An existing `lefthook.yml` is replaced; machine-local hooks belong in `lefthook-local.yml`.

## Bun

`bedrock adopt <target> typescript` (or `ts`, `bun`) edits `package.json` and the config files, and leaves your source code as it is, except for test files that move to `bun:test`. `--no-run` skips the `bun install` that follows, and the removal of the old lockfile after it.

### Package Manager

`bun install` imports `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `npm-shrinkwrap.json`, or `bun.lockb` into `bun.lock`. Adopt deletes the old lockfile only after `bun.lock` exists, so a failed install loses nothing. `package.json` gets `packageManager` and `engines.bun`, and loses any `npm`, `pnpm`, or `yarn` pin in `engines`.

### Scripts

Every adopted repo ends up with these scripts: `format`, `format:check`, `guard`, `lint`, `lint:fix`, `typecheck`, `verify`, and `test`. Your own `test` script stays, with the commands below rewritten; without one, adopt sets `bun test --pass-with-no-tests`. When one of the other names already ran something else, adopt prints the old command so you can move a step it still needs, such as a build or `cargo test`, into the new script. A script that installs a hook manager (`husky`, `lefthook install`, `simple-git-hooks`) is removed, because `just setup` installs the hooks in `lefthook.yml` with the lefthook the justfile pins.

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

It adds the tools bedrock configures: oxlint, oxlint-tsgolint, markdownlint-cli2 with its title case rule, `eslint-plugin-jsdoc` for the JSDoc rule, TypeScript, `@types/bun`, and `@types/node`. Their versions go in `workspaces.catalog`, merged with yours and sorted by name. Where both pin a package, the template version wins unless yours is a later plain `MAJOR.MINOR.PATCH` release, such as a Dependabot bump, which stays; `typescript` always takes the template's pin, the version Bun ships. The template's `overrides` entry for `@types/node` is added the same way. A `pnpm-workspace.yaml` has its `packages`, `catalog`, and `catalogs` moved into `workspaces`, and its `onlyBuiltDependencies` into `trustedDependencies`. Adopt also sorts the keys of `package.json` in the order `bun fmt` writes them.

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

## Swift

`bedrock adopt` on a Swift repo works with a `Package.swift` or an `.xcodeproj`; with neither, it prints a note, and `bedrock init <dir> swift:library` (or `swift:executable`, `swift:app`) starts a package. Naming `cpp`, or passing `--cxx`, adds the C++ setup, which [C and C++](#c-and-c) describes; when a repo whose `.editorconfig` already lists its languages has C, C++, or Objective-C++ files and no `cpp`, adopt prints a note to add it. Adopt:

- writes `.swift-version`, `.swift-format`, and `.swiftlint.yml`, `Tests/.swiftlint.yml` when `Tests` exists, and `Config/Bedrock.xcconfig` when the repo has an Xcode project, and removes `.swiftformat`, `.swiftlint.yaml`, and `Mintfile`;
- raises `Package.swift` to tools version 6.4, raises a macOS platform below 13 to `.macOS(.v13)`, and adds the `// >>> swift-bedrock` block, which turns on Swift 6 mode, the upcoming features, strict memory safety, and warnings as errors for each target;
- runs `swift package migrate` for the upcoming features it can rewrite code for. InternalImportsByDefault and ImmutableWeakCaptures need a person;
- sets `Config/Bedrock.xcconfig` as the base configuration of each Xcode project's build configurations;
- removes the `BuildTools` package that swift-bedrock releases before 0.2 wrote, and notes each `project.just`, `.gitignore`, `.just/*.just`, or workflow file that still mentions it;
- with C++, adds the `// >>> swift-bedrock-cxx` block to `Package.swift`, which sets C++20 and turns on C++ interop for each Swift target.

The Swift recipes run SwiftPM with `--disable-sandbox`, so a build inside an outer sandbox, such as a coding agent's or a git hook's, does not fail on SwiftPM's nested one. On macOS, outside CI and without C++, they also cache compiler output across checkouts.

To fix the errors, run `just lint-fix`, which also converts XCTest files to Swift Testing outside `*UITests` targets (`bedrock swift convert-tests [<path>...]` does the same on its own), then work through `just lint`. Mark each unsafe expression with `unsafe`, spell existentials `any P`, and document each `public` or `open` declaration.

## C and C++

`cpp` stands alone or beside any other language. Adopt with no language named, in a repo whose `.editorconfig` lists none yet, detects it from `.clang-format` or `.clang-tidy`, a root `CMakeLists.txt`, `meson.build`, or `xmake.lua`, a C or C++ file at the top two levels, or a C, C++, or Objective-C file under `Sources/`. Adopt writes `.clang-format` and `.clang-tidy` in place of `_clang-format` and `_clang-tidy`, `.just/cpp.just`, and the C++ workflow. The recipes cover each build system the repo uses:

- `just lint` runs `bedrock cpp compile-commands`, then clang-tidy over the database it writes from SwiftPM's Clang targets, each CMake and Meson project, and each xmake project. A project is a build file no other one encloses, such as `port/CMakeLists.txt`. CMake configures with `-DBUILD_TESTING=ON` and Meson with its defaults, each in `.cache/bedrock/<system>/<project>`.
- `just test` runs `bedrock cpp test`: `cmake --build` and `ctest`, `meson test`, and `xmake test` for each project, every one even after another fails. `swift test` covers SwiftPM's C++ targets.
- `just setup` installs LLVM and uv, and CMake, Meson, or xmake when the repo tracks its build file.

A subproject that cannot configure, such as a starter that needs an SDK fetched first, is skipped with a warning, while the root project's failure fails the recipe. Bedrock configures with each build's defaults, so a project that needs a preset or an option to configure fails until it builds with its defaults. A repo built only by Make or another build system has no compile database, so clang-tidy and the tests see none of its files; clang-format and the size bounds still cover them.

## Java and Kotlin

`bedrock adopt` writes the Gradle wrapper, `gradle/bedrock/` with the convention plugins the builds apply, `gradle/bedrock.versions.toml`, and the ProGuard rules those plugins read, and adds its blocks to `settings.gradle.kts` and `gradle.properties`. When `gradle.properties` names no `group`, adopt takes it from `--group`, then from the migrated build, or uses `com.example.<directory name>` for a new project, and otherwise prints a note asking you to set it; NullAway checks that package. A `--group`, kept as `bedrock_jvm_group` in `.editorconfig`, also replaces a different `group` that `gradle.properties` already names, with a note; the code stays where it is. Scala or Groovy sources get a note to port them first, since bedrock compiles Java and Kotlin only.

It then migrates the build it finds, in this order, to one `build.gradle.kts` per module that applies a `bedrock.*` plugin, with the dependencies in `gradle/libs.versions.toml`:

| Build | What it reads |
| --- | --- |
| Maven `pom.xml` | Modules, `groupId`, dependencies with their scopes, `dependencyManagement` imports and an external parent as platforms, `system` jars, and the main class from a plugin's `mainClass` or `Main-Class`, or the `start-class` or `exec.mainClass` property |
| sbt `build.sbt` | `organization`, each `project`, `dependsOn`, Java `%` dependencies, and `mainClass` |
| Ant `build.xml` and Ivy `ivy.xml` | The javac source folders, which move to `src/main/java` and `src/test/java`, the `Main-Class` attribute, Ivy dependencies by conf, and `lib/*.jar` |
| Gradle, either DSL | `include`s, `group`, the Kotlin plugin, `mainClass`, and each `dependencies` line in string, map, `project()`, `platform()`, or `libs.` form |

Adopt removes each old build file it migrates, so compare each new build with the old one in `git diff`. It prints a note for what it couldn't carry over: Java levels above 8, `%%` Scala dependencies, Maven classifiers, and Gradle lines it didn't parse. sbt and Gradle scripts are code, so read the new builds closely. It leaves Android and multiplatform Gradle modules, which need `bedrock.android`, `bedrock.kmp`, or `bedrock.cmp` applied by hand; `bedrock update` then writes each plugin a build applies and removes those none applies any longer, and it names Mill and Bazel builds without reading them.

To fix the errors, run `just lint-fix`, then work through `just verify`: code that needs a JDK API newer than Java 8, Error Prone findings, and NullAway's reports, which `@Nullable` from JSpecify answers where a value can be null. [jvm.md](jvm.md) explains the setup, and `just jvm::j2k <path>…` converts Java to Kotlin.

## Haxe

`haxe` stands alone or beside any other language, such as a Haxe game compiled to JavaScript in a Bun repo. Adopt with no language named, in a repo whose `.editorconfig` lists none yet, detects it from `haxelib.json`, or a tracked hxml or `.hx` file. Adopt writes `.just/haxe.just`, the Haxe workflow, and a starter `checkstyle.json`, which stays the project's own, and ignores `.haxelib/`. The recipes run the pinned tools from the repo's local haxelib repo:

- `just haxe::setup` makes `.haxelib` with `haxelib newrepo` and installs haxe-checkstyle 2.9.0 and haxe-formatter 1.18.0 there. A `project-setup` recipe in `project.just` installs the project's own libraries, such as utest or a `haxelib dev` checkout, which `just setup` runs after it.
- `just haxe::format` runs the formatter over each top-level directory that holds Haxe sources, with the `hxformat.json` it finds; `format-check` only reports.
- `just haxe::lint` runs checkstyle with `checkstyle.json` and fails on an error-level finding. Its bounds mirror the other languages': 56 lines per method, complexity 15, 7 parameters, and a warning at 603 lines per file. Fix a finding in the code; loosen `checkstyle.json` only with the project owner's agreement.
- `just haxe::typecheck` compiles each hxml file that names its target, such as `--js` or `--hl`, from the repo root with `--no-output`, so no file is written. An hxml file without a target is a part a recipe completes, such as one that adds `-js <file>`, and its target decides which packages it may import, so the recipe that completes it checks it.
- `just haxe::test` runs each test hxml file, such as `test.hxml` or `test-native.hxml`, that says how its tests run: `--interp`, `--run`, or a `--cmd` after the compilation. Adopt notes each one that does not; a `project-test` recipe runs it on its target.

Adopt replaces a `.github/workflows/haxe.yml` it did not write and notes it, since the old one's steps are the project's. A step the checks need on the runner, such as checking out a library repository, goes in `.github/actions/project-setup/action.yml`, which the Haxe workflow runs after `just haxe::setup`, so its libraries land in `.haxelib`. The workflow checks out only the repo itself, so a build that reads a sibling directory, such as `../framework`, needs that library installed into `.haxelib` there instead. Builds through lime's `project.xml` are not read; a `project-verify` recipe runs them.

## .NET (C# and F#)

`dotnet` covers C# and F# on .NET 10, the current LTS, alone or beside any other language. Adopt with no language named detects it from `global.json`, a root `.sln` or `.slnx`, or a tracked `.csproj` or `.fsproj`. Adopt writes `.just/dotnet.just` and the .NET workflow, and moves the repo onto .NET 10:

- `global.json` pins the 10.0.100 SDK with `rollForward: latestFeature`. Its other keys stay, and an `sdk` already on 10 or later is left alone. When every test project is a Microsoft.Testing.Platform application, adopt also sets `test.runner` to `Microsoft.Testing.Platform`, since the .NET 10 SDK refuses to run such a project in its default VSTest mode.
- Each `TargetFramework` and `TargetFrameworks` in a project or `Directory.Build.props` that names `net5.0` through `net9.0` or a `netcoreapp` moves to `net10.0`, keeping a platform suffix such as `-windows`. `netstandard` and .NET Framework targets stay, since a library keeps them for older callers, and a value built from an MSBuild property stays too.
- The tool manifest, `.config/dotnet-tools.json` or the root `dotnet-tools.json` the repo already has, pins CSharpier 1.4.0, Fantomas 8.1.3, and FSharpLint 0.27.2; the project's other tools stay.
- A starter `Directory.Build.props` turns on nullable references, implicit usings, `AnalysisLevel` `latest-recommended`, the code style in the build, warnings as errors, and the XML documentation file, which IDE0005 needs. A repo with its own keeps it, and adopt notes the settings it lacks.
- The `.editorconfig` block sets the layout both formatters read, the C# code-style, naming, and CA rules at `warning`, and the size bounds.

Adopt notes a C# `LangVersion` below 14, which holds the code below what .NET 10 defaults to. The recipes run the pinned tools through `dotnet tool restore`:

- `just dotnet::format` runs CSharpier over the repo, which formats C# and the MSBuild XML and skips what `.gitignore` lists, and Fantomas over the tracked F# files. Both read `indent_size` and `max_line_length` from `.editorconfig`, so a `.csharpierrc` or `.fantomasrc` is a guard finding. `format-check` only reports.
- `just dotnet::typecheck` builds each root solution, or each project when there is none, with the analyzers off and warnings as errors.
- `just dotnet::lint` rebuilds with `EnforceCodeStyleInBuild`, so each `.editorconfig` rule at `warning` fails it, then runs FSharpLint over each `.fsproj`. `lint-fix` applies the fixes `dotnet format style` and `dotnet format analyzers` can make, then formats. IDE0055, the SDK's formatting rule, stays off, since CSharpier owns the layout.
- `just dotnet::test` runs `dotnet test` on each solution, or each test project, on the runner `global.json` names. On Microsoft.Testing.Platform it builds first, since that runner hands MSBuild switches to the test applications, then runs `dotnet test --solution` or `--project` with `--no-build` and at most `BEDROCK_DOTNET_JOBS` test applications at once. It ignores exit code 8, a run with no test, so a test project with none yet passes on either runner. A failing test still fails.
- `just dotnet::guard` reports `#pragma warning disable` and F# `#nowarn` without a trailing `// <reason>`, `SuppressMessage` without `Justification`, a formatter config beside `.editorconfig`, and two tool manifests, and bounds sizes with lizard.

A project's packages and properties decide its runner. Microsoft.Testing.Platform takes `xunit.v3`, `xunit.v3.mtp-v1`, `xunit.v3.mtp-v2`, `TUnit`, `MSTest.Sdk` without `UseVSTest`, `EnableMSTestRunner` beside an MSTest package, `EnableNUnitRunner` beside `NUnit3TestAdapter`, and `IsTestingPlatformApplication`. This holds even beside `xunit.runner.visualstudio`, and with `UseMicrosoftTestingPlatformRunner` set to false, since VSTest mode still refuses such a project. VSTest takes `Microsoft.NET.Test.Sdk`, the `MSTest` metapackage, `MSTest.TestAdapter`, `NUnit3TestAdapter`, `xunit.runner.visualstudio`, `YoloDev.Expecto.TestSdk`, or `IsTestProject`. That covers xUnit v2 and `xunit.v3.mtp-off`. The project file states them, unless a `Directory.Build.props` or `Directory.Build.targets` above it, or the `Directory.Packages.props`, names a test package or runner property. Then `dotnet msbuild` evaluates the project, which applies their conditions and needs no restore. Each evaluation takes about a second, `BEDROCK_DOTNET_JOBS` at a time. Without `dotnet` the project file alone decides. `dotnet test` runs one runner per repo, so adopt notes test projects split across both, and a `global.json` on the testing platform beside VSTest projects. Moving the VSTest projects over is yours to do.

Every build runs with `-maxCpuCount` from `BEDROCK_DOTNET_JOBS`, 4 by default, and no MSBuild node reuse, so no idle node keeps its memory. Adopt replaces a `.github/workflows/dotnet.yml` it did not write and notes it. Steps the checks need on the runner, such as a workload or a private NuGet feed, go in `.github/actions/project-setup/action.yml`. Other workflows, such as a `ci.yml`, stay, so remove the .NET steps they duplicate.

## Commit the Migration

The pre-commit hook runs the checks of each language the commit touches, over the whole repo, so it accepts a commit once every error in those languages is fixed. Markdown is the exception: the hook lints only the staged `.md` files, as they are staged, with the staged Markdown configs, so neither an untracked draft nor an unstaged edit decides the result. Plan the migration as one branch that you commit when `just verify` passes.

When an agent does the work, have it fix each error at its cause and keep every rule, compiler flag, and severity as bedrock sets them. The guards reject suppression comments.
