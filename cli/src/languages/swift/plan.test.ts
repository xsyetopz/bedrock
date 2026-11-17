import { describe, expect, test } from "bun:test";

import type { Step } from "../../core/steps.ts";
import { tempRepo } from "../../core/testing.ts";
import { adoptSwift, contextIn, textOf, written } from "./harness.ts";
import { swift } from "./index.ts";
import { planSwift } from "./plan.ts";

const XCTEST =
	"import XCTest\nfinal class A: XCTestCase {\n  func testB() {}\n}\n";

const writes = (steps: readonly Step[]): Map<string, string> =>
	new Map(
		steps.flatMap((step) =>
			step.kind === "write" ? [[step.path, step.content] as const] : [],
		),
	);

describe("planSwift", () => {
	test("adopts the Package.swift with the strict block and the 6.4 tools version", async () => {
		const root = await tempRepo({
			"Package.swift":
				'// swift-tools-version:5.9\nimport PackageDescription\nlet package = Package(name: "X", platforms: [.macOS(.v10_15)])\n',
		});

		const plan = await planSwift(contextIn(root));

		const manifest = written(plan.steps, "Package.swift");
		expect(manifest.startsWith("// swift-tools-version: 6.4")).toBe(true);
		expect(manifest).toContain(".macOS(.v12)");
		expect(manifest.trimEnd().endsWith("// <<< swift-bedrock")).toBe(true);
		expect(plan.projectFiles).toContain("Package.swift");
	});

	test("writes the cxx block only when cpp is selected", async () => {
		const root = await tempRepo({ "Package.swift": "let package = 1\n" });

		await adoptSwift(root, { cxx: true });

		expect(await textOf(root, "Package.swift")).toContain(
			"// >>> swift-bedrock-cxx",
		);
	});

	test("converts XCTest files and keeps them the project's", async () => {
		const root = await tempRepo({
			"Package.swift": "let package = 1\n",
			"Tests/ATests/A.swift": XCTEST,
		});

		const plan = await planSwift(contextIn(root));

		expect(writes(plan.steps).get("Tests/ATests/A.swift")).toContain(
			"import Testing",
		);
		expect(plan.projectFiles).toContain("Tests/ATests/A.swift");
	});

	test("writes the test lint config only where Tests exists", async () => {
		const withTests = await tempRepo({
			"Package.swift": "let package = 1\n",
			"Tests/ATests/A.swift": "import Testing\n",
		});
		const without = await tempRepo({ "Package.swift": "let package = 1\n" });

		const first = await planSwift(contextIn(withTests));
		const second = await planSwift(contextIn(without));

		expect(writes(first.steps).has("Tests/.swiftlint.yml")).toBe(true);
		expect(writes(second.steps).has("Tests/.swiftlint.yml")).toBe(false);
	});

	test("writes Config/Bedrock.xcconfig and wires an Xcode project", async () => {
		const root = await tempRepo({
			"A.xcodeproj/project.pbxproj": "// empty\n",
		});

		const plan = await planSwift(contextIn(root));

		expect(writes(plan.steps).get("Config/Bedrock.xcconfig")).toContain(
			"SWIFT_VERSION",
		);
	});

	test("the cxx xcconfig turns interop on and the compile cache off", async () => {
		const root = await tempRepo({
			"A.xcodeproj/project.pbxproj": "// empty\n",
		});

		await adoptSwift(root, { cxx: true });

		const text = await textOf(root, "Config/Bedrock.xcconfig");
		expect(text).toContain("BEDROCK_CXX_FLAGS = $(BEDROCK_CXX_INTEROP)");
		expect(text).toContain("COMPILATION_CACHE_ENABLE_CACHING = NO");
	});

	test("notes that there is nothing to adopt without a package or project", async () => {
		const root = await tempRepo();

		const plan = await planSwift(contextIn(root));

		expect(plan.notes.some((note) => note.includes("Package.swift"))).toBe(
			true,
		);
	});

	test("includes the legacy cleanup", async () => {
		const root = await tempRepo({
			"Package.swift": "let package = 1\n",
			".swiftformat": "",
		});

		const plan = await planSwift(contextIn(root));

		expect(plan.steps).toContainEqual({ kind: "remove", path: ".swiftformat" });
	});
});

describe("init", () => {
	test("writes the library kit and a manifest named for the directory", async () => {
		const root = await tempRepo();

		await adoptSwift(root, { kind: "library", init: true });

		const manifest = await textOf(root, "Package.swift");
		expect(manifest).toContain('.library(name: "');
		expect(manifest).toContain("// >>> swift-bedrock");
	});

	test("an executable adds its target", async () => {
		const root = await tempRepo();

		await adoptSwift(root, { kind: "executable", init: true });

		expect(await textOf(root, "Package.swift")).toContain(".executableTarget(");
	});

	test("an existing manifest is kept", async () => {
		const root = await tempRepo({ "Package.swift": "let package = 1\n" });

		const plan = await adoptSwift(root, { kind: "library", init: true });

		expect(await textOf(root, "Package.swift")).toContain("let package = 1");
		expect(plan.projectFiles.has("Package.swift")).toBe(true);
	});

	test("an app starts from the Xcode project that uses the xcconfig", async () => {
		const root = await tempRepo();

		await adoptSwift(root, { kind: "app", init: true });

		const projects = await Array.fromAsync(
			new Bun.Glob("*.xcodeproj/project.pbxproj").scan(root),
		);
		expect(projects).toHaveLength(1);
		expect(await textOf(root, projects.join(""))).toContain("Bedrock.xcconfig");
		expect(await textOf(root, "Config/Bedrock.xcconfig")).toContain(
			"SWIFT_VERSION",
		);
	});

	test("rejects a directory name that is not a type name", async () => {
		const root = await tempRepo();
		const context = {
			...contextIn(root, "library", true),
			root: `${root}/1bad`,
		};

		const outcome = await swift.plan(context).then(
			() => "planned",
			(error_: unknown) => error_,
		);

		expect(outcome).toBeInstanceOf(Error);
	});
});
