import { tokens } from "./lexer.ts";

const GATE = "#![cfg(test)]";

const INTEGRATION_DOC =
	"//! Integration tests run against the public API, as another crate would.\n";

/** Whether `path` is a top-level integration test of a crate: `tests/<name>.rs`. */
export const isIntegrationTest = (path: string): boolean =>
	/(?:^|\/)tests\/[^/]+\.rs$/u.test(path);

/** Whether the file starts with `#![cfg(test)]` as code, not inside a comment or string. */
export const hasGate = (source: string): boolean =>
	tokens(source)
		.map((token) => (token.kind === "literal" ? "0" : token.text))
		.join("")
		.includes(GATE);

/**
 * The integration test with `#![cfg(test)]` after its leading `//!` lines, so
 * Clippy treats the whole file as test code. Clippy 1.100 extends
 * `allow-unwrap-in-tests` to integration tests, but `tests_outside_test_module`
 * still rejects each `#[test]` outside a `#[cfg(test)]` module. A file
 * without those lines gets one, since `missing_docs` checks each test file as
 * its own crate. Undefined when the file already has the attribute.
 */
export const gated = (source: string): string | undefined => {
	if (hasGate(source)) {
		return undefined;
	}
	const lines = source.split(/(?<=\n)/u);
	const docs = lines.findIndex((line) => !line.startsWith("//!"));
	const count = docs === -1 ? lines.length : docs;
	const head = lines.slice(0, count).join("");
	const tail = lines.slice(count).join("");
	const leading = head === "" ? INTEGRATION_DOC : `${head.trimEnd()}\n`;
	return `${leading}${GATE}\n\n${tail}`;
};
