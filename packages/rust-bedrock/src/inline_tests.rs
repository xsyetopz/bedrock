//! Finds `#[cfg(test)] mod name;` declarations, whose tests live in another
//! file, and moves those tests inline as `mod name { … }`.

use std::fs;
use std::path::{Path, PathBuf};

use crate::guard::is_test_file_in_src;
use crate::lexer::{Kind, Token, tokens};

/// A `mod name;` declaration.
#[derive(Debug)]
pub(crate) struct OutOfLine {
    pub name: String,
    pub line: usize,
    /// The `#[path = "…"]` value, if the declaration has one.
    pub path: Option<String>,
    /// Byte range of that `#[path = "…"]` attribute, which an inline module drops.
    pub path_attribute: Option<(usize, usize)>,
    /// Whether a `#[cfg(test)]`-style attribute gates it.
    pub test_only: bool,
    /// Byte range from `mod` through `;`.
    pub span: (usize, usize),
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

pub(crate) fn declarations(source: &str) -> Vec<OutOfLine> {
    let all = tokens(source);
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
        if let [keyword, name, semicolon, ..] = &all[cursor.min(all.len())..]
            && keyword.text(source) == "mod"
            && name.kind == Kind::Ident
            && semicolon.kind == Kind::Punct(b';')
        {
            found.push(OutOfLine {
                name: name.text(source).trim_start_matches("r#").to_owned(),
                line: keyword.line,
                path,
                path_attribute,
                test_only,
                span: (keyword.start, semicolon.end),
            });
            at = cursor + 3;
        } else {
            at = cursor.max(at + 1);
        }
    }
    found
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

/// Returns the file that holds the body of `declaration`, declared in `file`.
pub(crate) fn child_file(file: &Path, declaration: &OutOfLine) -> Option<PathBuf> {
    let directory = file.parent().unwrap_or_else(|| Path::new(""));
    if let Some(path) = &declaration.path {
        return Some(directory.join(path)).filter(|candidate| candidate.is_file());
    }
    // rustc looks in one place only: beside a mod.rs, lib.rs or main.rs, and in a
    // directory named for the file otherwise.
    let base = if owns_directory(file) {
        directory.to_path_buf()
    } else {
        directory.join(file.file_stem().unwrap_or_default())
    };
    [
        base.join(format!("{}.rs", declaration.name)),
        base.join(&declaration.name).join("mod.rs"),
    ]
    .into_iter()
    .find(|candidate| candidate.is_file())
}

const INCLUDES: [&str; 3] = ["include", "include_str", "include_bytes"];

/// An `include!`-style call with a plain relative string literal.
#[derive(Debug)]
pub(crate) struct Include {
    pub name: String,
    pub value: String,
    pub line: usize,
    /// Byte range of the string literal.
    pub literal: (usize, usize),
    /// Byte range of the call, through the `;` when it stands as an item.
    pub call: (usize, usize),
    /// Whether it is `include!("…");` standing on its own as an item, which can be
    /// replaced by the file's contents.
    pub item: bool,
}

pub(crate) fn includes(source: &str) -> Vec<Include> {
    let all = tokens(source);
    let mut found = Vec::new();
    for (index, token) in all.iter().enumerate() {
        let name = token.text(source);
        if token.kind != Kind::Ident || !INCLUDES.contains(&name) {
            continue;
        }
        let [_, bang, open, literal, close] = &all[index..all.len().min(index + 5)] else {
            continue;
        };
        if bang.kind != Kind::Punct(b'!')
            || open.kind != Kind::Punct(b'(')
            || literal.kind != Kind::Literal
            || close.kind != Kind::Punct(b')')
        {
            continue;
        }
        let Some(value) = literal
            .text(source)
            .strip_prefix('"')
            .and_then(|text| text.strip_suffix('"'))
            .filter(|text| !text.contains('\\') && !text.starts_with('/'))
        else {
            continue;
        };
        let standing = index
            .checked_sub(1)
            .is_none_or(|before| matches!(all[before].kind, Kind::Punct(b';' | b'}' | b'{')));
        let semicolon = all
            .get(index + 5)
            .filter(|next| next.kind == Kind::Punct(b';'));
        found.push(Include {
            name: name.to_owned(),
            value: value.to_owned(),
            line: token.line,
            literal: (literal.start, literal.end),
            call: (token.start, semicolon.map_or(close.end, |next| next.end)),
            item: name == "include" && standing && semicolon.is_some(),
        });
    }
    found
}

/// The components of `path` with each `..` folded into the directory before it, so
/// the result does not pass through a directory that is about to be deleted.
fn normalized(path: &Path) -> Vec<std::path::Component<'_>> {
    let mut parts: Vec<std::path::Component<'_>> = Vec::new();
    for part in path.components() {
        match part {
            std::path::Component::ParentDir
                if parts
                    .last()
                    .is_some_and(|last| matches!(last, std::path::Component::Normal(_))) =>
            {
                parts.pop();
            }
            std::path::Component::CurDir => {}
            other => parts.push(other),
        }
    }
    parts
}

