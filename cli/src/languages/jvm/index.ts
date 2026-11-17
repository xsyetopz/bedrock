import type { Language } from "../../core/language.ts";

/** Stub until the jvm port lands. */
export const jvm: Language = {
	id: "jvm",
	block: "jvm-bedrock",
	aliases: ["jvm"],
	markers: [],
	tools: {},
	sources: [],
	kinds: [],
	variables: () => Promise.resolve({}),
	plan: () => Promise.resolve({ steps: [], notes: [], projectFiles: [] }),
	guard: () => Promise.resolve([]),
};
