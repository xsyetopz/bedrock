import { expect, test } from "bun:test";

import { childOf, childrenOf, descendants, parseXml, textOf } from "./xml.ts";

test("reads nested elements, attributes, and decoded text", () => {
	// Arrange
	const source =
		'<?xml version="1.0"?><a x="1"><!-- c --><b>t &amp; &#65;&#x42;</b><c/></a>';

	// Act
	const root = parseXml(source);

	// Assert
	expect(root.attributes["x"]).toBe("1");
	expect(textOf(root, "b")).toBe("t & AB");
	expect(childrenOf(root).map((child) => child.name)).toEqual(["b", "c"]);
	expect(childOf(root, "c")?.children).toEqual([]);
});

test("finds descendants at any depth", () => {
	// Arrange
	const root = parseXml("<a><b><x>1</x></b><x>2</x></a>");

	// Act
	const found = descendants(root, "x").map((element) => element.text);

	// Assert
	expect(found).toEqual(["1", "2"]);
});

test("rejects a DOCTYPE", () => {
	// Arrange
	const source = '<!DOCTYPE a [<!ENTITY e "x">]><a/>';

	// Act
	const parse = () => parseXml(source);

	// Assert
	expect(parse).toThrow("DOCTYPE");
});
