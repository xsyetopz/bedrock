import { expect, test } from "bun:test";

import { gated, isIntegrationTest } from "./gating.ts";

test("adds the attribute after the leading doc lines", () => {
	// Arrange
	const source = "//! Tests.\n//! More.\nuse a::b;\n";

	// Act
	const result = gated(source);

	// Assert
	expect(result).toBe("//! Tests.\n//! More.\n#![cfg(test)]\n\nuse a::b;\n");
});

test("gates a file without doc lines too", () => {
	// Arrange
	const source = "#[test]\nfn t() {}\n";

	// Act
	const result = gated(source);

	// Assert
	expect(result).toContain("#![cfg(test)]\n\n#[test]");
});

test("leaves a file that has the attribute", () => {
	// Arrange
	const source = "//! Tests.\n#![cfg(test)]\n";

	// Act
	const result = gated(source);

	// Assert
	expect(result).toBeUndefined();
});

test("does not take the attribute in a comment or a string for the real one", () => {
	// Arrange
	const source = '// #![cfg(test)]\nconst S: &str = "#![cfg(test)]";\n';

	// Act
	const result = gated(source);

	// Assert
	expect(result).toContain("#![cfg(test)]\n\n// #![cfg(test)]");
});

test("keeps a doc line that has no newline", () => {
	// Arrange
	const source = "//! Only docs";

	// Act
	const result = gated(source);

	// Assert
	expect(result).toBe("//! Only docs\n#![cfg(test)]\n\n");
});

test("recognizes top-level files of tests/ and nothing deeper", () => {
	// Arrange
	const paths = [
		"tests/a.rs",
		"crates/x/tests/b.rs",
		"tests/util/mod.rs",
		"src/tests.rs"
	];

	// Act
	const results = paths.map((path) => isIntegrationTest(path));

	// Assert
	expect(results).toEqual([true, true, false, false]);
});
