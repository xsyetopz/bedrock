import { expect, test } from "bun:test";

import { splitSources, type Target } from "./targets.ts";

const target = (
	id: string,
	sources: string[],
	extra: Partial<Target> = {},
): Target => ({
	id,
	sources,
	deps: [],
	test: false,
	installed: false,
	...extra,
});

test("a test target's sources are test-only", () => {
	// Arrange
	const targets = [
		target("lib", ["lib.c"], { installed: true }),
		target("lib_test", ["lib_test.c"], { deps: ["lib"], test: true }),
	];

	// Act
	const split = splitSources(targets);

	// Assert
	expect([...split.test]).toEqual(["lib_test.c"]);
});

test("a helper only test targets link is test-only", () => {
	// Arrange
	const targets = [
		target("fixture", ["fixture.c"]),
		target("a_test", ["a_test.c"], { deps: ["fixture"], test: true }),
		target("b_test", ["b_test.c"], { deps: ["fixture"], test: true }),
	];

	// Act
	const split = splitSources(targets);

	// Assert
	expect([...split.test].toSorted()).toEqual([
		"a_test.c",
		"b_test.c",
		"fixture.c",
	]);
});

test("an installed library only tests link stays product code", () => {
	// Arrange
	const targets = [
		target("api", ["api.c"], { installed: true }),
		target("api_test", ["api_test.c"], { deps: ["api"], test: true }),
	];

	// Act
	const split = splitSources(targets);

	// Assert
	expect(split.other.has("api.c")).toBe(true);
	expect(split.test.has("api.c")).toBe(false);
});

test("a source a product target also compiles stays product code", () => {
	// Arrange
	const targets = [
		target("app", ["main.c", "shared.c"]),
		target("app_test", ["app_test.c", "shared.c"], { test: true }),
	];

	// Act
	const split = splitSources(targets);

	// Assert
	expect([...split.test]).toEqual(["app_test.c"]);
});

test("a helper a product target also links stays product code", () => {
	// Arrange
	const targets = [
		target("util", ["util.c"]),
		target("app", ["main.c"], { deps: ["util"] }),
		target("util_test", ["util_test.c"], { deps: ["util"], test: true }),
	];

	// Act
	const split = splitSources(targets);

	// Assert
	expect([...split.test]).toEqual(["util_test.c"]);
});

test("a dependency cycle ends without marking the cycle test-only", () => {
	// Arrange
	const targets = [
		target("a", ["a.c"], { deps: ["b"] }),
		target("b", ["b.c"], { deps: ["a"] }),
	];

	// Act
	const split = splitSources(targets);

	// Assert
	expect([...split.test]).toEqual([]);
});