/// The path from directory `from` to `to`, with `..` where they part.
fn relative_to(from: &Path, to: &Path) -> PathBuf {
    let from = normalized(from);
    let to = normalized(to);
    let shared = from.iter().zip(&to).take_while(|(a, b)| a == b).count();
    let mut path = PathBuf::new();
    for _ in shared..from.len() {
        path.push("..");
    }
    for part in &to[shared..] {
        path.push(part);
    }
    path
}

/// What `expand` found out about a tree: what blocks it, what it only reports, and
/// the files it emptied, leaves before the files that held them.
#[derive(Default)]
struct Outcome {
    problems: Vec<String>,
    notes: Vec<String>,
    moved: Vec<PathBuf>,
}

fn failure(operation: &str, path: &Path, error: &std::io::Error) -> String {
    format!("{}: cannot {operation}: {error}", path.display())
}

enum Edit {
    Module(OutOfLine),
    Include(Include),
}

/// Inlines the out-of-line modules of `source`, the text of `file`, depth first so
/// a module's own children move into it before it moves. `nested` is set inside a
/// test module, where every `mod name;` is test code and each `include!("…");` of a
/// file is spliced in; outside one, only the `include!` of a test file is. `dir` is where the text's relative include paths start, and
/// `home` the directory of the file the text ends up in. Any other include path is
/// rewritten to still reach its file from `home`. A module whose tree cannot all
/// move is left alone, with its problems; the other modules still move.
fn expand(
    file: &Path,
    dir: &Path,
    home: &Path,
    source: &mut String,
    nested: bool,
    out: &mut Outcome,
) {
    let mut edits: Vec<(usize, Edit)> = declarations(source)
        .into_iter()
        .filter(|found| nested || found.test_only)
        .map(|found| (found.span.0, Edit::Module(found)))
        .collect();
    // Outside a test module only the includes of test files are spliced in.
    edits.extend(
        includes(source)
            .into_iter()
            .filter(|found| {
                nested
                    || (found.item
                        && is_test_file_in_src(&dir.join(&found.value).to_string_lossy()))
            })
            .map(|found| (found.call.0, Edit::Include(found))),
    );
    edits.sort_by_key(|(start, _)| std::cmp::Reverse(*start));
    for (_, edit) in edits {
        match edit {
            Edit::Module(declaration) => expand_module(file, home, source, &declaration, out),
            Edit::Include(call) => expand_include(file, dir, home, source, &call, out),
        }
    }
}

