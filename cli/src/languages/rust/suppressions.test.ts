import { expect, test } from "bun:test";

import { suppressionProblems } from "./suppressions.ts";

test("flags #[allow] and #![allow], with their lines", () => {
	// Arrange
	const source = "#![allow(unused)]\n\n#[allow(clippy::all)]\nfn f() {}\n";

	// Act
	const found = suppressionProblems(source);

	// Assert
	expect(found.map((problem) => problem.line)).toEqual([1, 3]);
});

test("flags #[expect] without a reason, and accepts one with", () => {
	// Arrange
	const source =
		'#[expect(unused)]\nfn a() {}\n#[expect(unused, reason = "kept for the macro")]\nfn b() {}\n';

	// Act
	const found = suppressionProblems(source);

	// Assert
	expect(found).toHaveLength(1);
	expect(found[0]?.line).toBe(1);
});

test("ignores attributes in strings and comments", () => {
	// Arrange
	const source =
		'// #[allow(x)]\n/* #[allow(y)] */\nconst S: &str = "#[allow(z)]";\nconst R: &str = r#"#[allow(w)]"#;\n';

	// Act
	const found = suppressionProblems(source);

	// Assert
	expect(found).toEqual([]);
});

test("ignores other attributes, including a derive with allow in a path", () => {
	// Arrange
	const source =
		'#[derive(Debug)]\n#[cfg(test)]\n#[doc = "allow"]\nstruct S;\n';

	// Act
	const found = suppressionProblems(source);

	// Assert
	expect(found).toEqual([]);
});
