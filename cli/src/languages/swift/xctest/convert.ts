import { rewriteAssertions } from "./assertions.ts";
import {
	ahead,
	behind,
	identifiers,
	type Line,
	parseLines,
	prefixWhile,
	scalars,
} from "./lexer.ts";
import { restructure } from "./structure.ts";

/** A construct left for a person, with its 1-based line. */
interface Leftover {
	readonly line: number;
	readonly message: string;
	readonly needsXCTest: boolean;
}

const CONFIRMATION = "use confirmation(...)";
const SKIP = "use the .enabled(if:) or .disabled trait";

/** XCTest constructs with no direct Swift Testing form, by identifier. */
const MANUAL: ReadonlyMap<string, { label: string; advice: string }> = new Map(
	[
		["XCTSkip", "XCTSkip", SKIP],
		["XCTSkipIf", "XCTSkipIf", SKIP],
		["XCTSkipUnless", "XCTSkipUnless", SKIP],
		["XCTestExpectation", "XCTestExpectation", CONFIRMATION],
		["expectation", "expectation(description:)", CONFIRMATION],
		["wait", "wait(for:)", CONFIRMATION],
		["waitForExpectations", "waitForExpectations", CONFIRMATION],
		["fulfill", "fulfill()", CONFIRMATION],
		["measure", "measure", "time the work with ContinuousClock by hand"],
		["XCUIApplication", "XCUIApplication", "keep UI tests in an XCTest target"],
		["addTeardownBlock", "addTeardownBlock", "use defer or deinit"],
		[
			"continueAfterFailure",
			"continueAfterFailure",
			"use #require to stop at the first failure",
		],
		[
			"XCTAssertThrowsError",
			"XCTAssertThrowsError with an error handler",
			"use #expect(throws:) and inspect the error",
		],
	].map(([name = "", label = "", advice = ""]) => [name, { label, advice }]),
);

const CALL_ONLY = new Set(["expectation", "wait", "measure", "fulfill"]);

/** Whether an identifier of the call-only names really is the XCTest call. */
const isXCTestCall = (
	name: string,
	following: string,
	bare: boolean,
): boolean => {
	const isCall =
		following.startsWith(name === "wait" ? "(for:" : "(") ||
		(name === "measure" && following.startsWith("{"));
	return isCall && bare === (name !== "fulfill");
};

const classify = (
	name: string,
	previous: string,
	entry: { label: string; advice: string } | undefined,
): { message: string; needsXCTest: boolean } | undefined => {
	if (entry !== undefined) {
		return {
			message: `${entry.label} has no direct Swift Testing form; ${entry.advice}`,
			needsXCTest: true,
		};
	}
	if (name.startsWith("XCT") && previous !== "import") {
		return {
			message: `${name} could not be converted; left as is`,
			needsXCTest: true,
		};
	}
	if (
		previous === "func" &&
		(name.startsWith("setUp") || name.startsWith("tearDown"))
	) {
		return {
			message: `${name} left as is; init and deinit appear once, and deinit cannot throw or be async`,
			needsXCTest: false,
		};
	}
	return undefined;
};

const isNoted = (
	found: readonly Leftover[],
	line: number,
	message: string,
): boolean =>
	found.some((other) => other.line === line && other.message === message);

/** Constructs still left for a person, found in the converted text. */
const leftovers = (source: string): Leftover[] => {
	const b = scalars(source);
	const found: Leftover[] = [];
	let line = 1;
	let position = 0;
	const tokens = identifiers(b);
	for (const [k, token] of tokens.entries()) {
		for (let at = position; at < token.start; at += 1) {
			line += b[at] === "\n" ? 1 : 0;
		}
		position = token.start;
		const { name } = token;
		const previous = tokens[k - 1]?.name ?? "";
		const bare = behind(b, token.start) !== "." || previous === "self";
		const skipped =
			CALL_ONLY.has(name) && !isXCTestCall(name, ahead(b, token.end), bare);
		const hit = classify(
			name,
			previous,
			skipped ? undefined : MANUAL.get(name),
		);
		if (hit !== undefined && !isNoted(found, line, hit.message)) {
			found.push({ line, ...hit });
		}
	}
	return found;
};

const importsModule = (line: Line, module: string): boolean => {
	const code = line.code ? (line.text.split("//")[0] ?? "") : "";
	const parts = code.split(/[ \t\r]+/u).filter((part) => part !== "");
	return (
		parts.length < 4 &&
		parts.at(-2) === "import" &&
		parts.at(-1) === module &&
		!parts.includes("@testable")
	);
};

const rewriteImports = (source: string, keepXCTest: boolean): string => {
	const lines = parseLines(source);
	let hasTesting = lines.some((line) => importsModule(line, "Testing"));
	const out: string[] = [];
	for (const line of lines) {
		if (!importsModule(line, "XCTest")) {
			out.push(line.text);
			continue;
		}
		if (!hasTesting) {
			hasTesting = true;
			const indent = prefixWhile(line.text, (c) => c === " " || c === "\t");
			out.push(
				`${indent}import Testing${line.text.endsWith("\r") ? "\r" : ""}`,
			);
		}
		if (keepXCTest) {
			out.push(line.text);
		}
	}
	return out.join("\n");
};

/** The result of converting one file. */
export interface Converted {
	readonly text: string;
	/** What is left for a person, each as `line N: message`. */
	readonly notes: readonly string[];
}

/**
 * Rewrites an XCTest file into Swift Testing. Returns undefined when `source`
 * does not import XCTest.
 */
export const convertXCTest = (source: string): Converted | undefined => {
	if (!parseLines(source).some((line) => importsModule(line, "XCTest"))) {
		return undefined;
	}
	const restructured = restructure(rewriteAssertions(source));
	const keep = leftovers(restructured).some((entry) => entry.needsXCTest);
	const text = rewriteImports(restructured, keep);
	const notes = leftovers(text).map((entry): [number, string] => [
		entry.line,
		entry.message,
	]);
	const kept = parseLines(text).findIndex((line) =>
		importsModule(line, "XCTest"),
	);
	if (keep && kept !== -1) {
		notes.push([kept + 1, "import XCTest kept for the constructs noted here"]);
	}
	return {
		text,
		notes: notes
			.toSorted((left, right) => left[0] - right[0])
			.map(([line, message]) => `line ${line}: ${message}`),
	};
};

/** Whether the file imports XCTest. */
export const importsXCTest = (source: string): boolean =>
	parseLines(source).some((line) => importsModule(line, "XCTest"));
