//! Integration tests run against the public API, as another crate would.
#![cfg(test)]

#[test]
fn parses_a_file() {
    let pairs = __crate__::parse("name = demo\nversion = 1\n").unwrap();
    assert_eq!(pairs["name"], "demo");
}
