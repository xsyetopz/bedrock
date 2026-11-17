import { describe, expect, test } from "bun:test";

import { contextFor } from "../../core/plan.ts";
import { tempRepo } from "../../core/testing.ts";
import { guardSwift, manifestFindings } from "./guard.ts";
import { swift } from "./index.ts";

const contextIn = (root: string) =>
	contextFor(root, [{ language: swift, options: {} }], swift, {}, false);

const STRICT_PACKAGE = {
	toolsVersion: { _version: "6.4.0" },
	swiftLanguageVersions: ["6"],
	targets: [],
};

const XCTEST =
	"import XCTest\nfinal class A: XCTestCase {\n  func testB() {}\n}\n";

describe("manifestFindings", () => {
	test("accepts a strict package", () => {
		expect(manifestFindings(STRICT_PACKAGE)).toEqual([]);
	});

	test("flags an old tools version", () => {
		const found = manifestFindings({
			...STRICT_PACKAGE,
			toolsVersion: { _version: "5.9.0" },
		});

		expect(found).toEqual([
			"Package.swift: swift-tools-version is 5.9.0; use 6.4",
		]);
	});

	test("flags language modes other than 6", () => {
		const found = manifestFindings({
			...STRICT_PACKAGE,
			swiftLanguageVersions: ["5", "6"],
		});

		expect(found).toEqual([
			"Package.swift: the language modes are [5, 6]; use [.v6] alone",
		]);
	});

	test("flags unsafeFlags on a target", () => {
		const found = manifestFindings({
			...STRICT_PACKAGE,
			targets: [
				{
					name: "Lib",
					type: "regular",
					settings: [{ kind: { unsafeFlags: { _0: ["-x"] } } }],
				},
			],
		});

		expect(found.some((finding) => finding.includes("unsafeFlags"))).toBe(true);
	});
});

describe("guardSwift", () => {
	test("flags XCTest outside UI tests", async () => {
		const root = await tempRepo({ "Tests/ATests/A.swift": XCTEST });

		const found = await guardSwift(contextIn(root), false);

		expect(found).toContain(
			"Tests/ATests/A.swift: imports XCTest outside a *UITests target; use Swift Testing (just lint-fix converts it)",
		);
	});

	test("leaves XCTest in a UI test target", async () => {
		const root = await tempRepo({ "Tests/AppUITests/A.swift": XCTEST });

		expect(await guardSwift(contextIn(root), false)).toEqual([]);
	});

	test("converts XCTest with fix", async () => {
		const root = await tempRepo({ "Tests/ATests/A.swift": XCTEST });

		const found = await guardSwift(contextIn(root), true);

		const text = await Bun.file(`${root}/Tests/ATests/A.swift`).text();
		expect(found).toEqual([]);
		expect(text).toContain("import Testing");
		expect(text).toContain("@Test");
	});

	test("reports what conversion leaves", async () => {
		const root = await tempRepo({
			"Tests/ATests/A.swift":
				'import XCTest\nfinal class A: XCTestCase {\n  func testB() throws { throw XCTSkip("x") }\n}\n',
		});

		const found = await guardSwift(contextIn(root), true);

		expect(found.some((finding) => finding.includes("XCTSkip"))).toBe(true);
	});

	test("reports file-wide suppressions with the line", async () => {
		const root = await tempRepo({
			"Sources/A.swift": "// swiftlint:disable all\nlet a = 1\n",
		});

		const found = await guardSwift(contextIn(root), false);

		expect(found).toEqual([
			"Sources/A.swift:1: swiftlint:disable without a matching swiftlint:enable; use swiftlint:disable:next",
		]);
	});

	test.each([
		["Cartfile", "Carthage"],
		["Podfile", "CocoaPods"],
		[".swiftformat", "swift-format"],
	])("flags the foreign file %s", async (path, tool) => {
		const root = await tempRepo({ [path]: "" });

		const found = await guardSwift(contextIn(root), false);

		expect(found).toHaveLength(1);
		expect(found[0]?.startsWith(`${path}: `)).toBe(true);
		expect(found[0]).toContain(tool);
	});

	test("flags an Xcode project that skips the xcconfig and overrides settings", async () => {
		const root = await tempRepo({
			"A.xcodeproj/project.pbxproj": "SWIFT_VERSION = 5.0;\n",
		});

		const found = await guardSwift(contextIn(root), false);

		expect(found).toHaveLength(2);
		expect(found[0]).toContain("Config/Bedrock.xcconfig");
		expect(found[1]).toContain("keeps the target out of Swift 6 mode");
	});
});
