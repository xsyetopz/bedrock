use super::*;

#[test]
fn parses_pairs_and_skips_comments() {
    let pairs = parse("a=1\n\n# c\nb = 2\n").unwrap();
    assert_eq!(pairs.len(), 2);
    assert_eq!(pairs["b"], "2");
}

#[test]
fn reports_the_bad_line() {
    assert_eq!(parse("a=1\n=2"), Err(ParseError { line: 2 }));
    assert_eq!(
        parse("x").unwrap_err().to_string(),
        "line 1: expected key=value"
    );
}
