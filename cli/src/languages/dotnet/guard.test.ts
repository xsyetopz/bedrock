import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { guardDotnet, suppressionFindings } from "./guard.ts";

describe("suppressionFindings", () => {
	test("reports a C# pragma and SuppressMessage without a reason", () => {
		// Arrange
		const text = [
			"#pragma warning disable CA1822",
			"#pragma warning disable CA1822 // instance method for the test runner",
			'[SuppressMessage("Design", "CA1031")]',
			'[SuppressMessage("Design", "CA1031", Justification = "top-level handler")]',
		].join("\n");

		// Act
		const findings = suppressionFindings("src/A.cs", text);

		// Assert
		expect(
			findings.map((finding) => finding.split(":").slice(0, 2).join(":")),
		).toEqual(["src/A.cs:1", "src/A.cs:3"]);
	});

	test("reports an F# nowarn without a reason", () => {
		// Arrange
		const text =
			'#nowarn "40"\n#nowarn "3370" // the indexer syntax the API needs\n';

		// Act
		const findings = suppressionFindings("src/A.fs", text);

		// Assert
		expect(findings).toEqual([
			"src/A.fs:1: #nowarn without a reason; add `// <reason>` after the ids",
		]);
	});
});

describe("guardDotnet", () => {
	test("reports a formatter config and a second tool manifest", async () => {
		// Arrange
		const root = await tempRepo({
			".csharpierrc.json": "{}\n",
			"dotnet-tools.json": "{}\n",
			".config/dotnet-tools.json": "{}\n",
		});

		// Act
		const findings = await guardDotnet({
			root,
			options: {},
			languages: ["dotnet"],
			init: false,
			templates: () => {
				throw new Error("unused");
			},
		});

		// Assert
		expect(findings).toEqual([
			".csharpierrc.json: the formatters read their layout from .editorconfig; move the settings there and delete this file",
			".config/dotnet-tools.json, dotnet-tools.json: two tool manifests; keep the tools in .config/dotnet-tools.json and delete the other",
		]);
	});
});
