// Package typescript is the TypeScript brick: TypeScript on Bun, checked by
// oxlint, `bun fmt`, `bun check`, `bun test`, and markdownlint. Its plan
// moves an existing repo onto the Bun toolchain and keeps the project's own
// settings; its guard reports what the linters cannot see.
package typescript

import "github.com/xsyetopz/bedrock/internal/language"

// Typescript is the TypeScript language.
type Typescript struct{}

// New returns the TypeScript language.
func New() Typescript { return Typescript{} }

// Spec declares TypeScript's templates, markers, and tools.
func (Typescript) Spec() language.Spec {
	return language.Spec{
		ID:      "typescript",
		Block:   "typescript-bedrock",
		Aliases: []string{"typescript", "ts", nameBun},
		Markers: []string{"bun.lock", nameBunLockb, nameBunfig, namePackageJSON, nameTsconfig},
		Tools: []language.Tool{{
			Name:    nameBun,
			Install: "`curl -fsSL https://bun.sh/install | bash`, or `brew install oven-sh/bun/bun`",
		}},
		Sources: []string{
			"**/*.{js,jsx,mjs,cjs,ts,tsx,mts,cts}", "**/package.json", nameBunfig, "**/tsconfig.json",
		},
		Kinds: []string{"library", "cli", "plugin"},
	}
}
