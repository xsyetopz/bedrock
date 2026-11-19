import { expect, test } from "bun:test";

import { globMatcher } from "./glob.ts";

test.each([
	["*.kt", "A.kt", true],
	["*.kt", "app/src/A.kt", true],
	["*.kt", "A.kts", false],
	["src/*.kt", "src/A.kt", true],
	["src/*.kt", "src/sub/A.kt", false],
	["src/*.kt", "app/src/A.kt", false],
	["/src/*.kt", "src/A.kt", true],
	["src/**/*.kt", "src/A.kt", true],
	["src/**/*.kt", "src/a/b/A.kt", true],
	["gen/**", "gen/a/B.kt", true],
	["?.kt", "A.kt", true],
	["?.kt", "AB.kt", false],
	["[ab].kt", "b.kt", true],
	["[!ab].kt", "b.kt", false],
	["[!ab].kt", "c.kt", true],
	["*.{java,kt,kts}", "A.kts", true],
	["*.{java,kt}", "A.scala", false],
	["{src,test}/*.{java,kt}", "test/A.java", true],
	["*.{kt}", "A.{kt}", true],
	["*.{kt}", "A.kt", false],
	["Day{1..12}.kt", "Day7.kt", true],
	["Day{1..12}.kt", "Day12.kt", true],
	["Day{1..12}.kt", "Day13.kt", false],
	["Day{1..12}.kt", "Day0.kt", false],
	["v{-3..3}.txt", "v-2.txt", true],
	["v{3..-3}.txt", "v4.txt", false],
	[String.raw`a\*.kt`, "a*.kt", true],
	[String.raw`a\*.kt`, "ab.kt", false],
	["a.kt", "aXkt", false],
	["[a.kt", "[a.kt", true]
])("%s matches %s: %p", (glob, path, expected) => {
	expect(globMatcher(glob)(path)).toBe(expected);
});
