import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { cppTestFiles } from "./test-files.ts";

test("picks the files in a SwiftPM package's Tests directory", async () => {
	// Arrange
	const files = {
		"Package.swift": "// swift-tools-version: 6.4\n",
		"Sources/KitCxx/Kit.cpp": "int kit() { return 0; }\n",
		"Tests/KitTests/Helper.cpp": "int helper() { return 0; }\n",
		"vendor/Tests/Other.cpp": "int other() { return 0; }\n",
	};
	const root = await tempRepo(files);

	// Act
	const picked = await cppTestFiles(root, Object.keys(files).slice(1));

	// Assert
	expect([...picked]).toEqual(["Tests/KitTests/Helper.cpp"]);
});
