import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { adoptSwift, commitAll, contextIn } from "../swift/harness.ts";
import { asArray, asRecord, asString, asStrings } from "../swift/json.ts";
import { compileCommands, swiftEntries } from "./compile-commands.ts";

const INFO = {
	paths: { runtimeResourcePath: "/tc/usr/lib/swift" }
};

const describeOf = (sources: string[]) => ({
	cxx_language_standard: "c++20",
	targets: [
		{ module_type: "ClangTarget", path: "Sources/Lib", sources },
		{ module_type: "SwiftTarget", path: "Sources/App", sources: ["a.swift"] }
	]
});

describe("swiftEntries", () => {
	test.each([
		["a.c", "clang", "-std=c17"],
		["a.m", "clang", "-fobjc-arc"],
		["a.mm", "clang++", "-std=c++20"],
		["a.cpp", "clang++", "-std=c++20"]
	])("compiles %s with %s %s", (source, compiler, flag) => {
		const [entry] = swiftEntries("/r", {
			describe: describeOf([source]),
			info: INFO,
			sdk: "/sdk"
		});

		expect(entry?.arguments[0]).toBe(compiler);
		expect(entry?.arguments).toContain(flag);
		expect(entry?.file).toBe(`/r/Sources/Lib/${source}`);
		expect(entry?.directory).toBe("/r");
	});

	test("passes the sdk, toolchain include, deployment target, and target includes", () => {
		const [entry] = swiftEntries("/r", {
			describe: describeOf(["a.cpp"]),
			info: INFO,
			sdk: "/sdk"
		});

		expect(entry?.arguments).toEqual([
			"clang++",
			"-std=c++20",
			"-isysroot",
			"/sdk",
			"-isystem",
			"/tc/usr/include",
			"-mmacosx-version-min=12.0",
			"-I",
			"/r/Sources/Lib/include",
			"-c",
			"/r/Sources/Lib/a.cpp"
		]);
	});

	test("omits the sysroot when none is known and uses the package's standard", () => {
		const [entry] = swiftEntries("/r", {
			describe: { ...describeOf(["a.cpp"]), cxx_language_standard: "gnu++17" },
			info: INFO,
			sdk: ""
		});

		expect(entry?.arguments).not.toContain("-isysroot");
		expect(entry?.arguments).toContain("-std=gnu++17");
	});

	test("skips Swift targets", () => {
		const entries = swiftEntries("/r", {
			describe: describeOf([]),
			info: INFO,
			sdk: ""
		});

		expect(entries).toEqual([]);
	});
});

test.skipIf(Bun.which("swift") === null)(
	"writes compile_commands.json for the Clang targets of a package",
	async () => {
		// Arrange
		const root = await tempRepo();
		await adoptSwift(root, { kind: "library", init: true, cxx: true });
		await commitAll(root);

		// Act
		const status = await compileCommands.run(contextIn(root), []);

		// Assert
		const entries = asArray(
			JSON.parse(await Bun.file(`${root}/compile_commands.json`).text())
		).map((entry) => asRecord(entry));
		expect(status).toBe(0);
		expect(entries).toHaveLength(1);
		expect(asString(entries[0]?.["file"])).toEndWith("Cxx.cpp");
		expect(asStrings(entries[0]?.["arguments"])[0]).toBe("clang++");
	},
	120_000
);
