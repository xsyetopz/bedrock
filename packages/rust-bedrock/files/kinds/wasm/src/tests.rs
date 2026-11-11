use super::*;

#[test]
fn counts_and_saturates() {
    assert_eq!(fibonacci(10), 55);
    assert_eq!(fibonacci(200), u64::MAX);
}
