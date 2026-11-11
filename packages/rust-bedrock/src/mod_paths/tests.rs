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
