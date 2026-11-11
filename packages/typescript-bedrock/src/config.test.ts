import { describe, expect, test } from "bun:test";
import { join } from "node:path";

const config: unknown = await Bun.file(
	join(import.meta.dir, "..", "oxlintrc.json"),
).json();

const rulesOf = (value: unknown): Record<string, unknown> => {
	if (typeof value === "object" && value !== null && "rules" in value) {
		const { rules } = value;
		if (typeof rules === "object" && rules !== null) {
			return Object.fromEntries(Object.entries(rules));
		}
	}
	return {};
};

describe("oxlintrc.json", () => {
	const rules = rulesOf(config);

	test("skips the argument check of no-misused-promises, which type-checks every callback argument against its overloads and takes gigabytes on a large overloaded API", () => {
		expect(rules["typescript/no-misused-promises"]).toEqual([
			"error",
			{ checksVoidReturn: { arguments: false } },
		]);
	});

	test("turns off strict-void-return, which resolves the same contextual signatures", () => {
		expect(rules["typescript/strict-void-return"]).toBe("off");
	});
});
