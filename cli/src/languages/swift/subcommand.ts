import { join } from "node:path";

import { readText } from "../../core/fs.ts";
import type { Subcommand } from "../../core/language.ts";
import { xctestFiles } from "./conversion.ts";
import { convertXCTest } from "./xctest/convert.ts";

/**
 * `bedrock swift convert-tests [<path>...]`: rewrites XCTest unit tests to Swift
 * Testing in place, the given files or every one outside `*UITests` directories,
 * and prints what is left for a person.
 */
export const convertTests: Subcommand = {
	summary:
		"Convert XCTest unit tests to Swift Testing: convert-tests [<path>...]",
	run: async (context, args) => {
		const paths = args.length > 0 ? args : await xctestFiles(context.root);
		const results = await Promise.all(
			paths.map(async (path) => {
				const text = await readText(join(context.root, path));
				const converted = text === undefined ? undefined : convertXCTest(text);
				if (converted !== undefined) {
					await Bun.write(join(context.root, path), converted.text);
				}
				return { path, converted, missing: text === undefined };
			}),
		);
		let status = 0;
		for (const { path, converted, missing } of results) {
			if (missing) {
				console.error(`${path}: no such file`);
				status = 1;
			} else if (converted === undefined) {
				console.info(`${path}: no XCTest import; left as is`);
			} else {
				console.info(`${path}: converted`);
				for (const note of converted.notes) {
					console.error(`${path}: ${note}`);
				}
			}
		}
		return status;
	},
};
