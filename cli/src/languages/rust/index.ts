import type { Language } from "../../core/language.ts";

/** Stub until the rust port lands. */
export const rust: Language = {
	id: "rust",
	block: "rust-bedrock",
	aliases: ["rust"],
	markers: [],
	tools: {},
	sources: [],
	kinds: [],
	variables: () => Promise.resolve({}),
	plan: () => Promise.resolve({ steps: [], notes: [], projectFiles: [] }),
	guard: () => Promise.resolve([]),
};
