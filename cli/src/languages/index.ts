import type { Language } from "../core/language.ts";
import { cpp } from "./cpp/index.ts";
import { dotnet } from "./dotnet/index.ts";
import { go } from "./go/index.ts";
import { haxe } from "./haxe/index.ts";
import { jvm } from "./jvm/index.ts";
import { rust } from "./rust/index.ts";
import { swift } from "./swift/index.ts";
import { typescript } from "./typescript/index.ts";

/** Every language, in the order their blocks sit in shared files. */
export const LANGUAGES: readonly Language[] = [
	typescript,
	rust,
	swift,
	cpp,
	jvm,
	go,
	haxe,
	dotnet,
];

/** The language a command-line name means, or undefined. */
export const findLanguage = (name: string): Language | undefined => {
	const wanted = name.toLowerCase();
	return LANGUAGES.find(
		(language) => language.id === wanted || language.aliases.includes(wanted),
	);
};
