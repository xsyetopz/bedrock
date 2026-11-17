import { describe, expect, test } from "bun:test";

import { tempRepo } from "../core/testing.ts";
import { detect, parseNamed, select } from "./select.ts";

const ids = (selected: ReturnType<typeof select>): string[] =>
	selected.map((entry) => entry.language.id);

describe("parseNamed", () => {
	test("rejects an unknown language with the known names", () => {
		expect(() => parseNamed("cobol")).toThrow(/Unknown language cobol/u);
	});
});

describe("select", () => {
	test("merges configured and named languages in block order", () => {
		const config = { version: "0.2.0", languages: { jvm: { kind: "x" } } };
		const selected = select(config, [parseNamed("rust")]);
		expect(ids(selected)).toEqual(["rust", "jvm"]);
		expect(selected[1]?.options).toEqual({ kind: "x" });
	});

	test("adds cpp for --cxx", () => {
		expect(ids(select(undefined, [], { cxx: true }))).toContain("cpp");
	});

	test("--group joins the jvm options, which .bedrock.json keeps", () => {
		// Arrange
		const named = [parseNamed("jvm:kotlin-library")];

		// Act
		const [jvm] = select(undefined, named, { group: "org.acme.app" });

		// Assert
		expect(jvm?.options).toEqual({
			kind: "kotlin-library",
			group: "org.acme.app",
		});
	});

	test("--group without jvm in the run is an error", () => {
		// Arrange
		const named = [parseNamed("rust")];

		// Act
		const run = () => select(undefined, named, { group: "org.acme" });

		// Assert
		expect(run).toThrow(/name jvm too/u);
	});

	test("--group that is no Java package name is an error", () => {
		// Arrange
		const named = [parseNamed("jvm")];

		// Act
		const run = () => select(undefined, named, { group: "Org-Acme" });

		// Assert
		expect(run).toThrow(/no Java package name/u);
	});
});

describe("detect", () => {
	test("finds C++ by its sources in a Swift package", async () => {
		// Arrange
		const root = await tempRepo({
			"Package.swift": "// swift-tools-version:6.0\n",
			"Sources/Core/core.cpp": "int one() { return 1; }\n",
		});

		// Act
		const found = await detect(root);

		// Assert
		expect(found.map((language) => language.id)).toEqual(["swift", "cpp"]);
	});

	test("leaves C++ out of a Swift package without C sources", async () => {
		// Arrange
		const root = await tempRepo({
			"Package.swift": "// swift-tools-version:6.0\n",
			"Sources/App/main.swift": "print(1)\n",
		});

		// Act
		const found = await detect(root);

		// Assert
		expect(found.map((language) => language.id)).toEqual(["swift"]);
	});
});
