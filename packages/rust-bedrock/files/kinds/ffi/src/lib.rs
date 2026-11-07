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
mod tests {
    use super::*;

    #[test]
    fn adds_with_wrapping() {
        assert_eq!(__crate___add(2, 3), 5);
        assert_eq!(__crate___add(i32::MAX, 1), i32::MIN);
    }

    #[test]
    fn sums_a_slice_and_tolerates_null() {
        let values = [1, 2, 3];
        // SAFETY: the pointer and length come from a live array.
        assert_eq!(unsafe { __crate___sum(values.as_ptr(), values.len()) }, 6);
        // SAFETY: null is allowed.
        assert_eq!(unsafe { __crate___sum(std::ptr::null(), 4) }, 0);
    }
}
