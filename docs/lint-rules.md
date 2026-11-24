# Lint Rules

oxlint checks the code and markdownlint-cli2 checks the Markdown. Both run with:

```bash
just lint        # report problems
just lint-fix    # fix what can be fixed, then report the rest
```

Bedrock writes `.oxlintrc.json` and `.markdownlint-cli2.jsonc` into each repo as full configs, from `templates/typescript/files/` in this repo, and `bedrock update` rewrites them with a newer release. A repo's own entries in their ignore lists stay, after the template's. Any other change gives way to the template, with a note, and that includes removing one of the template's own ignore entries: bedrock owns those, so adopt puts them back, and the drift check reports the file until then. To add or change a rule, edit the template here, set it to `"error"`, and fix what it reports.

## How `oxlint` Is Set Up

Every rule in the `correctness`, `suspicious`, `pedantic`, and `perf` categories is an error, along with the extra rules listed below. Warnings fail the run too (`denyWarnings`), and so does a disable comment that no longer suppresses anything.

oxlint loads its eslint, typescript, unicorn, oxc, import, promise, node, and jest plugins, and it runs type-aware rules through `oxlint-tsgolint`, which brings its own copy of TypeScript. `bunfig.toml` sets `[run] bun = true`, so oxlint runs on Bun even where Node.js is installed.

`ignorePatterns` leaves out dependencies, build and coverage output, generated `*.gen.*` files, agent worktrees in `.claude`, and OpenSpec proposals in `openspec`.

## Claude Code Plugins

Claude Code writes a plugin's ambient types to `.claude-plugin/types/claude-code/index.d.ts` when a run loads the plugin, and git-ignores them. A plugin tsconfig that lists those files is invalid wherever no run has happened, such as CI, and oxlint reports `Invalid tsconfig`. For each folder that holds `.claude-plugin/plugin.json`, whose `tsconfig.json` lists `.claude-plugin/types/`, and that lacks the types file, the `lint` recipe writes them with a non-interactive `claude -p` run (`CLAUDE_CODE_PLUGIN_DIR_WATCH=1`, a closed local port and a dummy key, so it bills nothing and touches no login) and then lints the folder. When `claude` is not on `PATH` or the run writes no file, the recipe skips the folder and prints one line to stderr naming it and the cause. A path argument inside the folder, such as `just typescript::lint plugins/<name>`, lints it regardless, prints the same cause, and fails while the types are absent.

## Suppression Comments

The setup has no escape hatch. `typescript/ban-ts-comment` rejects `@ts-ignore`, `@ts-expect-error`, and `@ts-nocheck`, and `bun run guard` rejects `eslint-disable`, `oxlint-disable`, and `biome-ignore` anywhere in the source. `@ts-check` is fine. When a rule fires, change the code until it passes.

## Extra Rules

The template turns on rules beyond the four categories. The full list is in `oxlintrc.json`; these are the ones you will meet most:

| Area | Rules |
| --- | --- |
| Style | `func-style` (expressions), `prefer-template`, `one-var` (never), `curly`, `no-nested-ternary`, `consistent-type-definitions` (interface), `array-type` (array) |
| Types | `consistent-type-imports`, `consistent-type-exports`, `no-explicit-any`, `no-non-null-assertion`, `method-signature-style`, `consistent-type-assertions` (only `as const`) |
| Modules | `import/no-default-export`, `import/no-cycle`, `import/no-commonjs`, `oxc/no-barrel-file`, `unicorn/prefer-node-protocol` |
| Promises | `promise/prefer-await-to-then`, `promise/no-nesting` |
| Files | `unicorn/filename-case` (kebab-case) |
| Runtime | `node/no-sync`, `unicorn/no-process-exit`, `eslint/no-console` (allows `error`, `info`, and `warn`) |
| Complexity | `eslint/complexity` (at most 15) |
| Tests | `jest/no-focused-tests`, `jest/no-disabled-tests`, `jest/expect-expect`, `jest/no-conditional-expect`, which also cover `bun:test` |

`consistent-type-assertions` allows only `as const`, so `as T` and `as unknown as T` fail. Narrow the value with a type guard.

`typescript/dot-notation` matches the `noPropertyAccessFromIndexSignature` compiler flag: write `obj.key` for a declared key and `Bun.env["HOME"]` for an index signature.

## Rules That Stay Off

