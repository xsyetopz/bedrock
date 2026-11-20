# AGENTS.md

- Bedrock is one Go CLI, `cmd/bedrock`, and the Claude Code plugin in `plugins/bedrock`. `just bedrock <command>` builds it into `bin/` and runs it from source.
- Put what every repo gets in `templates/base` and what one language adds in `templates/<language>`; the code that plans and guards it goes in `internal/languages/<language>`, registered in `internal/catalog`. Shared logic belongs in the core packages under `internal/`, and the capabilities a language can implement are in `internal/language`.
- Write each managed file through a template, and keep the plan idempotent: a second `bedrock adopt` on the same repo plans no steps.
- Give each language kind an e2e test that inits it and runs its real toolchain. Start it with `bedrocktest.E2E(t, ...)`, which skips it unless `BEDROCK_E2E=1` is set, and skip it too when the toolchain is absent. CI's e2e workflow sets the variable; run one locally with `BEDROCK_E2E=1 go test -run <Test> ./internal/languages/<language>/`, since the matrix takes minutes of every core.
- Make each kind cross-platform and list the platforms it builds for in `Spec.Platforms`, such as an Avalonia desktop app `@macos,linux,windows`. A kind is platform-specific only when the platform requires it: Apple-only for Apple frameworks, Android-only for what only Android needs. Legacy single-platform frameworks such as WPF and WinForms get no kind, because a cross-platform one covers the same need.
- Keep `bedrock_version` in `.editorconfig` on `source`, so this repo checks itself with the code in its own tree.

<!-- bedrock -->
- Run each task through `just`. `just` lists the recipes. Check each change with the narrowest one, such as `just rust::test` or `just lint`, and run `just verify` once before you finish a change that alters behavior.
- Fix the cause of each lint, type, or guard error in the code. Keep the shared configs as they are; the bedrock that wrote them owns them.
- Put project recipes in `project.just`. The `justfile` and `.just/` are shared and get replaced on the next adopt. A recipe there named `project-<task>`, such as `project-verify` or `project-test`, runs after the languages' `<task>`, so a project extends `just verify` there. `project-pre-typecheck` runs before the TypeScript type check, for a build the types import.
- Write commit messages as Conventional Commits; the commit-msg hook checks them.
- When a recipe reports a missing tool, run `just setup`, which installs the pinned ones. Install any it can't, such as `just`, Bun, rustup, or Xcode, with the system's package manager, then rerun the recipe. Tell the user what you installed, and keep the check instead of skipping it.
<!-- /bedrock -->

<!-- typescript-bedrock -->
- Install with `bun install`, and run one-off tools with `bunx`.
- Check each change with the narrowest command: `bun test <file>`, `bun check`, `just typescript::lint <path>`, or `bun fmt`. A lint run starts type-aware tsgolint, which holds the whole program in memory, so run it through `just` (one run on the machine at a time) and not as a bare `bunx oxlint`.
- Fix the cause of each lint or type error in the code, and keep the lint and `tsconfig` settings as they are.
- Pin each dependency version once, in `workspaces.catalog`, and reference it as `catalog:`.
- Give a JSDoc comment to each declaration a package exports from its `exports`, `main`, or `module` entry; internal code needs none.
- Use Bun APIs where Bun has one, such as `Bun.file`, `Bun.$`, and `Bun.spawn`.
- Change the shared lint, type, and Markdown rules in bedrock's `templates/typescript`; this repo's configs are copies that `bedrock update` rewrites.
<!-- /typescript-bedrock -->

<!-- go-bedrock -->
- Build and test Go through `just`: `just go::test`, `just go::lint`, and `just go::format`. The recipes cap parallelism with `GOMEMLIMIT` and `GOFLAGS=-p`; `BEDROCK_GO_MEMLIMIT` and `BEDROCK_GO_JOBS` raise the caps on a dedicated machine.
- Put commands under `cmd/<name>/` and private code under `internal/`, as the golang-standards project layout does.
- Fix the cause of each golangci-lint finding in the code. `.golangci.yml` is the project's own once written, but loosen it only with the user's approval, and keep `run.concurrency` set.
- Silence a finding only with `//nolint:<linter> // <reason>`; nolintlint rejects a directive without a linter name or a reason.
- Keep `go.mod` and `go.sum` tidy: `just go::guard` fails when `go mod tidy` would change them.
<!-- /go-bedrock -->
