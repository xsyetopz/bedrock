import { expect } from "bun:test";

import { convertXCTest } from "./convert.ts";

export const fixture = (name: string): Promise<string> =>
	Bun.file(new URL(`fixtures/${name}`, import.meta.url)).text();

export const converted = (source: string) => {
	const result = convertXCTest(source);
	if (result === undefined) {
		throw new Error("expected the source to import XCTest");
	}
	return result;
};

/** Converts a statement and returns the statement that replaced it. */
export const statement = (code: string): string => {
	const { text } = converted(`import XCTest\n${code}\n`);
	return text.slice("import Testing\n".length, -1);
};

/** Converts members of an XCTestCase and returns the converted class body. */
export const members = (code: string) => {
	const result = converted(
		`import XCTest\nfinal class T: XCTestCase {\n${code}\n}\n`
	);
	const prefix = "import Testing\nfinal class T {\n";
	expect(result.text.startsWith(prefix)).toBe(true);
	return { body: result.text.slice(prefix.length, -3), notes: result.notes };
};
