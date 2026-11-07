# TypeScript Bedrock

Shared tooling for strict Bun and TypeScript 7 repos. It holds the configs each repo extends, the commands its recipes run, and the `adopt` command that sets a repo up. The Release workflow publishes each version to npm.

| File | Used by |
| --- | --- |
| `tsconfig.base.json` | `tsconfig.json`: `"extends": "typescript-bedrock/tsconfig.base.json"` |
| `oxlintrc.json` | `.oxlintrc.json`: `"extends": ["./node_modules/typescript-bedrock/oxlintrc.json"]` |
| `markdownlint.jsonc` | `.markdownlint-cli2.jsonc`: `"config": { "extends": "./node_modules/typescript-bedrock/markdownlint.jsonc" }` |
| `files/` | The files `adopt` writes into a repo |

## Commands

| Command | Does |
| --- | --- |
| `typescript-bedrock adopt <target>` | Set up a repo. `--help` lists the options. |
| `typescript-bedrock guard` | Fail on suppression comments, other package managers, TypeScript drift, unsorted catalogs, package API without JSDoc, and files that differ from what `adopt` writes |

`adopt` writes `.just/typescript.just` and `.github/workflows/typescript.yml` beside the files every bedrock shares (`files/base`, a copy of `base/`). Git hooks, Markdown linting, and `AGENTS.md` come from the shared part; `just setup` points git at `.githooks`. `adopt` upserts its own `typescript-bedrock` block in `AGENTS.md`, `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml` and keeps the rest of each file. It keeps every file it replaces or removes as `<file>.bak`, and a second `adopt` plans nothing.

The tools it configures are peer dependencies. `adopt` adds them to the repo's catalog, so editors and `bun run lint` find them in the repo.

`files/` stores `bunfig.toml`, `.oxlintrc.json`, and `.markdownlint-cli2.jsonc` with a `.template` suffix: `bun pm pack` leaves out the first, and oxlint and markdownlint-cli2 would read the other two as configs for `files/`.
