import { describe, expect, test } from "bun:test";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { $ } from "bun";

import { tempRepo } from "../../core/testing.ts";
import { adoptTypescript } from "./harness.ts";
import { asRecord, readJsonc } from "./json.ts";

const oxlint = join(
	dirname(fileURLToPath(import.meta.resolve("oxlint/package.json"))),
	"bin/oxlint",
);

/** What oxlint prints for `source` at `path`, under the config adopt writes. */
const reported = async (path: string, source: string): Promise<string> => {
	const root = await tempRepo();
	await adoptTypescript(root);
	const config = asRecord(await readJsonc(join(root, ".oxlintrc.json")));
	await Bun.write(
		join(root, ".oxlintrc.json"),
		JSON.stringify({
			...config,
			options: { ...asRecord(config["options"]), typeAware: false },
		}),
	);
	await Bun.write(join(root, path), source);
	const result = await $`bun ${oxlint} -c .oxlintrc.json ${path}`
		.cwd(root)
		.nothrow()
		.quiet();
	return result.stdout.toString();
};

const HOOK = "declare const hook: { catch(handler: unknown): void };\n";
const IMPORTS = Array.from(
	{ length: 40 },
	(_, index) =>
		`import { hook${index} } from "./hook-${index}.ts";\nhook${index}();\n`,
).join("");

describe("unicorn/catch-error-name", () => {
	test("accepts the engine $ and _$ as the first parameter of a hook catch", async () => {
		// Arrange
		const source = `${HOOK}hook.catch(($, e, next) => [$, e, next]);\nhook.catch((_$, e, next) => [e, next]);\n`;

		// Act
		const output = await reported("sample.ts", source);

		// Assert
		expect(output).not.toContain("catch-error-name");
	});

	test("still rejects e and engine as the name of a caught error", async () => {
		// Arrange
		const source = `${HOOK}hook.catch((e) => [e]);\nhook.catch((engine) => [engine]);\n`;

		// Act
		const output = await reported("sample.ts", source);

		// Assert
		expect(output).toContain("catch-error-name");
	});
});

describe("import/max-dependencies", () => {
	test("is off for hooks/register, which imports every hook module", async () => {
		// Act
		const output = await reported("hooks/register.ts", IMPORTS);

		// Assert
		expect(output).not.toContain("max-dependencies");
	});

	test("still applies to other files", async () => {
		// Act
		const output = await reported("src/index.ts", IMPORTS);

		// Assert
		expect(output).toContain("max-dependencies");
	});
});
