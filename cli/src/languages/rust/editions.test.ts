import { expect, test } from "bun:test";
import { join } from "node:path";

import { tempRepo } from "../../core/testing.ts";
import { migrateEditions, raisedEdition } from "./editions.ts";

const textOf = (root: string, path: string): Promise<string> =>
	Bun.file(join(root, path)).text();

const hasCargo = Bun.which("cargo") !== null;

test("raisedEdition raises each old edition one step and keeps 2024", () => {
	// Arrange
	const text =
		'[package]\nedition = "2018"\n\n[workspace.package]\nedition="2021"\n\n[lib]\nedition = "2024"\n';

	// Act
	const raised = raisedEdition(text);

	// Assert
	expect(raised).toBe(
		'[package]\nedition = "2021"\n\n[workspace.package]\nedition="2024"\n\n[lib]\nedition = "2024"\n',
	);
});

test.skipIf(!hasCargo)(
	"migrateEditions takes a 2018 crate to 2024 and rewrites the code each edition needs",
	async () => {
		// Arrange
		const root = await tempRepo({
			"Cargo.toml":
				'[package]\nname = "old"\nversion = "0.1.0"\nedition = "2018"\n',
			"src/lib.rs": 'extern "C" {\n    pub fn abs(x: i32) -> i32;\n}\n',
		});

		// Act
		await migrateEditions(root);

		// Assert
		expect(await textOf(root, "Cargo.toml")).toContain('edition = "2024"');
		expect(await textOf(root, "src/lib.rs")).toContain('unsafe extern "C"');
	},
	240_000,
);
