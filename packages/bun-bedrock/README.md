# Bun Bedrock

Shared tooling for strict Bun and TypeScript 7 repos. It holds the configs each repo extends, the commands its recipes run, and the `adopt` command that sets a repo up. It isn't on npm yet.

| File | Used by |
| --- | --- |
| `tsconfig.base.json` | `tsconfig.json`: `"extends": "bun-bedrock/tsconfig.base.json"` |
| `oxlintrc.json` | `.oxlintrc.json`: `"extends": ["./node_modules/bun-bedrock/oxlintrc.json"]` |
| `markdownlint.jsonc` | `.markdownlint-cli2.jsonc`: `"config": { "extends": "./node_modules/bun-bedrock/markdownlint.jsonc" }` |
| `files/` | The files `adopt` writes into a repo |

## Commands

| Command | Does |
| --- | --- |
| `bun-bedrock adopt <target>` | Set up a repo. `--help` lists the options. |
| `bun-bedrock guard` | Fail on suppression comments, other package managers, TypeScript drift, unsorted catalogs, package API without JSDoc, and files that differ from what `adopt` writes |

`adopt` writes `.just/bun.just` and `.github/workflows/bun.yml` beside the files every bedrock shares (`files/base`, a copy of `base/`). Git hooks, Markdown linting, and `AGENTS.md` come from the shared part; `just setup` points git at `.githooks`. `adopt` upserts its own `bun-bedrock` block in `AGENTS.md`, `.gitignore`, `.gitattributes`, `.editorconfig`, and `.github/dependabot.yml` and keeps the rest of each file. It keeps every file it replaces or removes as `<file>.bak`, and a second `adopt` plans nothing.

The tools it configures are peer dependencies. `adopt` adds them to the repo's catalog, so editors and `bun run lint` find them in the repo.

`files/` stores `bunfig.toml`, `.oxlintrc.json`, and `.markdownlint-cli2.jsonc` with a `.template` suffix: `bun pm pack` leaves out the first, and oxlint and markdownlint-cli2 would read the other two as configs for `files/`.
