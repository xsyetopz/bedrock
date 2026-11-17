const SUPPRESS = "swiftlint:disable";
const ENABLE = "swiftlint:enable";

/** The text of a line's `//` comment, where SwiftLint and swift-format read directives. */
const directive = (line: string): string => {
	const start = line.indexOf("//");
	return start === -1 ? "" : line.slice(start).replace(/^[/ ]+/u, "");
};

const rulesOf = (text: string): string[] => {
	const rules: string[] = [];
	for (const word of text.split(" ").filter((part) => part !== "")) {
		if (word.startsWith("//")) {
			break;
		}
		rules.push(word);
	}
	return rules;
};

const isReenabled = (
	directives: readonly string[],
	index: number,
	rules: readonly string[],
): boolean =>
	directives.slice(index + 1).some((later) => {
		if (!later.startsWith(ENABLE)) {
			return false;
		}
		const enabled = new Set(later.slice(ENABLE.length).split(" "));
		return rules.every((rule) => enabled.has(rule));
	});

/**
 * Lines that turn a SwiftLint rule off for the rest of the file, or swift-format
 * off for the whole file, each as `N: message`.
 */
export const suppressions = (text: string): string[] => {
	const directives = text.split("\n").map((line) => directive(line));
	const found: string[] = [];
	for (const [index, comment] of directives.entries()) {
		if (comment.startsWith("swift-format-ignore-file")) {
			found.push(
				`${index + 1}: swift-format-ignore-file; ignore the one declaration instead`,
			);
		}
		const tail = comment.slice(SUPPRESS.length);
		if (!comment.startsWith(SUPPRESS) || tail.startsWith(":")) {
			continue;
		}
		if (!isReenabled(directives, index, rulesOf(tail))) {
			found.push(
				`${index + 1}: swiftlint:disable without a matching swiftlint:enable; use swiftlint:disable:next`,
			);
		}
	}
	return found;
};

/** Whether a path sits in a `*UITests` or `*UITest` directory, where XCTest stays. */
export const isUITest = (path: string): boolean =>
	path
		.split("/")
		.slice(0, -1)
		.some((part) => part.endsWith("UITests") || part.endsWith("UITest"));

const IMPORT_XCTEST = /^\s*(?:@\w+\s+)*import\s+(?:\w+\s+)?XCTest\b/mu;

/** Whether the text imports XCTest, `@testable import XCTest` forms included. */
export const importsXCTest = (text: string): boolean =>
	IMPORT_XCTEST.test(text);
