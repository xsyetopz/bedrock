import { isWord, prefixWhile } from "./lexer.ts";

/** A class or extension body being rewritten, with the lifecycle members it has converted. */
export interface Container {
	bodyDepth: number;
	seenBody: boolean;
	hasInit: boolean;
	hasDeinit: boolean;
}

const isIndent = (c: string): boolean => c === " " || c === "\t";

const isIndentOnly = (text: string): boolean =>
	text.split("").every((c) => isIndent(c));

interface Lifecycle {
	readonly at: number;
	readonly name: string;
	readonly isSetUp: boolean;
	readonly rest: string;
}

/** Finds an `override func setUp…()` or `tearDown…()` declaration in the line. */
const lifecycleOf = (line: string): Lifecycle | undefined => {
	const marker = "override func ";
	const at = line.indexOf(marker);
	if (at === -1) {
		return undefined;
	}
	const rest = line.slice(at + marker.length);
	const name = prefixWhile(rest, isWord);
	const isSetUp = name === "setUp" || name === "setUpWithError";
	const isTearDown = name === "tearDown" || name === "tearDownWithError";
	const valid =
		(isSetUp || isTearDown) && rest.slice(name.length).startsWith("()");
	return valid ? { at, name, isSetUp, rest } : undefined;
};

/** deinit can neither throw nor be async, and takes no attributes. */
const canBeDeinit = (found: Lifecycle, line: string): boolean => {
	const effects = found.rest.slice(found.name.length + 2).split("{")[0] ?? "";
	return (
		!(effects.includes("async") || effects.includes("throws")) &&
		isIndentOnly(line.slice(0, found.at))
	);
};

/** `override func setUp…()` becomes `init()`, and `tearDown…()` becomes `deinit`. */
export const rewriteLifecycle = (
	line: string,
	container: Container
): string | undefined => {
	const found = lifecycleOf(line);
	if (found === undefined) {
		return undefined;
	}
	const { isSetUp } = found;
	if (isSetUp ? container.hasInit : container.hasDeinit) {
		return undefined;
	}
	if (!(isSetUp || canBeDeinit(found, line))) {
		return undefined;
	}
	container.hasInit ||= isSetUp;
	container.hasDeinit ||= !isSetUp;
	const end =
		found.at + "override func ".length + found.name.length + (isSetUp ? 0 : 2);
	return `${line.slice(0, found.at)}${isSetUp ? "init" : "deinit"}${line.slice(end)}`;
};

/** Whether the line only calls `super.setUp…()` or `super.tearDown…()`. */
export const isSuperCall = (line: string): boolean => {
	const calls = [
		"setUp",
		"setUpWithError",
		"tearDown",
		"tearDownWithError"
	].map((name) => `super.${name}()`);
	const trimmed = line.replaceAll(/^[; \t\r]+|[; \t\r]+$/gu, "");
	const parts = trimmed.split(/ +/u).filter((word) => word !== "");
	while (parts[0] === "try" || parts[0] === "await") {
		parts.shift();
	}
	return parts.length === 1 && calls.includes(parts[0] ?? "");
};
