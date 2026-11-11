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
