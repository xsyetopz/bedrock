import { describe, expect, test } from "bun:test";

import { upsert } from "./blocks.ts";

const blocks = [
	{ name: "bedrock", content: "- base\n" },
	{ name: "bun-bedrock", content: "- bun\n" },
];

describe("upsert", () => {
	test("starts from the header", () => {
		expect(upsert(undefined, "# Agents\n", "markdown", blocks)).toBe(
			"# Agents\n\n<!-- bedrock -->\n- base\n<!-- /bedrock -->\n\n<!-- bun-bedrock -->\n- bun\n<!-- /bun-bedrock -->\n",
		);
	});

	test("puts a new block before the later bedrocks", () => {
		const current =
			"# >>> bedrock\nbase\n# <<< bedrock\n\n# >>> rust-bedrock\nrust\n# <<< rust-bedrock\n";
		const updated = upsert(current, "", "hash", blocks.slice(1));
		expect(updated).toBe(
			"# >>> bedrock\nbase\n# <<< bedrock\n\n# >>> bun-bedrock\n- bun\n# <<< bun-bedrock\n\n# >>> rust-bedrock\nrust\n# <<< rust-bedrock\n",
		);
		expect(upsert(updated, "", "hash", blocks.slice(1))).toBe(updated);
	});

	test("replaces its blocks and keeps the rest", () => {
		const current =
			"own\n# >>> bun-bedrock\nold\n# <<< bun-bedrock\n# >>> rust-bedrock\nrust\n# <<< rust-bedrock\ntail\n";
		const updated = upsert(current, "", "hash", blocks.slice(1));
		expect(updated).toBe(
			"own\n# >>> bun-bedrock\n- bun\n# <<< bun-bedrock\n# >>> rust-bedrock\nrust\n# <<< rust-bedrock\ntail\n",
		);
		expect(upsert(updated, "", "hash", blocks.slice(1))).toBe(updated);
	});
});
