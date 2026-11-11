//! A C ABI library. Bun loads it with `bun:ffi`; C and Swift can link it too.
#![expect(
    unsafe_code,
    reason = "C ABI exports need #[unsafe(no_mangle)] and raw pointers"
)]

use std::slice;

/// Adds two numbers, wrapping on overflow.
#[unsafe(no_mangle)]
pub extern "C" fn __crate___add(left: i32, right: i32) -> i32 {
    left.wrapping_add(right)
}

/// Sums `len` values starting at `values`. Returns 0 for a null pointer.
///
/// # Safety
///
/// `values` must be null or valid for reads of `len` consecutive `i32` values.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn __crate___sum(values: *const i32, len: usize) -> i64 {
    if values.is_null() {
        return 0;
    }
    // SAFETY: the caller guarantees `values` is valid for `len` reads, and null is handled above.
    let values = unsafe { slice::from_raw_parts(values, len) };
    values.iter().map(|&value| i64::from(value)).sum()
}

#[cfg(test)]
mod tests;
