import { dlopen, FFIType, ptr, suffix } from "bun:ffi";

// `cargo build --package __name__` writes the library here.
const library = new URL(
	`../target/debug/lib__crate__.${suffix}`,
	import.meta.url,
);

const { symbols } = dlopen(library, {
	__crate___add: { args: [FFIType.i32, FFIType.i32], returns: FFIType.i32 },
	__crate___sum: { args: [FFIType.ptr, FFIType.u64], returns: FFIType.i64 },
});

/** Adds two 32-bit integers, wrapping on overflow. */
export const add = (left: number, right: number): number =>
	symbols.__crate___add(left, right);

/** Sums the values in Rust. */
export const sum = (values: Int32Array): bigint =>
	values.length === 0 ? 0n : symbols.__crate___sum(ptr(values), values.length);
