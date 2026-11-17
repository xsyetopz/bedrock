import { expect, test } from "bun:test";

import { crateVariables, kindsOf, packageName } from "./kinds.ts";

test("defaults to a library", () => {
	// Arrange
	const options = {};

	// Act
	const kinds = kindsOf(options);

	// Assert
	expect(kinds).toEqual(["lib"]);
});

test("reads the comma-separated kinds once each", () => {
	// Arrange
	const options = { kind: "lib,wasm,lib" };

	// Act
	const kinds = kindsOf(options);

	// Assert
	expect(kinds).toEqual(["lib", "wasm"]);
});

test("turns a directory name into a package name Cargo accepts", () => {
	// Arrange
	const roots = ["/w/my app", "/w/2fast", "/w/--", "/w/ok_name-1", "/w/MyApp"];

	// Act
	const names = roots.map((root) => packageName(root));

	// Assert
	expect(names).toEqual(["my-app", "app-2fast", "app", "ok_name-1", "myapp"]);
});

test("the crate name replaces hyphens with underscores", () => {
	// Arrange
	const pkg = "my-lib-wasm";

	// Act
	const variables = crateVariables(pkg);

	// Assert
	expect(variables).toEqual({ package: "my-lib-wasm", crate: "my_lib_wasm" });
});
