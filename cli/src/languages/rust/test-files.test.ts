import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { rustTestFiles } from "./test-files.ts";

test("picks the files rustc compiles only under test", async () => {
	// Arrange
	const files = {
		"Cargo.toml": '[package]\nname = "a"\nversion = "0.1.0"\n',
		"src/lib.rs": "#[cfg(test)]\nmod test_support;\npub fn run() {}\n",
		"src/test_support.rs": "pub fn fixture() {}\n",
		"src/test_support/data.rs": "pub fn data() {}\n",
		"src/gated.rs": "#![cfg(test)]\nfn helper() {}\n",
		"src/live.rs":
			"pub fn live() {}\n#[cfg(test)]\nmod tests {\n    #[test]\n    fn t() {}\n}\n",
		"src/quoted.rs": "// #![cfg(test)]\npub fn quoted() {}\n",
		"tests/api.rs": "fn helper() {}\n",
		"tests/common/mod.rs": "pub fn setup() {}\n",
		"src/tests/more.rs": "pub fn more() {}\n"
	};
	const root = await tempRepo(files);
	const rust = Object.keys(files).filter((file) => file.endsWith(".rs"));

	// Act
	const picked = await rustTestFiles(root, rust);

	// Assert
	expect([...picked].toSorted()).toEqual([
		"src/gated.rs",
		"src/test_support.rs",
		"src/test_support/data.rs",
		"tests/api.rs",
		"tests/common/mod.rs"
	]);
});
