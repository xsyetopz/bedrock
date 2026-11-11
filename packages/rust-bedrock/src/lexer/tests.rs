use super::{Kind, tokens};

fn texts(source: &str) -> Vec<&str> {
    tokens(source)
        .iter()
        .map(|token| token.text(source))
        .collect()
}

#[test]
fn skips_comments_including_nested_blocks() {
    assert_eq!(texts("a // mod x;\n/* /* mod y; */ */ b"), ["a", "b"]);
}

#[test]
fn keeps_strings_whole() {
    assert_eq!(
        texts(r###"x "a\"b" r#"c"d"# br##"e"#f"## b"g" c"h" y"###),
        [
            "x",
            r#""a\"b""#,
            r##"r#"c"d"#"##,
            r###"br##"e"#f"##"###,
            r#"b"g""#,
            r#"c"h""#,
            "y"
        ]
    );
}

#[test]
fn tells_chars_from_lifetimes() {
    let source = "'a' '\\'' 'é' b'x' &'static str";
    let kinds: Vec<Kind> = tokens(source).iter().map(|token| token.kind).collect();
    assert_eq!(
        kinds,
        [
            Kind::Literal,
            Kind::Literal,
            Kind::Literal,
            Kind::Literal,
            Kind::Punct(b'&'),
            Kind::Ident,
            Kind::Ident
        ]
    );
}

#[test]
fn counts_lines_across_literals() {
    let source = "\"a\nb\"\nmod";
    assert_eq!(tokens(source).last().map(|token| token.line), Some(3));
}

#[test]
fn reads_raw_identifiers() {
    assert_eq!(texts("r#mod r#type"), ["r#mod", "r#type"]);
}
