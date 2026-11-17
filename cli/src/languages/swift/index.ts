import type { Language } from "../../core/language.ts";

/** Stub until the swift port lands. */
export const swift: Language = {
	id: "swift",
	block: "swift-bedrock",
	aliases: ["swift"],
	markers: [],
	tools: {},
	sources: [],
	kinds: [],
	variables: () => Promise.resolve({}),
	plan: () => Promise.resolve({ steps: [], notes: [], projectFiles: [] }),
	guard: () => Promise.resolve([]),
};
