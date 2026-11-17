import type { Language } from "../../core/language.ts";

/** Stub until the cpp port lands. */
export const cpp: Language = {
	id: "cpp",
	block: "swift-bedrock-cxx",
	aliases: ["cpp"],
	markers: [],
	tools: {},
	sources: [],
	kinds: [],
	variables: () => Promise.resolve({}),
	plan: () => Promise.resolve({ steps: [], notes: [], projectFiles: [] }),
	guard: () => Promise.resolve([]),
};
