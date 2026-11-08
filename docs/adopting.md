# Adopting an Existing Repo

Each bedrock's `adopt` moves a repo onto its strict setup, and works the same way for a fresh folder, an active project, or a legacy codebase. It removes the tools it replaces instead of running beside them. For a repo with several languages, run each bedrock's adopt; their blocks in the shared files land in the same order whichever runs first. `/adopt` in the [Claude Code plugin](../README.md#claude-code-plugin) runs them for you.

## Run It

The bedrocks aren't published yet, so run them from a clone of this repo:

```bash
git clone --depth 1 https://github.com/xsyetopz/bedrock ~/bedrock
bun ~/bedrock/packages/typescript-bedrock/src/cli.ts adopt <target> --package file:$HOME/bedrock/packages/typescript-bedrock --dry-run
cargo run --manifest-path ~/bedrock/packages/rust-bedrock/Cargo.toml -- adopt <target> --dry-run
swift run --package-path ~/bedrock swift-bedrock adopt <target> [--cxx] --dry-run
~/bedrock/gradlew --project-dir ~/bedrock :jvm-bedrock:installDist
~/bedrock/packages/jvm-bedrock/build/install/jvm-bedrock/bin/jvm-bedrock adopt <target> --dry-run
```

`--dry-run` prints the plan and stops. Without it, adopt prints the plan and asks `Apply these changes? [y/N]`. An agent or CI job, which has no terminal to answer in, passes `--yes`.

Keep the clone afterwards. The adopted repo installs `typescript-bedrock` from that folder until the package is published.

When it finishes, run `just setup`, then `just verify`, in the target. Each file adopt changed or removed is saved next to it as `<file>.bak`. Compare each backup with its replacement, move what a removed Makefile or hook manager did into `project.just`, and delete the backups; `.gitignore` already ignores them.

## What Every Bedrock Changes

- It writes the shared `justfile`, `.githooks/`, `.markdownlint.jsonc`, and `.github/workflows/base.yml`, plus its own `.just/<language>.just` and `.github/workflows/<language>.yml`.
- It adds its block to `AGENTS.md`, `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml`, and keeps the rest of each file. An existing `CLAUDE.md` becomes `AGENTS.md`, and `CLAUDE.md` and `GEMINI.md` link to it.
- It removes hook managers (husky, lefthook, pre-commit, simple-git-hooks) and the `Makefile`, whose jobs the hooks and `just` take over.

## Bun

`typescript-bedrock adopt` edits `package.json` and the config files, and leaves your source code as it is.

| Flag | Effect |
| --- | --- |
| `--package <spec>` | Install `typescript-bedrock` from `<spec>`, such as `file:<path>`. A version or range goes in the catalog; any other spec goes straight into `devDependencies`. |
| `--dry-run` | Print the plan and stop. |
| `--yes` | Apply the plan without asking. |
| `--no-install` | Skip `bun install` and the steps that run after it. |

Without `--package`, adopt keeps the repo's current `typescript-bedrock` spec, or pins the version it ships with. The target is the repo to change; adopt stops with an error when you point it at the `typescript-bedrock` folder.

### Package Manager

`bun install` imports `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `npm-shrinkwrap.json`, or `bun.lockb` into `bun.lock`, and adopt then deletes the old file. `package.json` gets `packageManager` and `engines.bun`, and loses any `npm`, `pnpm`, or `yarn` pin in `engines`.

### Scripts

Every adopted repo ends up with the same script names: `format`, `format:check`, `guard`, `lint`, `lint:fix`, `prepare`, `typecheck`, `verify`, and `test`. Your own `test` script stays, with the commands below rewritten. When one of the other names already ran something else, adopt prints the old command so you can move a step it still needs, such as a build or `cargo test`, into the new script.

Commands inside your other scripts are rewritten:

| Before | After |
| --- | --- |
| `npm`, `pnpm`, `yarn` | `bun` |
| `npx`, `pnpx` | `bunx` |
| `node`, `tsx`, `ts-node`, `ts-node-dev`, `esno` | `bun` |
| `tsc --noEmit` | `bun check` |
| `jest`, `vitest` | `bun test` |
| `prettier`, `oxfmt` | `bun fmt` |
| `eslint`, `biome` | `oxlint` |
| `rimraf` | `rm -rf` |
| `cross-env A=1 cmd` | `A=1 cmd` |

Scripts named `check` or `fmt` become `check:project` and `fmt:project`, so `bun check` and `bun fmt` keep running Bun's own commands. Test files that import `@jest/globals` or `vitest` import `bun:test` instead, and test files that call the Jest globals without an import get one from `bun:test`. Adopt prints a note for each script that ran Jest or Vitest: move its setup files and other config into `[test]` in `bunfig.toml`, such as `preload`.

### Dependencies

Adopt removes the packages that Bun and `typescript-bedrock` replace: Biome, ESLint and its plugins, typescript-eslint, husky, Jest, Vitest, lefthook, lint-staged, simple-git-hooks, cross-env, esno, npm-run-all, rimraf, ts-node, and tsx. It also removes their config files and their `package.json` fields, plus `.eslintignore`, `jest.config.*`, `vitest.config.*`, `.yarnrc*`, and `pnpm-workspace.yaml`.

It adds `typescript-bedrock` and the tools it configures: oxlint, oxlint-tsgolint, markdownlint-cli2 with its title case rule, TypeScript, `@types/bun`, and `@types/node`. Their versions go in `workspaces.catalog`, merged with yours and sorted by name. Where both pin a package, the template version wins. A `pnpm-workspace.yaml` has its `packages`, `catalog`, and `catalogs` moved into `workspaces`, and its `onlyBuiltDependencies` into `trustedDependencies`.

### TypeScript

`tsconfig.json` extends `typescript-bedrock/tsconfig.base.json`, after any `extends` you already had. Adopt then:

- removes the options TypeScript 7 rejects or always turns on, such as `esModuleInterop`, `downlevelIteration`, `alwaysStrict`, and `outFile`;
- turns `baseUrl` into `paths`;
- removes the options the base config sets, so a weaker setting of yours gives way to the strict one;
- keeps `jsx`, `lib`, and `types`, and adds `bun` to `types`;
- sets `noEmit: false` when the project emits with `tsc` (`composite`, `declaration`, `emitDeclarationOnly`, or `outDir`). Check `module` and `target` yourself in that case.

### Lint, Format, and Hooks

`.oxlintrc.json` and `.markdownlint-cli2.jsonc` extend the configs in `typescript-bedrock`. Adopt replaces these thin files whenever they differ from the template, along with `.bun-version`, `.just/typescript.just`, and `.github/workflows/typescript.yml`. It removes a `.github/workflows/ci.yml` that runs npm, pnpm, or yarn, or the old `bun run verify`.

`bunfig.toml` cannot extend another file, so adopt writes a full copy, with the template's `[fmt]` style. Every adopted repo uses that style: adopt removes Prettier and oxfmt configs and the `prettier` field of `package.json`, and after install it runs `bun fmt` over the repo, so `just format-check` passes from the start. Expect that first commit to touch most files. Adopt also runs `bun fmt --doctor` and removes the formatter packages it lists.

`.vscode/extensions.json` and `.vscode/settings.json` are written only when the repo has none, so a tuned copy stays.

### Fixing the Errors

A legacy repo goes straight to the full strict config, so the first `just verify` usually reports many errors. The common ones:

- implicit `any`, values that can be `null` or `undefined`, and unchecked index access;
- types imported without `import type`;
- enums, namespaces, and constructor parameter properties;
- a caught `error` used as an `Error`;
- `obj.key` on a type with an index signature;
- CommonJS `require` and `module.exports`.

Work through them on a branch, in this order:

1. Run `just format`, then `just lint-fix`. Both rewrite files, so review the diff.
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
5. Repeat `just verify` until it passes.

## Rust

`rust-bedrock adopt` needs a `Cargo.toml`; for a new repo, run `rust-bedrock new <dir> --kind lib` (or `bin`, `no-std`, `ffi`, `wasm`). Adopt:

- writes `rustfmt.toml` and `clippy.toml`, writes `deny.toml` when the repo has none, and removes `.rustfmt.toml`, `.clippy.toml`, `rust-toolchain`, and `.cargo/config`;
- pins `rust-toolchain.toml` to Rust 1.99.1 with Clippy and rustfmt, keeping the targets the repo lists;
- gives the root `Cargo.toml` the managed workspace lints, resolver, and release profile, turning a single package into a workspace, and points each member at the workspace lints in place of its own;
- adds the `xtask` member and the `cargo bedrock` alias in `.cargo/config.toml`, which run the guard;
- removes `cargo-husky`;
- moves each `foo/mod.rs` to `foo.rs`, and starts each integration test in `tests/` with `#![cfg(test)]`, and gives it a `//!` crate doc when it has none, since `missing_docs` checks each test crate;
- runs `cargo fix --edition` one edition at a time until every member is on 2024.

`just setup` installs cargo-deny and [kache](https://github.com/kunobi-ninja/kache), and runs `kache init`, which makes kache Cargo's compiler wrapper for your user and starts its background service. Set `KACHE_DISABLED=1` to build without it. CI caches through `kache-action`.

To fix the errors, run `just lint-fix`, which moves test modules from `tests.rs` files inline and applies Clippy's fixes to every target, tests included, then work through `just lint` one lint at a time. Silence a lint only with `#[expect(lint, reason = "…")]`, and document each public item, with `# Panics` and `# Errors` sections where they apply.

## Swift and C++

`swift-bedrock adopt` needs a `Package.swift` or an `.xcodeproj`; for a new repo, run `swift-bedrock new <dir> --kind library` (or `executable`, `app`). It turns on the C++ setup when you pass `--cxx` or the repo has C, C++, or Objective-C++ files. Adopt:

- writes `.swift-version`, `.swift-format`, and `.swiftlint.yml`, `Tests/.swiftlint.yml` when `Tests` exists, and `Config/Bedrock.xcconfig` when the repo has an Xcode project, and removes `.swiftformat`, `.swiftlint.yaml`, and `Mintfile`;
- raises `Package.swift` to tools version 6.4, raises a macOS platform below 12 to `.macOS(.v12)`, and adds the `// >>> swift-bedrock` block, which turns on Swift 6 mode, the upcoming features, strict memory safety, and warnings as errors for each target;
- runs `swift package migrate` for the upcoming features it can rewrite code for. InternalImportsByDefault and ImmutableWeakCaptures need a person;
- sets `Config/Bedrock.xcconfig` as the base configuration of each Xcode project's build configurations;
- adds the `BuildTools` package that runs the guard, pinned to the swift-bedrock release; a later adopt leaves that pin to you, unless `--bedrock-path` points it at a local checkout;
- with C++, writes `.clang-format` and `.clang-tidy` in place of `_clang-format` and `_clang-tidy`, and `.just/cpp.just`. clang-tidy reads the compile database that `swift-bedrock compile-commands` writes from the SwiftPM targets and each xmake project, and `just setup` installs xmake when the repo tracks an `xmake.lua`.

To fix the errors, run `just lint-fix`, which also converts XCTest files to Swift Testing outside `*UITests` targets, then work through `just lint`. Mark each unsafe expression with `unsafe`, spell existentials `any P`, and document each `public` or `open` declaration.

## Java and Kotlin

`jvm-bedrock adopt` writes the Gradle wrapper, `gradle/bedrock/` with the convention plugins, `gradle/bedrock.versions.toml`, and the ProGuard rules, and adds its blocks to `settings.gradle.kts` and `gradle.properties`. `--group` sets `group` in `gradle.properties` when the build doesn't name one; NullAway checks that package. It stops when the repo has Scala or Groovy sources, which bedrock doesn't compile.

It then migrates the build it finds, in this order, to one `build.gradle.kts` per module that applies a `bedrock.*` plugin, with the dependencies in `gradle/libs.versions.toml`:

| Build | What it reads |
| --- | --- |
| Maven `pom.xml` | Modules, `groupId`, dependencies with their scopes, `dependencyManagement` imports and an external parent as platforms, `system` jars, and the main class from a plugin's `mainClass` or `Main-Class`, or the `start-class` or `exec.mainClass` property |
| sbt `build.sbt` | `organization`, each `project`, `dependsOn`, Java `%` dependencies, and `mainClass` |
| Ant `build.xml` and Ivy `ivy.xml` | The javac source folders, which move to `src/main/java` and `src/test/java`, the `Main-Class` attribute, Ivy dependencies by conf, and `lib/*.jar` |
| Gradle, either DSL | `include`s, `group`, the Kotlin plugin, `mainClass`, and each `dependencies` line in string, map, `project()`, `platform()`, or `libs.` form |

Each old build file is kept as `<file>.bak`, and adopt prints a note for what it couldn't carry over: Java levels above 8, `%%` Scala dependencies, Maven classifiers, and Gradle lines it didn't parse. sbt and Gradle scripts are code, so compare each new build with the old one. It leaves Android and multiplatform Gradle modules, which need `bedrock.android`, `bedrock.kmp`, or `bedrock.cmp` applied by hand, and it names Mill and Bazel builds without reading them.

To fix the errors, run `just lint-fix`, then work through `just verify`: code that needs a JDK API newer than Java 8, Error Prone findings, and NullAway's reports, which `@Nullable` from JSpecify answers where a value can be null. [jvm.md](jvm.md) explains the setup, and `just jvm::j2k <path>…` converts Java to Kotlin.

## Commit the Migration

The pre-commit hook checks the whole repo, so it accepts a commit once every error is fixed. Plan the migration as one branch that you commit when `just verify` passes.

When an agent does the work, have it fix each error at its cause and keep every rule, compiler flag, and severity as the bedrock sets them. The guards reject suppression comments.
