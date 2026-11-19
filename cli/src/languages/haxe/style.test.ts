import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { checkstyleErrors, sourceRoots } from "./style.ts";

describe("checkstyleErrors", () => {
	test.each([
		["Total Issues: 3 (Errors: 1, Warnings: 2, Infos: 0)\n", 1],
		["Total Issues: 300 (Errors: 256, Warnings: 44, Infos: 0)\n", 256],
		["No issues found.\n", 0],
		["Error: Unknown command\n", undefined]
	])("reads %j", (report, expected) => {
		// Act
		const errors = checkstyleErrors(report);

		// Assert
		expect(errors).toBe(expected);
	});
});

describe("sourceRoots", () => {
	test("takes the top of each Haxe source, outside the local haxelib repo", async () => {
		// Arrange
		const root = await tempRepo({
			"src/app/Main.hx": "",
			"src/app/Util.hx": "",
			"test/TestMain.hx": "",
			"Build.hx": "",
			".haxelib/utest/1,13,2/src/utest/Test.hx": "",
			"README.md": ""
		});

		// Act
		const roots = await sourceRoots(root);

		// Assert
		expect(roots).toEqual(["Build.hx", "src", "test"]);
	});
});
