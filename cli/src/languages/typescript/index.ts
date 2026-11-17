import type { Language } from "../../core/language.ts";

/** Stub until the typescript port lands. */
export const typescript: Language = {
	id: "typescript",
	block: "typescript-bedrock",
	aliases: ["typescript"],
	markers: [],
	tools: {},
	sources: [],
	kinds: [],
	variables: () => Promise.resolve({}),
	plan: () => Promise.resolve({ steps: [], notes: [], projectFiles: [] }),
	guard: () => Promise.resolve([]),
};