| Rule | Reason |
| --- | --- |
| `typescript/prefer-readonly-parameter-types` | It wants every parameter deeply readonly, which library types such as `Request`, `BunFile`, and `Map` can never be. |
| `typescript/strict-void-return` | A callback whose result the caller ignores, such as `items.forEach((item) => seen.add(item))`, is fine where a void function is expected. |
| `eslint/no-throw-literal` | `typescript/only-throw-error` checks the thrown value's type, so it catches every literal this rule does, and a non-`Error` variable this rule cannot see. |
| `unicorn/no-new-array` | `new Array(size)` is about 3x faster than `Array.from({ length: size })` in Bun 1.4 on arm64, for 10,000 elements, and the types tell a length from an element. |
| `unicorn/number-literal-case` | `bun fmt` prints number literals in lowercase, hex digits included, with no option to change it, and this rule wants uppercase hex digits and takes no options in oxlint, so each would undo the other. The format check already keeps the `0x` prefix and the exponent in lowercase. |
| `typescript/promise-function-async` | It conflicts with `eslint/require-await`, which is on. |
| `eslint/no-undef` | It does not know `Bun`, `console`, or `process`, and `bun check` already reports undeclared names. |

## Per-File Exceptions

| Files | Exception |
| --- | --- |
| `*.test.*`, `*_test.*`, `*.spec.*`, `*_spec.*`, the files `bun test` runs | `max-lines-per-function`, `max-nested-callbacks`, and `no-script-url` are off. Tests pass `javascript:` URLs to sanitizers as data, which the rule cannot tell from code that runs them. |
| `scripts/**` | `no-console` is off. |
| `*.config.*` | Default exports are allowed. |
| `src/theme/**` | Default exports are allowed. Docusaurus imports a swizzled theme component as its default export. |
| `hooks/**` | `func-style`, `promise/prefer-await-to-then`, and `promise/no-callback-in-promise` are off. Claude Code's hook loader follows the engine object `$` across files only into a function declaration, and a hook registration's `.catch(handler)` is the framework's failure handler, not `Promise.prototype.catch`. |
| `hooks/register.{js,mjs,ts,mts}` | `import/max-dependencies` is off. `hooks.json` loads this one module, which imports every hook module and calls each in order, so its import count grows with the hook count. Every other file keeps the limit of 10. |
| `*.d.ts`, `*.d.mts`, `*.d.cts` | `typescript/no-namespace` is off. |

## Imports With a Bun Replacement

`eslint/no-restricted-imports` rejects these packages, in `import` statements and `import()` calls. Each message shows the Bun code to write in their place. [node-apis.md](node-apis.md) covers the rest of the Node.js API.

| Import | Bun replacement |
| --- | --- |
| `node:child_process`, `cross-spawn` | `Bun.spawn` or `Bun.$` |
| `execa`, `zx`, `shelljs` | `Bun.$`, or `Bun.spawn` for execa |
| `node:sqlite`, `better-sqlite3`, `sqlite3` | `bun:sqlite` |
| `node:test`, `jest`, `@jest/globals`, `vitest` | `bun:test` |
| `argon2`, `bcrypt` | `Bun.password` |
| `node-fetch`, `cross-fetch` | global `fetch` |
| `glob`, `fast-glob` | `Bun.Glob` |
| `js-yaml` | `Bun.YAML` |
| `uuid` | `crypto.randomUUID` or `Bun.randomUUIDv7` |

## Coming From Biome

The template once used Biome, and its strict rules carried over to oxlint. `func-style`, `import/no-mutable-exports`, `unicorn/no-array-reduce`, `unicorn/prefer-structured-clone`, `no-use-before-define`, and a few smaller rules come from that config. `no-use-before-define` runs with `variables: false`, so two functions can still call each other through a later `const`.

TypeScript covers some Biome rules itself: `erasableSyntaxOnly` rejects enums, modules reject duplicate parameters and octal escapes, and `noImplicitAny` catches most implicitly typed `let`s. A handful have no oxlint equivalent, such as `useTopLevelRegex`, `noReExportAll`, and `noUndeclaredDependencies`, along with the Playwright, Qwik, Vue, React Native, and Drizzle rule sets.

## Markdown

`bun fmt` pads Markdown tables and has no setting to stop it, so `bunfig.toml` leaves `*.md` out and markdownlint-cli2 checks Markdown in its place. `.markdownlint-cli2.jsonc` turns on every default rule except `line-length`, and adds two:

- `table-column-style` set to `compact`: one space around each cell, and `| --- |` separators;
- `title-case-style` from [`markdownlint-rule-title-case-style`](https://github.com/greyscaled/markdownlint-rule-title-case-style), set to `title`. Put identifiers in a heading in backticks, and the rule skips them.

`just lint-fix` fixes both. `.github/.markdownlint-cli2.jsonc` turns `first-line-heading` off for the pull request template, which GitHub shows as a form body.

## `bun lint`

Bun's own `bun lint` reads `.oxlintrc.json`, but only canary builds have it. On Bun 1.4.5, `bun lint` runs the `lint` script, so the template keeps calling oxlint directly until `bun lint` reaches a stable release.