fn expand_module(
    file: &Path,
    home: &Path,
    source: &mut String,
    declaration: &OutOfLine,
    out: &mut Outcome,
) {
    let location = format!("{}:{}", file.display(), declaration.line);
    let Some(child) = child_file(file, declaration) else {
        out.problems.push(format!(
            "{location}: cannot find the file for mod {}",
            declaration.name
        ));
        return;
    };
    let mut body = match fs::read_to_string(&child) {
        Ok(body) => body,
        Err(error) => {
            out.problems.push(failure("read", &child, &error));
            return;
        }
    };
    let mut inner = Outcome::default();
    let child_dir = child.parent().unwrap_or_else(|| Path::new(""));
    expand(&child, child_dir, home, &mut body, true, &mut inner);
    out.notes.append(&mut inner.notes);
    if !inner.problems.is_empty() {
        out.problems.append(&mut inner.problems);
        return;
    }
    let body = body.trim_end();
    let replacement = format!("mod {} {{\n{body}\n}}", declaration.name);
    source.replace_range(declaration.span.0..declaration.span.1, &replacement);
    if let Some((start, end)) = declaration.path_attribute {
        let line_end = source[end..]
            .find(|character: char| !character.is_whitespace() || character == '\n')
            .map_or(end, |offset| end + offset);
        let line_end = line_end + usize::from(source[line_end..].starts_with('\n'));
        source.replace_range(start..line_end, "");
    }
    out.moved.append(&mut inner.moved);
    out.moved.push(child);
}

fn expand_include(
    file: &Path,
    dir: &Path,
    home: &Path,
    source: &mut String,
    call: &Include,
    out: &mut Outcome,
) {
    let target = dir.join(&call.value);
    if call.item && target.is_file() {
        let mut text = match fs::read_to_string(&target) {
            Ok(text) => text,
            Err(error) => {
                out.problems.push(failure("read", &target, &error));
                return;
            }
        };
        let mut inner = Outcome::default();
        let target_dir = target.parent().unwrap_or_else(|| Path::new(""));
        expand(file, target_dir, home, &mut text, true, &mut inner);
        out.notes.append(&mut inner.notes);
        if !inner.problems.is_empty() {
            out.problems.append(&mut inner.problems);
            return;
        }
        source.replace_range(call.call.0..call.call.1, text.trim_end());
        out.moved.append(&mut inner.moved);
        out.moved.push(target);
        return;
    }
    if !target.exists() {
        out.notes.push(format!(
            "{}:{}: cannot find {} named by {}!; its path now points at the same place",
            file.display(),
            call.line,
            target.display(),
            call.name
        ));
    }
    let path = relative_to(home, &target);
    if path != Path::new(&call.value) {
        let literal = format!("\"{}\"", path.display());
        source.replace_range(call.literal.0..call.literal.1, &literal);
    }
}

