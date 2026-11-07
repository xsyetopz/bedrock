import { expect, test } from "bun:test";

const build = Bun.spawn(
	["cargo", "build", "--quiet", "--package", "__name__"],
	{
		stderr: "inherit",
	},
);
if ((await build.exited) !== 0) {
	throw new Error("cargo build --package __name__ failed");
}
const { add, sum } = await import("./__name__.ts");

test("add wraps like Rust", () => {
	expect(add(2, 3)).toBe(5);
	expect(add(2 ** 31 - 1, 1)).toBe(-(2 ** 31));
});

test("sum reads a typed array", () => {
	expect(sum(new Int32Array([1, 2, 3]))).toBe(6n);
	expect(sum(new Int32Array())).toBe(0n);
});
