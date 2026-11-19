import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { hxmlFiles, hxmlLines, isTestHxml, readHxml } from "./hxml.ts";

describe("hxmlLines", () => {
	test("keeps the argument lines, without blanks and comments", () => {
		// Arrange
		const text = "# build\n-cp src\n\n  --main Main  \r\n#-js out.js\n";

		// Act
		const lines = hxmlLines(text);

		// Assert
		expect(lines).toEqual(["-cp src", "--main Main"]);
	});
});

describe("readHxml", () => {
	test.each([
		["-cp src\n--main Main\n-js out.js\n", true, false],
		["-cp src\n--main Main\n--interp\n", true, true],
		["-cp src\n--main Main\n-js out.js\n--cmd node out.js\n", true, true],
		["-cp src\n--main Main\n-lib utest\n", false, false]
	])("reads %j", async (text, target, runs) => {
		// Arrange
		const root = await tempRepo({ "build.hxml": text });

		// Act
		const hxml = await readHxml(root, "build.hxml");

		// Assert
		expect(hxml).toEqual({ target, runs });
	});

	test("adds the flags of an included hxml file", async () => {
		// Arrange
		const root = await tempRepo({
			"common.hxml": "-cp src\n--interp\n",
			"test.hxml": "common.hxml\n--main TestMain\n"
		});

		// Act
		const hxml = await readHxml(root, "test.hxml");

		// Assert
		expect(hxml).toEqual({ target: true, runs: true });
	});

	test("stops at an include cycle and a missing include", async () => {
		// Arrange
		const root = await tempRepo({
			"a.hxml": "b.hxml\nmissing.hxml\n-cp src\n",
			"b.hxml": "a.hxml\n"
		});

		// Act
		const hxml = await readHxml(root, "a.hxml");

		// Assert
		expect(hxml).toEqual({ target: false, runs: false });
	});
});

describe("hxmlFiles", () => {
	test("lists the hxml files outside the local haxelib repo", async () => {
		// Arrange
		const root = await tempRepo({
			"build.hxml": "",
			"services/api/build.hxml": "",
			".haxelib/utest/1,13,2/haxelib.hxml": ""
		});

		// Act
		const files = await hxmlFiles(root);

		// Assert
		expect(files).toEqual(["build.hxml", "services/api/build.hxml"]);
	});
});

describe("isTestHxml", () => {
	test.each([
		["test.hxml", true],
		["tests.hxml", true],
		["test-native.hxml", true],
		["sub/Test.hxml", true],
		["build.hxml", false],
		["testing.hxml", false],
		["latest.hxml", false]
	])("%s is a test build: %p", (path, expected) => {
		// Act
		const found = isTestHxml(path);

		// Assert
		expect(found).toBe(expected);
	});
});
