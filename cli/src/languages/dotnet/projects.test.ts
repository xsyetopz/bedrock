import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { buildEntries } from "./projects.ts";

describe("buildEntries", () => {
	test("takes the root solutions, the slnx over a same-named sln", async () => {
		// Arrange
		const root = await tempRepo({
			"App.sln": "",
			"App.slnx": "<Solution />\n",
			"src/App/App.csproj": "<Project />\n",
		});

		// Act
		const entries = await buildEntries(root);

		// Assert
		expect(entries).toEqual(["App.slnx"]);
	});

	test("takes each project without a solution", async () => {
		// Arrange
		const root = await tempRepo({
			"src/B/B.fsproj": "<Project />\n",
			"src/A/A.csproj": "<Project />\n",
		});

		// Act
		const entries = await buildEntries(root);

		// Assert
		expect(entries).toEqual(["src/A/A.csproj", "src/B/B.fsproj"]);
	});
});
