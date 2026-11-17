import { describe, expect, test } from "bun:test";

import {
	adoptedManifest,
	isTypeName,
	newManifest,
	TOOLS_VERSION,
	typeName,
	withMinimumMacOS,
	withToolsVersion,
} from "./manifest.ts";

describe("withToolsVersion", () => {
	test("replaces an existing tools version", () => {
		const text = "// swift-tools-version:5.9\nimport PackageDescription\n";

		expect(withToolsVersion(text)).toBe(
			`${TOOLS_VERSION}\nimport PackageDescription\n`,
		);
	});

	test("adds the line when none exists", () => {
		expect(withToolsVersion("import PackageDescription\n")).toBe(
			`${TOOLS_VERSION}\nimport PackageDescription\n`,
		);
	});
});

describe("withMinimumMacOS", () => {
	test.each([
		".macOS(.v10_15)",
		".macOS(.v11)",
		'.macOS("10.13")',
		'.macOS("11.0")',
		".macOS( .v10_9 )",
	])("raises %s to .v12", (old) => {
		expect(withMinimumMacOS(`platforms: [${old}]`)).toBe(
			"platforms: [.macOS(.v12)]",
		);
	});

	test.each([".macOS(.v12)", ".macOS(.v14)", ".iOS(.v13)"])(
		"keeps %s",
		(kept) => {
			expect(withMinimumMacOS(kept)).toBe(kept);
		},
	);
});

describe("adoptedManifest", () => {
	test("appends the blocks after the manifest", () => {
		const blocks = [{ name: "swift-bedrock", content: "let strict = true\n" }];

		const result = adoptedManifest(
			"// swift-tools-version: 5.5\nlet package = 1\n",
			blocks,
		);

		expect(result).toContain("// >>> swift-bedrock");
		expect(result.trimEnd().endsWith("// <<< swift-bedrock")).toBe(true);
		expect(result.startsWith(TOOLS_VERSION)).toBe(true);
	});

	test("is idempotent", () => {
		const blocks = [{ name: "swift-bedrock", content: "let strict = true\n" }];
		const once = adoptedManifest("let package = 1\n", blocks);

		expect(adoptedManifest(once, blocks)).toBe(once);
	});
});

describe("typeName", () => {
	test.each([
		["my-tool", "MyTool"],
		["foo_bar baz", "FooBarBaz"],
		["Plain", "Plain"],
	])("turns %s into %s", (raw, expected) => {
		expect(typeName(raw)).toBe(expected);
	});

	test.each([
		["Valid1", true],
		["1Bad", false],
		["", false],
		["Bad-Name", false],
		["Café", false],
	])("isTypeName(%s) is %s", (name, expected) => {
		expect(isTypeName(name)).toBe(expected);
	});
});

describe("newManifest", () => {
	test("declares a library and its tests", () => {
		const text = newManifest("Lib", new Set(["library"]), false);

		expect(text).toContain('.library(name: "Lib", targets: ["Lib"])');
		expect(text).toContain('.testTarget(name: "LibTests"');
		expect(text).not.toContain("Cxx");
	});

	test("adds the executable and its target", () => {
		const text = newManifest("Tool", new Set(["executable"]), false);

		expect(text).toContain('.executable(name: "tool", targets: ["ToolCLI"])');
		expect(text).toContain('.executableTarget(name: "ToolCLI"');
	});

	test("adds the C++ target the library depends on", () => {
		const text = newManifest("Lib", new Set(["library"]), true);

		expect(text).toContain('.target(name: "Lib", dependencies: ["LibCxx"])');
		expect(text).toContain('.target(name: "LibCxx")');
	});
});
