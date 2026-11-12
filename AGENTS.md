# AGENTS.md

- Install with `bun install`, and run one-off tools with `bunx`.
- Check each change with the narrowest command: `bun test <file>`, `bun check`, `bun run lint`, or `bun fmt`.
- Run `bun run verify` once before you finish a change that alters behavior.
- Fix the cause of each lint or type error in the code, and keep the lint and `tsconfig` settings as they are.
- Pin each dependency version once, in `workspaces.catalog`, and reference it as `catalog:`.
- Use Bun APIs where Bun has one, such as `Bun.file`, `Bun.$`, and `Bun.spawn`.
- Change shared lint, type, and Markdown rules in `typescript-bedrock`, which the repo configs extend.

<!-- bedrock -->
- Run each task through `just`. `just` lists the recipes. Check each change with the narrowest one, such as `just rust::test` or `just lint`, and run `just verify` once before you finish a change that alters behavior.
- Fix the cause of each lint, type, or guard error in the code. Keep the shared configs as they are; the bedrock that wrote them owns them.
- Put project recipes in `project.just`. The `justfile` and `.just/` are shared and get replaced on the next adopt. A recipe there named `project-<task>`, such as `project-verify` or `project-test`, runs after the languages' `<task>`, so a project extends `just verify` there. `project-pre-typecheck` runs before the TypeScript type check, for a build the types import.
- Write commit messages as Conventional Commits; the commit-msg hook checks them.
- When a recipe reports a missing tool, run `just setup`, which installs the pinned ones. Install any it can't, such as `just`, Bun, rustup, or Xcode, with the system's package manager, then rerun the recipe. Tell the user what you installed, and keep the check instead of skipping it.
<!-- /bedrock -->

<!-- rust-bedrock -->
- Keep Rust unit tests out of the file they test: declare `#[cfg(test)] mod tests;` with no `#[path]`, and put the body in `foo/tests.rs` beside `foo.rs`, or in `tests.rs` beside `lib.rs`, `main.rs`, or `mod.rs`. `cargo bedrock guard` rejects a `#[cfg(test)] mod name { … }` with its body in the file, and `just lint-fix` moves it out.
- Silence a lint only with `#[expect(lint, reason = "…")]`. Clippy rejects `#[allow]`, and an `#[expect]` that no longer fires.
- Start each integration test in `tests/` with `#![cfg(test)]`, and give its helpers `pub(crate)`.
- Name module files `foo.rs` beside `foo/`, not `foo/mod.rs`.
- Document each item the crate exports, with a `# Panics` section where it can panic and `# Errors` where it returns `Result`. `pub(crate)` and private items need no docs; `just lint` runs `cargo doc`, which rejects public docs that link to private items.
- Cargo compiles through kache, the build cache `just setup` installs and turns on. When a build is slower than it should be, `kache stats --last-build` and `kache explain` show which crates missed the cache and why.
<!-- /rust-bedrock -->

<!-- typescript-bedrock -->
- Install with `bun install`, and run one-off tools with `bunx`.
- Check each change with the narrowest command: `bun test <file>`, `bun check`, `bun run lint`, or `bun fmt`.
- Fix the cause of each lint or type error in the code, and keep the lint and `tsconfig` settings as they are.
- Pin each dependency version once, in `workspaces.catalog`, and reference it as `catalog:`.
- Give a JSDoc comment to each declaration a package exports from its `exports`, `main`, or `module` entry; internal code needs none.
- Use Bun APIs where Bun has one, such as `Bun.file`, `Bun.$`, and `Bun.spawn`.
- Change shared lint, type, and Markdown rules in `typescript-bedrock`, which the repo configs extend.
<!-- /typescript-bedrock -->

<!-- swift-bedrock -->
- Write tests with Swift Testing: `import Testing`, `@Test`, `#expect`, and `#require`. XCTest is only for `*UITests` targets, which need `XCUIApplication`; `just lint-fix` converts other XCTest files.
- Keep the `// >>> swift-bedrock` block at the end of `Package.swift`. It turns on Swift 6 mode, the upcoming features, strict memory safety, and warnings as errors for each target.
- Mark an unsafe expression with `unsafe`, and spell an existential `any P`.
- Build an app in Xcode against `Config/Bedrock.xcconfig`, which sets the same flags as the package.
- Build Swift through `just`, which turns on the compile cache every checkout shares, in a repository without C or C++. When worktrees fill the disk, `just swift::clean-worktrees` deletes each `.build` folder no process is using.
- Document each `public` or `open` declaration other modules can reach. Internal and private code needs no docs, and SwiftLint reports a `public` member of an internal type instead.
<!-- /swift-bedrock -->

<!-- jvm-bedrock -->
- Build Java and Kotlin with `./gradlew` through `just`. Each module's `build.gradle.kts` applies one `bedrock.*` plugin: `bedrock.java`, `bedrock.kotlin`, `bedrock.application`, `bedrock.native-image`, `bedrock.kmp`, `bedrock.cmp`, `bedrock.android`, or `bedrock.kotlin-native`. They compile with GraalVM Community 25 to Java 8 bytecode, with warnings as errors.
- Keep main code on the Java 8 API and Android 5 (API 21). The compilers reject newer JDK APIs, and Android Lint rejects newer Android APIs.
- Pin each dependency once, in `gradle/libs.versions.toml`, and reference it as `libs.…`. `gradle/bedrock.versions.toml` and `gradle/bedrock/` belong to jvm-bedrock, and `just guard` reports a change to them.
- Mark nullable Java types with JSpecify's `@Nullable`; NullAway treats the rest of the project's packages as non-null.
- Silence a warning only with `@SuppressWarnings` or `@Suppress` and a `//` comment on it or the line above that says why.
- Write tests with JUnit 6, `org.junit.jupiter.api`, or `kotlin.test`. JUnit 4 is only for Android tests.
- Keep only the classes reflection needs in a module's `proguard-rules.pro`; `just guard` rejects rules that turn shrinking, optimizing, or obfuscating off.
- Port Java to Kotlin with `just jvm::j2k <path>…`, which runs IntelliJ IDEA's converter and rewrites the files in place, instead of porting by hand. Commit first, then fix what the build reports.
<!-- /jvm-bedrock -->
