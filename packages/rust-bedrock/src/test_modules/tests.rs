use std::fs;
use std::path::PathBuf;

use super::{inline_tests, move_out, path_tests, tests_file};

fn scratch(name: &str) -> PathBuf {
    let root = std::env::temp_dir().join(format!("rust-bedrock-{name}-{}", std::process::id()));
    if root.exists() {
        fs::remove_dir_all(&root).unwrap();
    }
    fs::create_dir_all(root.join("src")).unwrap();
    root
}

fn found(source: &str) -> Vec<(String, usize)> {
    inline_tests(source)
        .into_iter()
        .map(|module| (module.name, module.line))
        .collect()
}

#[test]
fn finds_each_inline_test_module_with_its_line() {
    let source = "fn f() {}\n\n#[cfg(test)]\n#[allow(dead_code)]\nmod tests {\n    fn t() {}\n}\n#[cfg(test)]\npub(crate) mod extra { }\n";
    assert_eq!(
        found(source),
        [("tests".to_owned(), 5), ("extra".to_owned(), 9)]
    );
}

#[test]
fn allows_the_out_of_line_form_and_ignores_other_modules() {
    let source = "#[cfg(test)]\nmod tests;\nmod util { }\n#[cfg(not(test))]\nmod live { }\n// #[cfg(test)] mod x { }\nconst S: &str = \"#[cfg(test)] mod y { }\";\n";
    assert_eq!(found(source), Vec::new());
}

#[test]
fn reads_compound_cfgs() {
    assert_eq!(found("#[cfg(all(test, unix))] mod a { }").len(), 1);
    assert_eq!(
        found("#[cfg(any(test, all(test, unix)))] mod d { }").len(),
        1
    );
    assert_eq!(
        found("#[cfg(any(test, debug_assertions))] mod c { }").len(),
        0
    );
    assert_eq!(found("#[cfg(feature = \"test\")] mod f { }").len(), 0);
}

#[test]
fn puts_the_file_beside_the_roots_and_in_a_directory_for_other_modules() {
    let root = scratch("place");
    assert_eq!(
        tests_file(&root.join("src/foo.rs")),
        root.join("src/foo/tests.rs")
    );
    for name in ["lib.rs", "main.rs", "mod.rs"] {
        assert_eq!(
            tests_file(&root.join("src").join(name)),
            root.join("src/tests.rs")
        );
    }
    fs::remove_dir_all(root).unwrap();
}

#[test]
fn moves_the_body_out_and_leaves_the_declaration() {
    let root = scratch("move");
    let parent = root.join("src/foo.rs");
    fs::write(
        &parent,
        "fn f() {}\n\n#[cfg(test)]\nmod tests {\n    use super::f;\n\n    #[test]\n    fn t() {\n        f();\n    }\n}\n",
    )
    .unwrap();
    assert_eq!(move_out(&parent), Vec::<String>::new());
    assert_eq!(
        fs::read_to_string(&parent).unwrap(),
        "fn f() {}\n\n#[cfg(test)]\nmod tests;\n"
    );
    assert_eq!(
        fs::read_to_string(root.join("src/foo/tests.rs")).unwrap(),
        "    use super::f;\n\n    #[test]\n    fn t() {\n        f();\n    }\n"
    );
    assert!(inline_tests(&fs::read_to_string(&parent).unwrap()).is_empty());
    fs::remove_dir_all(root).unwrap();
}

#[test]
fn moves_the_body_of_a_crate_root_beside_it() {
    let root = scratch("root");
    let parent = root.join("src/lib.rs");
    fs::write(&parent, "#[cfg(test)]\nmod tests {\n    fn t() {}\n}\n").unwrap();
    assert_eq!(move_out(&parent), Vec::<String>::new());
    assert!(root.join("src/tests.rs").is_file());
    fs::remove_dir_all(root).unwrap();
}

