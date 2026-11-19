import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { globalJsonStep, retarget, retargetProject } from "./manifests.ts";

describe("retarget", () => {
	test.each([
		["net8.0", "net10.0"],
		["net9.0-windows", "net10.0-windows"],
		["netcoreapp3.1", "net10.0"],
		["net10.0", "net10.0"],
		["net11.0", "net11.0"],
		["netstandard2.0", "netstandard2.0"],
		["net48", "net48"],
	])("%s becomes %s", (from, to) => {
		expect(retarget(from)).toBe(to);
	});

	test("dedupes a TargetFrameworks list and keeps an MSBuild property", () => {
		// Arrange
		const text =
			"<TargetFrameworks>net8.0;net9.0;netstandard2.0</TargetFrameworks>\n<TargetFramework>$(AppTarget)</TargetFramework>\n";

		// Act
		const next = retargetProject(text);

		// Assert
		expect(next).toBe(
			"<TargetFrameworks>net10.0;netstandard2.0</TargetFrameworks>\n<TargetFramework>$(AppTarget)</TargetFramework>\n",
		);
	});
});

describe("globalJsonStep", () => {
	test("moves an older SDK to 10 and keeps the other keys", async () => {
		// Arrange
		const root = await tempRepo({
			"global.json":
				'{ "sdk": { "version": "8.0.100" }, "msbuild-sdks": { "X": "1.0" } }\n',
		});

		// Act
		const [step] = await globalJsonStep(root);

		// Assert
		expect(
			step?.kind === "write" ? JSON.parse(String(step.content)) : undefined,
		).toEqual({
			sdk: { version: "10.0.100", rollForward: "latestFeature" },
			"msbuild-sdks": { X: "1.0" },
		});
	});

	test("leaves an SDK on 10 or later", async () => {
		// Arrange
		const root = await tempRepo({
			"global.json": '{ "sdk": { "version": "11.0.100" } }\n',
		});

		// Act
		const steps = await globalJsonStep(root);

		// Assert
		expect(steps).toEqual([]);
	});

	test("adds the testing platform runner beside a current SDK", async () => {
		// Arrange
		const root = await tempRepo({
			"global.json": '{ "sdk": { "version": "10.0.100" } }\n',
		});

		// Act
		const [step] = await globalJsonStep(root, true);

		// Assert
		expect(
			step?.kind === "write" ? JSON.parse(String(step.content)) : undefined,
		).toEqual({
			sdk: { version: "10.0.100" },
			test: { runner: "Microsoft.Testing.Platform" },
		});
	});

	test("leaves a file that already names the testing platform", async () => {
		// Arrange
		const root = await tempRepo({
			"global.json":
				'{ "sdk": { "version": "10.0.100" }, "test": { "runner": "Microsoft.Testing.Platform" } }\n',
		});

		// Act
		const steps = await globalJsonStep(root, true);

		// Assert
		expect(steps).toEqual([]);
	});
});
