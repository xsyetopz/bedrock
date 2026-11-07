# AGENTS.md

- Install with `bun install`, and run one-off tools with `bunx`.
- Check each change with the narrowest command: `bun test <file>`, `bun check`, `bun run lint`, or `bun fmt`.
- Run `bun run verify` once before you finish a change that alters behavior.
- Fix the cause of each lint or type error in the code, and keep the lint and `tsconfig` settings as they are.
- Pin each dependency version once, in `workspaces.catalog`, and reference it as `catalog:`.
- Use Bun APIs where Bun has one, such as `Bun.file`, `Bun.$`, and `Bun.spawn`.
- Change shared lint, type, and Markdown rules in `bun-bedrock`, which the repo configs extend.

<!-- bedrock -->
- Run each task through `just`. `just` lists the recipes. Check each change with the narrowest one, such as `just rust::test` or `just lint`, and run `just verify` once before you finish a change that alters behavior.
- Fix the cause of each lint, type, or guard error in the code. Keep the shared configs as they are; the bedrock that wrote them owns them.
- Put project recipes in `project.just`. The `justfile` and `.just/` are shared and get replaced on the next adopt.
- Write commit messages as Conventional Commits; the commit-msg hook checks them.
- When a recipe reports a missing tool, run `just setup`, which installs the pinned ones. Install any it can't, such as `just`, Bun, rustup, or Xcode, with the system's package manager, then rerun the recipe. Tell the user what you installed, and keep the check instead of skipping it.
<!-- /bedrock -->

<!-- rust-bedrock -->
- Keep Rust unit tests inline, as `#[cfg(test)] mod tests { … }` at the end of the file. `cargo bedrock guard` rejects a `mod tests;` whose body sits in another file, and `just lint-fix` moves it inline.
- Silence a lint only with `#[expect(lint, reason = "…")]`. Clippy rejects `#[allow]`, and an `#[expect]` that no longer fires.
- Start each integration test in `tests/` with `#![cfg(test)]`, and give its helpers `pub(crate)`.
- Name module files `foo.rs` beside `foo/`, not `foo/mod.rs`.
- Document each item the crate exports, with a `# Panics` section where it can panic and `# Errors` where it returns `Result`. `pub(crate)` and private items need no docs; `just lint` runs `cargo doc`, which rejects public docs that link to private items.
- Cargo compiles through kache, the build cache `just setup` installs and turns on. When a build is slower than it should be, `kache stats --last-build` and `kache explain` show which crates missed the cache and why.
<!-- /rust-bedrock -->

<!-- bun-bedrock -->
- Install with `bun install`, and run one-off tools with `bunx`.
- Check each change with the narrowest command: `bun test <file>`, `bun check`, `bun run lint`, or `bun fmt`.
- Fix the cause of each lint or type error in the code, and keep the lint and `tsconfig` settings as they are.
- Pin each dependency version once, in `workspaces.catalog`, and reference it as `catalog:`.
- Give a JSDoc comment to each declaration a package exports from its `exports`, `main`, or `module` entry; internal code needs none.
- Use Bun APIs where Bun has one, such as `Bun.file`, `Bun.$`, and `Bun.spawn`.
- Change shared lint, type, and Markdown rules in `bun-bedrock`, which the repo configs extend.
<!-- /bun-bedrock -->

<!-- swift-bedrock -->
- Write tests with Swift Testing: `import Testing`, `@Test`, `#expect`, and `#require`. XCTest is only for `*UITests` targets, which need `XCUIApplication`; `just lint-fix` converts other XCTest files.
- Keep the `// >>> swift-bedrock` block at the end of `Package.swift`. It turns on Swift 6 mode, the upcoming features, strict memory safety, and warnings as errors for each target.
- Mark an unsafe expression with `unsafe`, and spell an existential `any P`.
- Build an app in Xcode against `Config/Bedrock.xcconfig`, which sets the same flags as the package.
- Document each `public` or `open` declaration other modules can reach. Internal and private code needs no docs, and SwiftLint reports a `public` member of an internal type instead.
<!-- /swift-bedrock -->
