//! Finds `#[cfg(test)] mod name;` declarations, whose tests live in another
//! file, and moves those tests inline as `mod name { … }`.

use std::fs;
use std::path::{Path, PathBuf};

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
    let parent = file
        .parent()
        .and_then(Path::file_name)
        .and_then(|name| name.to_str())
        .unwrap_or_default();
    matches!(name, "lib.rs" | "main.rs" | "mod.rs" | "build.rs")
        || matches!(parent, "bin" | "tests" | "examples" | "benches")
}

/// Returns the file that holds the body of `declaration`, declared in `file`.
pub(crate) fn child_file(file: &Path, declaration: &OutOfLine) -> Option<PathBuf> {
    let directory = file.parent().unwrap_or_else(|| Path::new(""));
    if let Some(path) = &declaration.path {
        return Some(directory.join(path)).filter(|candidate| candidate.is_file());
    }
    let nested = directory.join(file.file_stem().unwrap_or_default());
    let mut bases = [directory.to_path_buf(), nested];
    if !owns_directory(file) {
        bases.reverse();
    }
    bases.iter().find_map(|base| {
        [
            base.join(format!("{}.rs", declaration.name)),
            base.join(&declaration.name).join("mod.rs"),
        ]
        .into_iter()
        .find(|candidate| candidate.is_file())
    })
}

/// Inlines the out-of-line modules of `source`, the text of `file`, depth first so
/// a module's own children move into it before it moves. `nested` is set inside a
/// test module, where every `mod name;` is test code. A module whose tree cannot
/// all move is left alone, with its problems; the other modules still move.
/// Appends the files it emptied to `moved`, leaves before the files that held them.
fn expand(
    file: &Path,
    source: &mut String,
    nested: bool,
    problems: &mut Vec<String>,
    moved: &mut Vec<PathBuf>,
) -> std::io::Result<()> {
    for declaration in declarations(source)
        .into_iter()
        .rev()
        .filter(|found| nested || found.test_only)
    {
        let location = format!("{}:{}", file.display(), declaration.line);
        let Some(child) = child_file(file, &declaration) else {
            problems.push(format!(
                "{location}: cannot find the file for mod {}",
                declaration.name
            ));
            continue;
        };
        let mut body = fs::read_to_string(&child)?;
        let mut inner_problems = Vec::new();
        let mut inner_moved = Vec::new();
        expand(
            &child,
            &mut body,
            true,
            &mut inner_problems,
            &mut inner_moved,
        )?;
        if !inner_problems.is_empty() {
            problems.extend(inner_problems);
            continue;
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
        moved.extend(inner_moved);
        moved.push(child);
    }
    Ok(())
}

/// Moves every out-of-line test module of `file` inline, with the modules inside
/// them. Returns one message per module it could not move.
pub(crate) fn inline(file: &Path) -> std::io::Result<Vec<String>> {
    let mut source = fs::read_to_string(file)?;
    let mut problems = Vec::new();
    let mut moved = Vec::new();
    expand(file, &mut source, false, &mut problems, &mut moved)?;
    if !moved.is_empty() {
        fs::write(file, source)?;
        for child in moved {
            fs::remove_file(&child)?;
            let mut directory = child.parent();
            while let Some(parent) = directory
                && fs::read_dir(parent)?.next().is_none()
            {
                fs::remove_dir(parent)?;
                directory = parent.parent();
            }
        }
    }
    Ok(problems)
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
        assert_eq!(inline(&parent).unwrap(), Vec::<String>::new());
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
        assert_eq!(inline(&parent).unwrap(), Vec::<String>::new());
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
        assert_eq!(inline(&parent).unwrap(), Vec::<String>::new());
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
        let problems = inline(&parent).unwrap();
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
}
