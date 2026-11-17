import { expect, test } from "bun:test";

import { tokens } from "./lexer.ts";

const texts = (source: string): string[] =>
	tokens(source).map((token) => token.text);

test("comments, including nested block comments, produce no tokens", () => {
	// Arrange
	const source = "a // mod x;\n/* /* mod y; */ still comment */ b";

	// Act
	const found = texts(source);

	// Assert
	expect(found).toEqual(["a", "b"]);
});

test("strings, raw strings, and byte strings stay whole", () => {
	// Arrange
	const source = String.raw`x "a\"b" r#"c"d"# br##"e"#f"## b"g" c"h" y`;

	// Act
	const found = texts(source);

	// Assert
	expect(found).toEqual([
		"x",
		String.raw`"a\"b"`,
		'r#"c"d"#',
		'br##"e"#f"##',
		'b"g"',
		'c"h"',
		"y",
	]);
});

test("char literals are literals and lifetimes are identifiers", () => {
	// Arrange
	const source = String.raw`'a' '\'' 'é' b'x' &'static str`;

	// Act
	const kinds = tokens(source).map((token) => token.kind);

	// Assert
	expect(kinds).toEqual([
		"literal",
		"literal",
		"literal",
		"literal",
		"punct",
		"ident",
		"ident",
	]);
});

test("lines are counted across multi-line literals", () => {
	// Arrange
	const source = '"a\nb"\nmod';

	// Act
	const last = tokens(source).at(-1);

	// Assert
	expect(last?.line).toBe(3);
});

test("raw identifiers are one token", () => {
	// Arrange / Act
	const found = texts("r#mod r#type");

	// Assert
	expect(found).toEqual(["r#mod", "r#type"]);
});

test("a raw string ends at its own number of hashes", () => {
	// Arrange
	const source = 'r##"a"# mod x; "## mod y;';

	// Act
	const found = texts(source);

	// Assert
	expect(found).toEqual(['r##"a"# mod x; "##', "mod", "y", ";"]);
});
