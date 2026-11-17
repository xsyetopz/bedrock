import { afterEach, beforeEach, expect, spyOn, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { contextIn, textOf } from "./harness.ts";
import { convertTests } from "./subcommand.ts";

const XCTEST =
	"import XCTest\nfinal class A: XCTestCase {\n  func testB() {}\n}\n";

const logs: string[] = [];
const spies: { mockRestore: () => void }[] = [];

beforeEach(() => {
	logs.length = 0;
	spies.push(
		spyOn(console, "info").mockImplementation((line: string) => {
			logs.push(line);
		}),
		spyOn(console, "error").mockImplementation((line: string) => {
			logs.push(line);
		}),
	);
});

afterEach(() => {
	for (const spy of spies.splice(0)) {
		spy.mockRestore();
	}
});

test("converts every unit-test file when no path is given", async () => {
	const root = await tempRepo({
		"Tests/ATests/A.swift": XCTEST,
		"Tests/AppUITests/U.swift": XCTEST,
	});

	const status = await convertTests.run(contextIn(root), []);

	expect(status).toBe(0);
	expect(await textOf(root, "Tests/ATests/A.swift")).toContain("@Test");
	expect(await textOf(root, "Tests/AppUITests/U.swift")).toBe(XCTEST);
	expect(logs).toEqual(["Tests/ATests/A.swift: converted"]);
});

test("converts the given paths and reports a file without XCTest", async () => {
	const root = await tempRepo({
		"A.swift": XCTEST,
		"B.swift": "import Testing\n",
	});

	const status = await convertTests.run(contextIn(root), [
		"A.swift",
		"B.swift",
	]);

	expect(status).toBe(0);
	expect(logs).toEqual([
		"A.swift: converted",
		"B.swift: no XCTest import; left as is",
	]);
});

test("fails on a missing file", async () => {
	const root = await tempRepo();

	const status = await convertTests.run(contextIn(root), ["Nope.swift"]);

	expect(status).toBe(1);
	expect(logs).toEqual(["Nope.swift: no such file"]);
});

test("prints what conversion leaves for a person", async () => {
	const root = await tempRepo({
		"A.swift":
			'import XCTest\nfinal class A: XCTestCase {\n  func testB() throws { throw XCTSkip("x") }\n}\n',
	});

	await convertTests.run(contextIn(root), ["A.swift"]);

	expect(logs.some((line) => line.includes("XCTSkip"))).toBe(true);
});
