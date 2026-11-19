import { expect, test } from "bun:test";

import {
	findSection,
	keyOf,
	parseSections,
	removeSections,
	renderSections,
	type Section,
	tidy,
	withKey
} from "./toml.ts";

const sectionOf = (text: string, name: string): Section => {
	const found = findSection(parseSections(text), name);
	if (found === undefined) {
		throw new Error(`no [${name}]`);
	}
	return found;
};

const SAMPLE = `# top
name = "x"

# About deps.
[dependencies]
serde = { version = "1", features = [
  "derive", # kept
] }
note = """
[not-a-table]
"""

[lints.rust]
unsafe_code = "deny"
`;

test("renders what it parsed, byte for byte", () => {
	// Arrange
	const text = SAMPLE;

	// Act
	const rendered = renderSections(parseSections(text));

	// Assert
	expect(rendered).toBe(text);
});

test("keeps multi-line values and strings inside one entry", () => {
	// Arrange
	const sections = parseSections(SAMPLE);

	// Act
	const names = sections.map((section) => section.name);
	const dependencies = findSection(sections, "dependencies");

	// Assert
	expect(names).toEqual([undefined, "dependencies", "lints.rust"]);
	expect(dependencies?.units.map((unit) => keyOf(unit))).toEqual([
		"serde",
		"note"
	]);
});

test("attaches the comment above a header to that header", () => {
	// Arrange
	const sections = parseSections(SAMPLE);

	// Act
	const dependencies = findSection(sections, "dependencies");

	// Assert
	expect(dependencies?.leading).toEqual(["# About deps.\n"]);
});

test("reads dotted and quoted keys", () => {
	// Arrange
	const units = ["edition.workspace = true\n", '"a b" . c = 1\n', "# x = 1\n"];

	// Act
	const keys = units.map((unit) => keyOf(unit));

	// Assert
	expect(keys).toEqual(["edition.workspace", "a b.c", undefined]);
});

test("replaces a key in place and appends a new one after the last entry", () => {
	// Arrange
	const section = sectionOf("[a]\nx = 1\ny = 2\n", "a");

	// Act
	const replaced = withKey(section, "x", "9");
	const appended = withKey(section, "z", "3");

	// Assert
	expect(replaced.units).toEqual(["x = 9\n", "y = 2\n"]);
	expect(appended.units).toEqual(["x = 1\n", "y = 2\n", "z = 3\n"]);
});

test("removes sections by name and says where the first one sat", () => {
	// Arrange
	const sections = parseSections("[a]\n[lints]\nw = 1\n[b]\n");

	// Act
	const result = removeSections(sections, (name) => name === "lints");

	// Assert
	expect(result.at).toBe(2);
	expect(renderSections(result.sections)).toBe("[a]\n[b]\n");
});

test("tidy leaves one blank line between sections and none at the end", () => {
	// Arrange
	const sections = parseSections("[a]\nx = 1\n\n\n\n[b]\ny = 2\n\n\n");

	// Act
	const text = renderSections(tidy(sections));

	// Assert
	expect(text).toBe("[a]\nx = 1\n\n[b]\ny = 2\n");
});
