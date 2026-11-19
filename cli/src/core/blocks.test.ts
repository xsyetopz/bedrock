import { describe, expect, test } from "bun:test";

import { removeBlock, upsert } from "./blocks.ts";

describe("upsert", () => {
	test("starts a missing file from the header", () => {
		expect(
			upsert(undefined, "# Agents\n", "markdown", [
				{ name: "bedrock", content: "base\n" },
			]),
		).toBe("# Agents\n\n<!-- bedrock -->\nbase\n<!-- /bedrock -->\n");
	});

	test("replaces a block in place and keeps the project's lines", () => {
		const current = "own\n\n# >>> bedrock\nold\n# <<< bedrock\nmore\n";
		expect(
			upsert(current, "", "hash", [{ name: "bedrock", content: "new\n" }]),
		).toBe("own\n\n# >>> bedrock\nnew\n# <<< bedrock\nmore\n");
	});

	test("inserts a missing block before the blocks listed after it", () => {
		const current =
			"# >>> bedrock\nb\n# <<< bedrock\n\n# >>> jvm-bedrock\nj\n# <<< jvm-bedrock\n";
		expect(
			upsert(current, "", "hash", [{ name: "rust-bedrock", content: "r\n" }]),
		).toBe(
			"# >>> bedrock\nb\n# <<< bedrock\n\n# >>> rust-bedrock\nr\n# <<< rust-bedrock\n\n# >>> jvm-bedrock\nj\n# <<< jvm-bedrock\n",
		);
	});

	test("is idempotent", () => {
		const blocks = [
			{ name: "bedrock", content: "b\n" },
			{ name: "typescript-bedrock", content: "t\n" },
		];
		const once = upsert("own\n", "", "slash", blocks);
		expect(upsert(once, "", "slash", blocks)).toBe(once);
	});
});

describe("removeBlock", () => {
	test("drops the block and the blank line that separated it", () => {
		const text =
			"# >>> bedrock\nb\n# <<< bedrock\n\n# >>> rust-bedrock\nr\n# <<< rust-bedrock\n";
		expect(removeBlock(text, "hash", "bedrock")).toBe(
			"# >>> rust-bedrock\nr\n# <<< rust-bedrock\n",
		);
	});

	test("drops the blank line before the last block", () => {
		const text =
			"# >>> bedrock\nb\n# <<< bedrock\n\n# >>> rust-bedrock\nr\n# <<< rust-bedrock\n";
		expect(removeBlock(text, "hash", "rust-bedrock")).toBe(
			"# >>> bedrock\nb\n# <<< bedrock\n",
		);
	});

	test("returns the text unchanged when the block is missing", () => {
		expect(removeBlock("own\n", "hash", "bedrock")).toBe("own\n");
	});
});
