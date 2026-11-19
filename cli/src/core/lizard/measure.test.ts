import { describe, expect, test } from "bun:test";

import { tempRepo } from "../testing.ts";
import { findLizard, measure, parseFiles, parseFunctions } from "./measure.ts";

const lizard = await findLizard();

describe("parseFunctions", () => {
	test("reads each function, with commas inside quoted fields", () => {
		// Arrange
		const csv =
			'4,2,27,2,5,"a@3-7@./src/A.kt","./src/A.kt","a","a x : Int , y : Int",3,7\n';

		// Act
		const functions = parseFunctions(csv);

		// Assert
		expect(functions).toEqual([
			{ path: "src/A.kt", line: 3, name: "a", nloc: 4, ccn: 2, params: 2 },
		]);
	});
});

describe("parseFiles", () => {
	test("reads each file's code lines from the File measure only", () => {
		// Arrange
		const xml = `<cppncss>
	<measure type="Function">
		<item name="a(...) at ./A.kt:3"><value>1</value><value>4</value></item>
	</measure>
	<measure type="File">
		<item name="./src/A&amp;B.kt">
			<value>1</value>
			<value>5</value>
			<value>2</value>
		</item>
	</measure>
</cppncss>`;

		// Act
		const files = parseFiles(xml);

		// Assert
		expect(files).toEqual([{ path: "src/A&B.kt", nloc: 5 }]);
	});
});

describe("measure", () => {
	test.skipIf(lizard === undefined)(
		"counts code lines without comments or blanks",
		async () => {
			// Arrange
			const root = await tempRepo({
				"src/A.kt":
					"// note\n\nfun a(x: Int, y: Int): Int {\n  if (x > y) { return x }\n  return y\n}\n",
			});

			// Act
			const measures = await measure(lizard ?? [], root, ["kotlin"]);

			// Assert
			expect(measures.files).toEqual([{ path: "src/A.kt", nloc: 4 }]);
			expect(measures.functions).toEqual([
				{ path: "src/A.kt", line: 3, name: "a", nloc: 4, ccn: 2, params: 2 },
			]);
		},
		120_000,
	);
});