#[test]
fn refuses_what_it_cannot_move_and_changes_nothing() {
    let root = scratch("refuse");
    let parent = root.join("src/foo.rs");
    let source =
        "#[cfg(test)]\nmod checks { }\nmod inner {\n    #[cfg(test)]\n    mod tests { }\n}\n";
    fs::write(&parent, source).unwrap();
    let problems = move_out(&parent);
    assert_eq!(problems.len(), 2, "{problems:?}");
    assert!(problems[0].contains(":2:") && problems[0].contains("checks"));
    assert_eq!(fs::read_to_string(&parent).unwrap(), source);
    assert!(!root.join("src/foo").exists());

    let taken = root.join("src/bar.rs");
    fs::create_dir_all(root.join("src/bar")).unwrap();
    fs::write(root.join("src/bar/tests.rs"), "// mine\n").unwrap();
    fs::write(&taken, "#[cfg(test)]\nmod tests { }\n").unwrap();
    assert_eq!(move_out(&taken).len(), 1);
    assert_eq!(
        fs::read_to_string(root.join("src/bar/tests.rs")).unwrap(),
        "// mine\n"
    );
    fs::remove_dir_all(root).unwrap();
}

#[test]
fn finds_a_path_attribute_on_a_test_module() {
    let source = "#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;\n#[path = \"live.rs\"]\nmod live;\n#[cfg(test)]\nmod plain;\n";
    let found: Vec<_> = path_tests(source)
        .into_iter()
        .map(|module| (module.name, module.line))
        .collect();
    assert_eq!(found, [("tests".to_owned(), 3)]);
}

#[test]
fn drops_the_path_attribute_and_moves_the_file_to_the_default_place() {
    let root = scratch("path");
    let parent = root.join("src/foo.rs");
    fs::write(
        &parent,
        "#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;\n",
    )
    .unwrap();
    fs::write(root.join("src/checks.rs"), "fn t() {}\n").unwrap();
    assert_eq!(move_out(&parent), Vec::<String>::new());
    assert_eq!(
        fs::read_to_string(&parent).unwrap(),
        "#[cfg(test)]\nmod tests;\n"
    );
    assert_eq!(
        fs::read_to_string(root.join("src/foo/tests.rs")).unwrap(),
        "fn t() {}\n"
    );
    assert!(!root.join("src/checks.rs").exists());
    fs::remove_dir_all(root).unwrap();
}

#[test]
fn only_drops_the_path_attribute_when_the_file_already_sits_in_the_default_place() {
    let root = scratch("pathhome");
    let parent = root.join("src/foo.rs");
    fs::create_dir_all(root.join("src/foo")).unwrap();
    fs::write(
        &parent,
        "#[cfg(test)]\n#[path = \"foo/tests.rs\"]\nmod tests;\n",
    )
    .unwrap();
    fs::write(root.join("src/foo/tests.rs"), "fn t() {}\n").unwrap();
    assert_eq!(move_out(&parent), Vec::<String>::new());
    assert_eq!(
        fs::read_to_string(&parent).unwrap(),
        "#[cfg(test)]\nmod tests;\n"
    );
    assert_eq!(
        fs::read_to_string(root.join("src/foo/tests.rs")).unwrap(),
        "fn t() {}\n"
    );
    fs::remove_dir_all(root).unwrap();
}

#[test]
fn leaves_a_path_attribute_it_cannot_move_safely() {
    let root = scratch("pathkeep");
    let parent = root.join("src/foo.rs");
    let source = "#[cfg(test)]\n#[path = \"checks.rs\"]\nmod tests;\n";
    fs::write(&parent, source).unwrap();
    fs::write(
        root.join("src/checks.rs"),
        "const D: &str = include_str!(\"data.txt\");\n",
    )
    .unwrap();
    assert_eq!(move_out(&parent).len(), 1, "a relative include would break");
    assert_eq!(fs::read_to_string(&parent).unwrap(), source);
    assert!(root.join("src/checks.rs").is_file());

    fs::write(root.join("src/checks.rs"), "fn t() {}\n").unwrap();
    fs::create_dir_all(root.join("src/foo")).unwrap();
    fs::write(root.join("src/foo/tests.rs"), "// mine\n").unwrap();
    assert_eq!(move_out(&parent).len(), 1, "it does not overwrite a file");
    assert_eq!(fs::read_to_string(&parent).unwrap(), source);
    fs::remove_dir_all(root).unwrap();
}
