# TypeScript Config

Every compiler option lives in `bun-bedrock/tsconfig.base.json`. A repo's `tsconfig.json` extends it and adds only `rootDir` and `exclude`. `bun check` reads `tsconfig.json`, type checks with the TypeScript 7 built into Bun, and never writes files. `bun check --help` lists its flags.

## Runtime and Modules

Bun runs TypeScript directly, so nothing is compiled down or bundled for type checking.

| Option | Value | Reason |
| --- | --- | --- |
| `target` | `ESNext` | Bun runs the code as written. |
| `lib` | `["ESNext"]` | Server code, with no DOM types. |
| `types` | `["bun"]` | Since TypeScript 6, a `@types` package counts only when listed. |
| `module` | `Preserve` | Module syntax stays as written. |
| `moduleResolution` | `bundler` | Imports resolve the way Bun resolves them. |
| `moduleDetection` | `force` | Every file is a module. |
| `jsx` | `preserve` | |
| `allowImportingTsExtensions` | `true` | Imports name the `.ts` file, as Bun expects. |
| `allowArbitraryExtensions` | `true` | `./a.css` can be imported when a `.d.css.ts` file describes it. |
| `resolveJsonModule` | `true` | JSON files can be imported. |
| `verbatimModuleSyntax` | `true` | Types come in through `import type`, and imports stay as written. |
| `isolatedModules` | `true` | Each file compiles on its own. |
| `erasableSyntaxOnly` | `true` | Only syntax Bun can strip: no enums, value namespaces, or parameter properties. |
| `noUncheckedSideEffectImports` | `true` | `import "./x"` must point at a real file. |
| `forceConsistentCasingInFileNames` | `true` | |
| `libReplacement` | `false` | TypeScript skips the lookup for `@typescript/lib-*` packages. |

`allowJs` and `checkJs` are on, so JavaScript files are type checked too. `noEmit` and `skipLibCheck` are on.

## Strictness

| Option | Value | Effect |
| --- | --- | --- |
| `strict` | `true` | Turns on the whole strict family. |
| `exactOptionalPropertyTypes` | `true` | `x?: string` accepts `undefined` only when written as `x?: string \| undefined`. |
| `noUncheckedIndexedAccess` | `true` | `array[i]` and index-signature reads include `undefined`. |
| `noPropertyAccessFromIndexSignature` | `true` | Index-signature keys are read with brackets: `env["HOME"]`. |
| `noImplicitOverride` | `true` | An overriding method says `override`. |
| `noImplicitReturns` | `true` | Every code path returns. |
| `noFallthroughCasesInSwitch` | `true` | |
| `noUnusedLocals` | `true` | |
| `noUnusedParameters` | `true` | |
| `allowUnreachableCode` | `false` | Unreachable code is an error. |
| `allowUnusedLabels` | `false` | An unused label is an error. |
| `useDefineForClassFields` | `true` | |

The `typescript/dot-notation` lint rule follows the same split as `noPropertyAccessFromIndexSignature`: dots for declared keys, brackets for index signatures. See [lint-rules.md](lint-rules.md).

## Editor Support

Editors use the `typescript` package installed in the project, while `bun check` uses the copy built into Bun. The catalog pins `typescript` to the version Bun ships, so both report the same errors, and `bun run guard` fails when they drift apart. To bring them back in line:

```bash
bun add -d typescript@$(bun -p process.versions.typescript)
```

TypeScript 7 has no `tsserver`, so VS Code needs the TypeScript Native Preview extension (`TypeScriptTeam.native-preview`). `.vscode/extensions.json` recommends it.

## One Copy of `@types/node`

`bun-types` depends on `@types/node` at version `*`. The catalog pins one version, and the top-level `overrides` repeats it as a literal so the install keeps a single copy. Writing `catalog:` in `overrides` installs a second copy, which is why the version appears twice.
