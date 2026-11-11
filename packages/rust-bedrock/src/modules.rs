//! Reads the `mod name;` and `mod name { … }` declarations of a Rust file and finds
//! the file that holds the body of an out-of-line one.

use std::path::{Path, PathBuf};

use crate::lexer::{Kind, Token, tokens};

/// A `mod name;` or `mod name { … }` declaration.
#[derive(Debug)]
pub(crate) struct Module {
    pub name: String,
    pub line: usize,
    /// The `#[path = "…"]` value, if the declaration has one.
    pub path: Option<String>,
    /// Byte range of that `#[path = "…"]` attribute.
    pub path_attribute: Option<(usize, usize)>,
    /// Whether a `#[cfg(test)]`-style attribute gates it.
    pub test_only: bool,
    /// Byte offset of `mod`.
    pub start: usize,
    /// Byte offsets of the `{` and `}` of an inline body, or `None` for `mod name;`.
    pub body: Option<(usize, usize)>,
    /// How many braces enclose the declaration.
    pub depth: usize,
}

fn closing(tokens: &[Token], open_at: usize) -> Option<usize> {
    let (open, close) = match tokens.get(open_at)?.kind {
        Kind::Punct(b'[') => (b'[', b']'),
        Kind::Punct(b'(') => (b'(', b')'),
        _ => return None,
    };
    let mut depth = 0_usize;
    for (index, token) in tokens.iter().enumerate().skip(open_at) {
        match token.kind {
            Kind::Punct(byte) if byte == open => depth += 1,
            Kind::Punct(byte) if byte == close => {
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

/// Splits the tokens inside `( … )` at top-level commas.
fn arguments(group: &[Token]) -> Vec<&[Token]> {
    let mut parts = Vec::new();
    let mut depth = 0_usize;
    let mut start = 0;
    for (index, token) in group.iter().enumerate() {
        match token.kind {
            Kind::Punct(b'(') => depth += 1,
            Kind::Punct(b')') => depth -= 1,
            Kind::Punct(b',') if depth == 0 => {
                parts.push(&group[start..index]);
                start = index + 1;
            }
            _ => {}
        }
    }
    parts.push(&group[start..]);
    parts.retain(|part| !part.is_empty());
    parts
}

/// Whether the cfg predicate holds only when compiling tests: `all` needs one such
/// part, `any` needs every part to be one, and `not` never is.
fn implies_test(source: &str, predicate: &[Token]) -> bool {
    match predicate {
        [word] => word.text(source) == "test",
        [word, open, inner @ .., close]
            if open.kind == Kind::Punct(b'(') && close.kind == Kind::Punct(b')') =>
        {
            let parts = arguments(inner);
            match word.text(source) {
                "all" => parts.iter().any(|part| implies_test(source, part)),
                "any" => !parts.is_empty() && parts.iter().all(|part| implies_test(source, part)),
                _ => false,
            }
        }
        _ => false,
    }
}

fn is_test_cfg(source: &str, inner: &[Token]) -> bool {
    match inner {
        [word, open, predicate @ .., close]
            if word.text(source) == "cfg"
                && open.kind == Kind::Punct(b'(')
                && close.kind == Kind::Punct(b')') =>
        {
            implies_test(source, predicate)
        }
        _ => false,
    }
}

fn path_value(source: &str, inner: &[Token]) -> Option<String> {
    match inner {
        [name, equals, value]
            if name.text(source) == "path"
                && equals.kind == Kind::Punct(b'=')
                && value.kind == Kind::Literal =>
        {
            Some(value.text(source).trim_matches('"').to_owned())
        }
        _ => None,
    }
}

/// Every `mod name;` and `mod name { … }` declaration in `source`, in order.
pub(crate) fn modules(source: &str) -> Vec<Module> {
    let all = tokens(source);
    let mut depths = Vec::with_capacity(all.len());
    let mut depth = 0_usize;
    for token in &all {
        if token.kind == Kind::Punct(b'}') {
            depth = depth.saturating_sub(1);
        }
        depths.push(depth);
        if token.kind == Kind::Punct(b'{') {
            depth += 1;
        }
    }
    let mut found = Vec::new();
    let mut at = 0;
    while at < all.len() {
        let mut test_only = false;
        let mut path = None;
        let mut path_attribute = None;
        let mut cursor = at;
        while all.get(cursor).map(|token| token.kind) == Some(Kind::Punct(b'#')) {
            let Some(end) = closing(&all, cursor + 1) else {
                break;
            };
            let inner = &all[cursor + 2..end];
            test_only |= is_test_cfg(source, inner);
            if path.is_none() {
                path = path_value(source, inner);
                path_attribute = path.as_ref().map(|_| (all[cursor].start, all[end].end));
            }
            cursor = end + 1;
        }
        if all
            .get(cursor)
            .is_some_and(|token| token.text(source) == "pub")
        {
            cursor += 1;
            if let Some(end) = closing(&all, cursor) {
                cursor = end + 1;
            }
        }
        if all
            .get(cursor)
            .is_some_and(|token| token.text(source) == "unsafe")
        {
            cursor += 1;
        }
        if let [keyword, name, after, ..] = &all[cursor.min(all.len())..]
            && keyword.text(source) == "mod"
            && name.kind == Kind::Ident
        {
            let body = match after.kind {
                Kind::Punct(b';') => Some(None),
                Kind::Punct(b'{') => {
                    braces_end(&all, cursor + 2).map(|end| Some((after.start, all[end].start)))
                }
                _ => None,
            };
            if let Some(body) = body {
                found.push(Module {
                    name: name.text(source).trim_start_matches("r#").to_owned(),
                    line: keyword.line,
                    path,
                    path_attribute,
                    test_only,
                    start: keyword.start,
                    body,
                    depth: depths[cursor],
                });
                at = cursor + 3;
                continue;
            }
        }
        at = cursor.max(at + 1);
    }
    found
}

/// The index of the token that closes the `{` at `open`.
fn braces_end(all: &[Token], open: usize) -> Option<usize> {
    let mut depth = 0_usize;
    for (index, token) in all.iter().enumerate().skip(open) {
        match token.kind {
            Kind::Punct(b'{') => depth += 1,
            Kind::Punct(b'}') => {
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

/// Whether `file` is a crate root or a `mod.rs`, whose child modules sit beside it
/// rather than in a directory named after it.
fn owns_directory(file: &Path) -> bool {
    let name = file
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or_default();
    if matches!(name, "lib.rs" | "main.rs" | "mod.rs" | "build.rs") {
        return true;
    }
    // A target's own root: tests/x.rs, examples/x.rs, benches/x.rs beside the
    // Cargo.toml, or src/bin/x.rs. A directory merely named `tests` inside src/ is
    // an ordinary module directory.
    let parent = file.parent().unwrap_or_else(|| Path::new(""));
    let kind = parent
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or_default();
    let package = match kind {
        "tests" | "examples" | "benches" => parent.parent(),
        "bin" => parent.parent().and_then(Path::parent),
        _ => None,
    };
    package.is_some_and(|package| package.join("Cargo.toml").is_file())
}

/// The directory where the child modules of `file` sit: beside a `mod.rs`, `lib.rs`,
/// or `main.rs`, and in a directory named for the file otherwise.
pub(crate) fn module_directory(file: &Path) -> PathBuf {
    let directory = file.parent().unwrap_or_else(|| Path::new(""));
    if owns_directory(file) {
        directory.to_path_buf()
    } else {
        directory.join(file.file_stem().unwrap_or_default())
    }
}

/// Returns the file that holds the body of `declaration`, declared in `file`.
pub(crate) fn child_file(file: &Path, declaration: &Module) -> Option<PathBuf> {
    let directory = file.parent().unwrap_or_else(|| Path::new(""));
    if let Some(path) = &declaration.path {
        return Some(directory.join(path)).filter(|candidate| candidate.is_file());
    }
    let base = module_directory(file);
    [
        base.join(format!("{}.rs", declaration.name)),
        base.join(&declaration.name).join("mod.rs"),
    ]
    .into_iter()
    .find(|candidate| candidate.is_file())
}

#[cfg(test)]
mod tests;