/// Moves every out-of-line test module of `file` inline, with the modules inside
/// them and the files they `include!`. Returns one message per module it could not
/// move or file it could not read or remove; none of them stops the other modules.
pub(crate) fn inline(file: &Path) -> Vec<String> {
    let mut source = match fs::read_to_string(file) {
        Ok(source) => source,
        Err(error) => return vec![failure("read", file, &error)],
    };
    let home = file.parent().unwrap_or_else(|| Path::new(""));
    let mut out = Outcome::default();
    expand(file, home, home, &mut source, false, &mut out);
    let mut problems = out.problems;
    problems.append(&mut out.notes);
    if out.moved.is_empty() {
        return problems;
    }
    if let Err(error) = fs::write(file, source) {
        problems.push(failure("write", file, &error));
        return problems;
    }
    for child in out.moved {
        match fs::remove_file(&child) {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(error) => {
                problems.push(failure("remove", &child, &error));
                continue;
            }
        }
        let mut directory = child.parent();
        while let Some(parent) = directory
            && fs::read_dir(parent).is_ok_and(|mut entries| entries.next().is_none())
        {
            if let Err(error) = fs::remove_dir(parent) {
                problems.push(failure("remove", parent, &error));
                break;
            }
            directory = parent.parent();
        }
    }
    problems
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::Path;

    use super::{child_file, declarations, inline};

    fn names(source: &str) -> Vec<(String, bool)> {
        declarations(source)
            .into_iter()
            .map(|found| (found.name, found.test_only))
            .collect()
    }

    #[test]
    fn finds_test_modules_behind_attributes_and_visibility() {
        let source = "#[cfg(test)]\n#[allow(dead_code)]\npub(crate) mod tests;\nmod util;\n";
        assert_eq!(
            names(source),
            [("tests".to_owned(), true), ("util".to_owned(), false)]
        );
        assert_eq!(declarations(source)[0].line, 3);
    }

    #[test]
    fn reads_compound_cfgs() {
        assert!(names("#[cfg(all(test, unix))] mod a;")[0].1);
        assert!(!names("#[cfg(not(test))] mod b;")[0].1);
        assert!(!names("#[cfg(any(test, debug_assertions))] mod c;")[0].1);
        assert!(names("#[cfg(any(test, all(test, unix)))] mod d;")[0].1);
        assert!(!names("#[cfg(test = \"x\")] mod e;")[0].1);
        assert!(!names("#[cfg(feature = \"test\")] mod f;")[0].1);
        assert!(names("#[cfg( test )] unsafe mod g;")[0].1);
    }

    #[test]
    fn ignores_inline_modules_comments_and_strings() {
        let source = "#[cfg(test)] mod tests { }\n// #[cfg(test)] mod x;\nconst S: &str = \"#[cfg(test)] mod y;\";\n";
        assert_eq!(names(source), Vec::new());
    }

    #[test]
    fn reads_path_attributes() {
        let found = declarations("#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;");
        assert_eq!(found[0].path.as_deref(), Some("checks.rs"));
    }

    fn scratch(name: &str) -> std::path::PathBuf {
        let root = std::env::temp_dir().join(format!("rust-bedrock-{name}-{}", std::process::id()));
        if root.exists() {
            fs::remove_dir_all(&root).unwrap();
        }
        fs::create_dir_all(root.join("src/parser")).unwrap();
        root
    }

    #[test]
    fn finds_children_for_both_module_layouts() {
        let root = scratch("layouts");
        fs::write(root.join("src/parser/tests.rs"), "").unwrap();
        fs::write(root.join("src/tests.rs"), "").unwrap();
        let declaration = &declarations("#[cfg(test)] mod tests;")[0];
        assert_eq!(
            child_file(&root.join("src/parser.rs"), declaration).as_deref(),
            Some(root.join("src/parser/tests.rs").as_path())
        );
        assert_eq!(
            child_file(&root.join("src/lib.rs"), declaration).as_deref(),
            Some(root.join("src/tests.rs").as_path())
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn moves_the_tests_inline_and_removes_the_file() {
        let root = scratch("inline");
        let parent = root.join("src/parser.rs");
        fs::write(&parent, "fn f() {}\n\n#[cfg(test)]\nmod tests;\n").unwrap();
        fs::write(
            root.join("src/parser/tests.rs"),
            "use super::f;\n\n#[test]\nfn raw() {\n    let _ = r\"\n  kept\n\";\n}\n",
        )
        .unwrap();
        assert_eq!(inline(&parent), Vec::<String>::new());
        assert_eq!(
            fs::read_to_string(&parent).unwrap(),
            "fn f() {}\n\n#[cfg(test)]\nmod tests {\nuse super::f;\n\n#[test]\nfn raw() {\n    let _ = r\"\n  kept\n\";\n}\n}\n"
        );
        assert!(!Path::new(&root.join("src/parser")).exists());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn drops_the_path_attribute_of_an_inlined_module() {
        let root = scratch("path");
        let parent = root.join("src/parser.rs");
        fs::write(
            &parent,
            "#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;\n",
        )
        .unwrap();
        fs::write(root.join("src/checks.rs"), "#[test]\nfn t() {}\n").unwrap();
        assert_eq!(inline(&parent), Vec::<String>::new());
        assert_eq!(
            fs::read_to_string(&parent).unwrap(),
            "#[cfg(test)]\nmod tests {\n#[test]\nfn t() {}\n}\n"
        );
        assert!(!root.join("src/checks.rs").exists());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn moves_a_whole_nested_tree_in_one_run() {
        let root = scratch("tree");
        fs::create_dir_all(root.join("src/parser/tests/deep")).unwrap();
        let parent = root.join("src/parser.rs");
        fs::write(&parent, "#[cfg(test)]\nmod tests;\n").unwrap();
        fs::write(
            root.join("src/parser/tests.rs"),
            "mod fixtures;\nmod deep;\n#[path = \"extra.rs\"]\nmod more;\n",
        )
        .unwrap();
        fs::write(root.join("src/parser/tests/fixtures.rs"), "fn a() {}\n").unwrap();
        fs::write(root.join("src/parser/tests/deep.rs"), "mod leaf;\n").unwrap();
        fs::write(root.join("src/parser/tests/deep/leaf.rs"), "fn b() {}\n").unwrap();
        fs::write(root.join("src/parser/extra.rs"), "fn c() {}\n").unwrap();
        assert_eq!(inline(&parent), Vec::<String>::new());
        assert_eq!(
            fs::read_to_string(&parent).unwrap(),
            "#[cfg(test)]\nmod tests {\nmod fixtures {\nfn a() {}\n}\nmod deep {\nmod leaf {\nfn b() {}\n}\n}\nmod more {\nfn c() {}\n}\n}\n"
        );
        assert!(!root.join("src/parser").exists());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn leaves_a_tree_with_a_missing_file_and_still_moves_the_others() {
        let root = scratch("partial");
        let parent = root.join("src/parser.rs");
        fs::write(
            &parent,
            "#[cfg(test)]\nmod broken;\n#[cfg(test)]\nmod tests;\n",
        )
        .unwrap();
        fs::write(root.join("src/parser/broken.rs"), "mod missing;\n").unwrap();
        fs::write(root.join("src/parser/tests.rs"), "fn t() {}\n").unwrap();
        let problems = inline(&parent);
        assert_eq!(problems.len(), 1);
        assert!(problems[0].contains("mod missing"));
        assert!(root.join("src/parser/broken.rs").exists());
        assert!(!root.join("src/parser/tests.rs").exists());
        assert!(
            fs::read_to_string(&parent)
                .unwrap()
                .contains("mod tests {\nfn t() {}\n}")
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn resolves_a_path_attribute_from_the_directory_of_the_file() {
        let root = scratch("pathdir");
        fs::write(root.join("src/parser/checks.rs"), "").unwrap();
        fs::write(root.join("src/checks.rs"), "").unwrap();
        let declaration = &declarations("#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;")[0];
        assert_eq!(
            child_file(&root.join("src/parser.rs"), declaration).as_deref(),
            Some(root.join("src/checks.rs").as_path())
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn splices_included_test_files_and_deletes_them() {
        let root = scratch("splice");
        fs::create_dir_all(root.join("src/parser/tests/deep")).unwrap();
        let parent = root.join("src/parser.rs");
        fs::write(&parent, "#[cfg(test)]\nmod tests;\n").unwrap();
        fs::write(
            root.join("src/parser/tests.rs"),
            "include!(\"tests/one.rs\");\nfn t() {}\n",
        )
        .unwrap();
        fs::write(
            root.join("src/parser/tests/one.rs"),
            "include!(\"deep/two.rs\");\nfn one() {}\n",
        )
        .unwrap();
        fs::write(root.join("src/parser/tests/deep/two.rs"), "fn two() {}\n").unwrap();
        assert_eq!(inline(&parent), Vec::<String>::new());
        assert_eq!(
            fs::read_to_string(&parent).unwrap(),
            "#[cfg(test)]\nmod tests {\nfn two() {}\nfn one() {}\nfn t() {}\n}\n"
        );
        assert!(!root.join("src/parser").exists());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn repoints_includes_it_cannot_splice_and_reports_missing_ones() {
        let root = scratch("repoint");
        let parent = root.join("src/parser.rs");
        fs::write(&parent, "#[cfg(test)]\nmod tests;\n").unwrap();
        fs::write(
            root.join("src/parser/tests.rs"),
            "const A: &str = include_str!(\"data.txt\");\ninclude!(\"nope.rs\");\n",
        )
        .unwrap();
        fs::write(root.join("src/parser/data.txt"), "x").unwrap();
        let problems = inline(&parent);
        assert_eq!(problems.len(), 1);
        assert!(problems[0].contains("nope.rs"));
        let text = fs::read_to_string(&parent).unwrap();
        assert!(text.contains("include_str!(\"parser/data.txt\")"), "{text}");
        assert!(text.contains("include!(\"parser/nope.rs\")"), "{text}");
        assert!(root.join("src/parser/data.txt").exists());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn an_unreadable_file_is_reported_and_the_other_modules_still_move() {
        use std::os::unix::fs::PermissionsExt;

        let root = scratch("unreadable");
        let parent = root.join("src/parser.rs");
        fs::write(
            &parent,
            "#[cfg(test)]\nmod locked;\n#[cfg(test)]\nmod tests;\n",
        )
        .unwrap();
        let locked = root.join("src/parser/locked.rs");
        fs::write(&locked, "fn l() {}\n").unwrap();
        fs::set_permissions(&locked, fs::Permissions::from_mode(0o000)).unwrap();
        fs::write(root.join("src/parser/tests.rs"), "fn t() {}\n").unwrap();
        let problems = inline(&parent);
        fs::set_permissions(&locked, fs::Permissions::from_mode(0o644)).unwrap();
        assert_eq!(problems.len(), 1);
        assert!(problems[0].contains("locked.rs") && problems[0].contains("read"));
        assert!(
            fs::read_to_string(&parent)
                .unwrap()
                .contains("mod tests {\nfn t() {}\n}")
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn splices_a_top_level_include_of_a_test_file() {
        let root = scratch("toplevel");
        fs::create_dir_all(root.join("src/parser/tests")).unwrap();
        let parent = root.join("src/parser.rs");
        fs::write(
            &parent,
            "fn f() {}\ninclude!(\"parser/tests/a.rs\");\ninclude!(\"parser/data.rs\");\n",
        )
        .unwrap();
        fs::write(root.join("src/parser/tests/a.rs"), "#[test]\nfn a() {}\n").unwrap();
        fs::write(root.join("src/parser/data.rs"), "").unwrap();
        assert_eq!(inline(&parent), Vec::<String>::new());
        assert_eq!(
            fs::read_to_string(&parent).unwrap(),
            "fn f() {}\n#[test]\nfn a() {}\ninclude!(\"parser/data.rs\");\n"
        );
        assert!(root.join("src/parser/data.rs").exists());
        assert!(!root.join("src/parser/tests").exists());
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn a_nested_module_never_takes_a_same_named_file_from_the_parent_directory() {
        let root = scratch("samename");
        fs::create_dir_all(root.join("src/parser/tests/npm")).unwrap();
        let parent = root.join("src/parser.rs");
        fs::write(&parent, "#[cfg(test)]\nmod tests;\n").unwrap();
        fs::write(root.join("src/parser/tests.rs"), "mod npm;\nmod smoke;\n").unwrap();
        fs::write(root.join("src/parser/tests/npm.rs"), "mod smoke;\n").unwrap();
        fs::write(
            root.join("src/parser/tests/npm/smoke.rs"),
            "fn inner() {}\n",
        )
        .unwrap();
        fs::write(root.join("src/parser/tests/smoke.rs"), "fn outer() {}\n").unwrap();
        assert_eq!(inline(&parent), Vec::<String>::new());
        assert_eq!(
            fs::read_to_string(&parent).unwrap(),
            "#[cfg(test)]\nmod tests {\nmod npm {\nmod smoke {\nfn inner() {}\n}\n}\nmod smoke {\nfn outer() {}\n}\n}\n"
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn repointed_includes_do_not_pass_through_a_removed_directory() {
        let root = scratch("normal");
        fs::create_dir_all(root.join("src/parser/tests")).unwrap();
        let parent = root.join("src/parser.rs");
        fs::write(&parent, "#[cfg(test)]\nmod tests;\n").unwrap();
        fs::write(
            root.join("src/parser/tests.rs"),
            "const D: &str = include_str!(\"../../data.txt\");\n",
        )
        .unwrap();
        fs::write(root.join("data.txt"), "x").unwrap();
        assert_eq!(inline(&parent), Vec::<String>::new());
        assert_eq!(
            fs::read_to_string(&parent).unwrap(),
            "#[cfg(test)]\nmod tests {\nconst D: &str = include_str!(\"../data.txt\");\n}\n"
        );
        fs::remove_dir_all(root).unwrap();
    }
}
