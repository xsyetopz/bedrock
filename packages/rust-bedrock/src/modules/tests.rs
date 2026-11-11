use std::fs;

use super::{child_file, modules};

fn names(source: &str) -> Vec<(String, bool)> {
    modules(source)
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
    assert_eq!(modules(source)[0].line, 3);
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
fn reads_inline_bodies_and_depth_and_skips_comments_and_strings() {
    let source = "// #[cfg(test)] mod x;\nconst S: &str = \"mod y;\";\nmod outer {\n    mod inner { }\n}\nmod after;\n";
    let found = modules(source);
    let seen: Vec<_> = found
        .iter()
        .map(|module| (module.name.as_str(), module.depth, module.body.is_some()))
        .collect();
    assert_eq!(
        seen,
        [("outer", 0, true), ("inner", 1, true), ("after", 0, false)]
    );
    let (open, close) = found[0].body.unwrap();
    assert_eq!(&source[open..=open], "{");
    assert_eq!(&source[close..=close], "}");
}

#[test]
fn reads_path_attributes() {
    let source = "#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;";
    let found = modules(source);
    assert_eq!(found[0].path.as_deref(), Some("checks.rs"));
    let (from, to) = found[0].path_attribute.unwrap();
    assert_eq!(&source[from..to], "#[path = \"checks.rs\"]");
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
    let declaration = &modules("#[cfg(test)] mod tests;")[0];
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
fn resolves_a_path_attribute_from_the_directory_of_the_file() {
    let root = scratch("pathdir");
    fs::write(root.join("src/parser/checks.rs"), "").unwrap();
    fs::write(root.join("src/checks.rs"), "").unwrap();
    let declaration = &modules("#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;")[0];
    assert_eq!(
        child_file(&root.join("src/parser.rs"), declaration).as_deref(),
        Some(root.join("src/checks.rs").as_path())
    );
    fs::remove_dir_all(root).unwrap();
}
