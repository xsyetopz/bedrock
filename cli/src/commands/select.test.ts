import { describe, expect, test } from "bun:test";

import { tempRepo } from "../core/testing.ts";
import { detect, dropped, parseNamed, select } from "./select.ts";

const ids = (selected: ReturnType<typeof select>): string[] =>
	selected.map((entry) => entry.language.id);

describe("parseNamed", () => {
	test("rejects an unknown language", () => {
		expect(() => parseNamed("cobol")).toThrow();
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

	test("--group joins the jvm options, which .editorconfig keeps", () => {
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
		expect(run).toThrow();
	});

	test("--group that is no Java package name is an error", () => {
		// Arrange
		const named = [parseNamed("jvm")];

		// Act
		const run = () => select(undefined, named, { group: "Org-Acme" });

		// Assert
		expect(run).toThrow();
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

	test("finds C++ by a root CMakeLists.txt beside another language", async () => {
		// Arrange
		const root = await tempRepo({
			"Cargo.toml": "[package]\n",
			"CMakeLists.txt": "project(native C)\n",
		});

		// Act
		const found = await detect(root);

		// Assert
		expect(found.map((language) => language.id)).toEqual(["rust", "cpp"]);
	});

	test("finds C in a Make-built repo by its top-level sources", async () => {
		// Arrange
		const root = await tempRepo({
			Makefile: "all:\n",
			"py/obj.c": "int x;\n",
		});

		// Act
		const found = await detect(root);

		// Assert
		expect(found.map((language) => language.id)).toEqual(["cpp"]);
	});

	test("finds Haxe by a nested hxml build beside TypeScript", async () => {
		// Arrange
		const root = await tempRepo({
			"package.json": '{ "name": "game" }\n',
			"services/api/build.hxml": "-cp src\n-js out.js\n",
		});

		// Act
		const found = await detect(root);

		// Assert
		expect(found.map((language) => language.id)).toEqual([
			"typescript",
			"haxe",
		]);
	});

	test("finds Haxe by a deep source without a build file", async () => {
		// Arrange
		const root = await tempRepo({ "lib/src/game/Main.hx": "class Main {}\n" });

		// Act
		const found = await detect(root);

		// Assert
		expect(found.map((language) => language.id)).toEqual(["haxe"]);
	});
});

describe("dropped", () => {
	test("leaves out the selected languages and those with a marker", async () => {
		const root = await tempRepo({ "Cargo.toml": "[package]\n" });
		const selected = select(undefined, [parseNamed("go")]);
		const languages = await dropped(root, selected);
		expect(languages.map((language) => language.id)).toEqual([
			"typescript",
			"swift",
			"cpp",
			"jvm",
			"haxe",
		]);
	});

	test("counts a language bedrock's own files alone mark as dropped", async () => {
		// Arrange
		const root = await tempRepo({
			".swift-version": "6.3\n",
			".just/cpp.just": "",
			".just/haxe.just": "",
			".clang-format": "",
			gradlew: "",
			"Cargo.toml": "[package]\n",
		});
		const selected = select(undefined, [parseNamed("go")]);

		// Act
		const languages = await dropped(root, selected);

		// Assert
		expect(languages.map((language) => language.id)).toEqual([
			"typescript",
			"swift",
			"cpp",
			"jvm",
			"haxe",
		]);
	});
});
