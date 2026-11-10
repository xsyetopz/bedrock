//! Renaming `foo/mod.rs` to `foo.rs` moves the file's own directory one level up,
//! so the relative paths it names need a `foo/` prefix to keep reaching the same files.

use crate::lexer::{Kind, Token, tokens};

const INCLUDES: [&str; 3] = ["include", "include_str", "include_bytes"];

/// The index of the token that closes the `[` at `open`.
fn bracket_end(all: &[Token], open: usize) -> Option<usize> {
    let mut depth = 0_usize;
    for (index, token) in all.iter().enumerate().skip(open) {
        match token.kind {
            Kind::Punct(b'[') => depth += 1,
            Kind::Punct(b']') => {
                depth -= 1;
                if depth == 0 {
                    return Some(index);
                }
            }
            _ => {}
        }
    }
    None
}

/// What to do with a path literal.
enum Literal {
    /// A relative path to rewrite: its byte range and its text.
    Relative(usize, usize, String),
    /// An absolute path, which a rename does not change.
    Absolute,
}

/// Reads a plain string literal; `None` when it has escapes or is not a string.
fn classify(source: &str, token: &Token) -> Option<Literal> {
    let value = token
        .text(source)
        .strip_prefix('"')?
        .strip_suffix('"')?
        .to_owned();
    if value.contains('\\') {
        None
    } else if value.starts_with('/') {
        Some(Literal::Absolute)
    } else {
        Some(Literal::Relative(token.start, token.end, value))
    }
}

fn kind_at(all: &[Token], index: usize) -> Option<Kind> {
    all.get(index).map(|token| token.kind)
}

/// The literal of an `include!("…")`-style call whose name is at `index`.
fn include_literal(all: &[Token], index: usize) -> Option<Token> {
    let literal = *all.get(index + 3)?;
    let opens = matches!(
        kind_at(all, index + 2),
        Some(Kind::Punct(b'(' | b'[' | b'{'))
    );
    let closes = matches!(
        kind_at(all, index + 4),
        Some(Kind::Punct(b')' | b']' | b'}'))
    );
    (opens && closes && literal.kind == Kind::Literal).then_some(literal)
}

/// Whether the item an attribute ending at `end` decorates is an inline module.
fn precedes_inline_module(all: &[Token], end: usize) -> bool {
    all.iter()
        .skip(end + 1)
        .find(|token| matches!(token.kind, Kind::Punct(b';' | b'{')))
        .is_some_and(|token| token.kind == Kind::Punct(b'{'))
}

/// Rewrites the relative paths in `source`, the text of a `mod.rs`, for a file one
/// directory up: each `#[path = "…"]` on a `mod x;` and each `include!`-style literal
/// gains `prefix`. A path inside an inline `mod x { }` keeps its text, since rustc
/// resolves it through the module's own directory, which does not move. Returns
/// `None` when a path cannot be rewritten safely: one that is computed, escaped, set
/// through `cfg_attr`, or given to an inline module.
pub(crate) fn prefix_paths(source: &str, prefix: &str) -> Option<String> {
    let all = tokens(source);
    let mut edits: Vec<(usize, usize, String)> = Vec::new();
    let mut depth = 0_usize;
    let mut index = 0;
    while index < all.len() {
        let token = all[index];
        let mut literal = None;
        match token.kind {
            Kind::Punct(b'{') => depth += 1,
            Kind::Punct(b'}') => depth = depth.saturating_sub(1),
            Kind::Ident
                if INCLUDES.contains(&token.text(source))
                    && kind_at(&all, index + 1) == Some(Kind::Punct(b'!')) =>
            {
                literal = Some(include_literal(&all, index)?);
            }
            Kind::Punct(b'#') => {
                let open =
                    index + 1 + usize::from(kind_at(&all, index + 1) == Some(Kind::Punct(b'!')));
                if kind_at(&all, open) == Some(Kind::Punct(b'[')) {
                    let end = bracket_end(&all, open)?;
                    let inner = &all[open + 1..end];
                    let names_path = inner.iter().enumerate().any(|(at, part)| {
                        part.text(source) == "path"
                            && kind_at(inner, at + 1) == Some(Kind::Punct(b'='))
                    });
                    if names_path && depth == 0 {
                        if inner.len() != 3 || precedes_inline_module(&all, end) {
                            return None;
                        }
                        literal = Some(inner[2]);
                    }
                    index = end;
                }
            }
            _ => {}
        }
        if let Some(found) = literal {
            match classify(source, &found)? {
                Literal::Relative(start, end, value) => edits.push((start, end, value)),
                Literal::Absolute => {}
            }
        }
        index += 1;
    }
    let mut text = source.to_owned();
    for (start, end, value) in edits.into_iter().rev() {
        text.replace_range(start..end, &format!("\"{prefix}{value}\""));
    }
    Some(text)
}

#[cfg(test)]
mod tests {
    use super::prefix_paths;

    #[test]
    fn prefixes_path_attributes_and_includes() {
        let source = "#[cfg(test)]\n#[path = \"checking_matrix/tests.rs\"]\nmod checking_matrix;\ninclude!(\"tests/urls.rs\");\nconst A: &str = include_str!(\"data.txt\");\n";
        assert_eq!(
            prefix_paths(source, "session/").unwrap(),
            "#[cfg(test)]\n#[path = \"session/checking_matrix/tests.rs\"]\nmod checking_matrix;\ninclude!(\"session/tests/urls.rs\");\nconst A: &str = include_str!(\"session/data.txt\");\n"
        );
    }

    #[test]
    fn leaves_absolute_paths_and_inline_module_paths_alone() {
        let source = "include!(\"/abs/x.rs\");\nmod inline {\n    #[path = \"deep.rs\"]\n    mod deep;\n}\n// include!(\"c.rs\");\n";
        assert_eq!(prefix_paths(source, "foo/").unwrap(), source);
    }

    #[test]
    fn still_prefixes_an_include_inside_an_inline_module() {
        let source = "mod inline {\n    const A: &str = include_str!(\"a.txt\");\n}\n";
        assert_eq!(
            prefix_paths(source, "foo/").unwrap(),
            "mod inline {\n    const A: &str = include_str!(\"foo/a.txt\");\n}\n"
        );
    }

    #[test]
    fn refuses_what_it_cannot_rewrite() {
        assert!(prefix_paths("include!(concat!(env!(\"OUT_DIR\"), \"/x.rs\"));", "foo/").is_none());
        assert!(prefix_paths("#[cfg_attr(test, path = \"a.rs\")]\nmod a;", "foo/").is_none());
        assert!(prefix_paths("#[path = \"d\"]\nmod d { mod e; }", "foo/").is_none());
        assert!(prefix_paths("#[path = \"a\\\\b.rs\"]\nmod a;", "foo/").is_none());
    }
}
