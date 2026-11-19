import { describe, expect, test } from "bun:test";
import { join } from "node:path";

import { $ } from "bun";

import { tempRepo } from "../../core/testing.ts";
import { adoptTypescript } from "./harness.ts";

/** Runs `just typescript::typecheck` in a repo whose project.just holds `project`. */
const typecheck = async (project: string): Promise<string> => {
	const root = await tempRepo();
	await adoptTypescript(root);
	await Bun.write(
		join(root, "justfile"),
		'mod typescript ".just/typescript.just"\nimport? "project.just"\n'
	);
	await Bun.write(join(root, "project.just"), project);
	await Bun.write(
		join(root, "package.json"),
		'{"scripts":{"check":"echo ran-check"}}'
	);
	const result = await $`just typescript::typecheck`
		.cwd(root)
		.nothrow()
		.quiet();
	return `${result.stdout.toString()}${result.stderr.toString()}`;
};

describe("typescript::typecheck", () => {
	test("runs project-pre-typecheck before bun check", async () => {
		// Arrange
		const project = 'project-pre-typecheck:\n\t@echo "ran-pre"\n';

		// Act
		const output = await typecheck(project);

		// Assert
		expect(output).toContain("ran-pre");
		expect(output.indexOf("ran-pre")).toBeLessThan(output.indexOf("ran-check"));
	});

	test("runs only bun check without such a recipe", async () => {
		// Arrange
		const project = 'project-lint:\n\t@echo "ran-lint"\n';

		// Act
		const output = await typecheck(project);

		// Assert
		expect(output).not.toContain("ran-lint");
		expect(output).toContain("ran-check");
	});
});
