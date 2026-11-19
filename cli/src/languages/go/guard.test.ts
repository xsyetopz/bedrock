import { expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { guardGo } from "./guard.ts";
import { contextIn } from "./harness.ts";
import { moduleOf, packageOf } from "./module.ts";

test("reports dependency managers and a second golangci-lint config", async () => {
	// Arrange
	const root = await tempRepo({
		"go.mod": "module m\n",
		"Gopkg.toml": "",
		"glide.yaml": "",
		".golangci.yaml": ""
	});

	// Act
	const found = await guardGo(contextIn(root));

	// Assert
	expect(found.map((finding) => finding.split(":")[0])).toEqual([
		".golangci.yaml",
		"Gopkg.toml",
		"glide.yaml"
	]);
});

test("passes a repo with a go.mod and .golangci.yml", async () => {
	// Arrange
	const root = await tempRepo({ "go.mod": "module m\n", ".golangci.yml": "" });

	// Act
	const found = await guardGo(contextIn(root));

	// Assert
	expect(found).toEqual([]);
});

test.each([
	["module github.com/a/b\n\ngo 1.27\n", "github.com/a/b"],
	['module "x.org/y"\n', "x.org/y"],
	["go 1.27\n", undefined]
])("reads the module of %j", (text, expected) => {
	// Act
	const path = moduleOf(text);

	// Assert
	expect(path).toBe(expected);
});

test.each([
	["my-app", "myapp"],
	["9lives", "p9lives"]
])("names the package of %s", (name, expected) => {
	// Act
	const word = packageOf(name);

	// Assert
	expect(word).toBe(expected);
});
