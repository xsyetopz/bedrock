use super::{Source, conflicting, gate_integration_test, mod_files, toolchain, update_lock};
use crate::modules::{child_file, modules};
use std::fs;
use std::path::Path;
use std::process::Command;

fn run(dir: &Path, program: &str, args: &[&str]) {
    let output = Command::new(program)
        .args(args)
        .current_dir(dir)
        .output()
        .expect("the command starts");
    assert!(
        output.status.success(),
        "{program} {args:?}: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn locked(root: &Path, name: &str) -> String {
    let lock = fs::read_to_string(root.join("Cargo.lock")).expect("Cargo.lock exists");
    let (_, rest) = lock
        .split_once(&format!("name = \"{name}\"\nversion = \""))
        .expect("the package is locked");
    rest.split('"').next().unwrap_or_default().to_owned()
}

#[test]
fn names_the_packages_cargo_cannot_select() {
    let stderr = "error: failed to select a version for `toml_edit`.\n  required by x\nfailed to select a version for `serde_json` which could resolve this conflict\n";
    assert_eq!(conflicting(stderr), ["toml_edit", "serde_json"]);
    assert_eq!(conflicting("error: no matching package").len(), 0);
}

#[test]
fn unlocks_only_the_pins_a_new_git_dependency_conflicts_with() {
    let base = std::env::temp_dir().join(format!("rust-bedrock-lock-{}", std::process::id()));
    drop(fs::remove_dir_all(&base));
    let dep = base.join("dep");
    let root = base.join("target");
    fs::create_dir_all(dep.join("src")).expect("dep dir");
    fs::create_dir_all(root.join("app/src")).expect("app dir");
    fs::write(
            dep.join("Cargo.toml"),
            "[package]\nname = \"needs-newer\"\nversion = \"0.1.0\"\nedition = \"2024\"\n\n[dependencies]\nserde_json = \"1.0.154\"\n",
        )
        .expect("dep manifest");
    fs::write(dep.join("src/lib.rs"), "").expect("dep source");
    run(&dep, "git", &["init", "--quiet"]);
    run(&dep, "git", &["add", "."]);
    run(
        &dep,
        "git",
        &[
            "-c",
            "user.name=t",
            "-c",
            "user.email=t@example.com",
            "commit",
            "--quiet",
            "-m",
            "dep",
        ],
    );
    fs::write(
        root.join("Cargo.toml"),
        "[workspace]\nmembers = [\"app\"]\nresolver = \"3\"\n",
    )
    .expect("root manifest");
    fs::write(
            root.join("app/Cargo.toml"),
            "[package]\nname = \"app\"\nversion = \"0.1.0\"\nedition = \"2024\"\n\n[dependencies]\nserde = \"1\"\nserde_json = \"1.0.150\"\n",
        )
        .expect("app manifest");
    fs::write(root.join("app/src/lib.rs"), "").expect("app source");
    run(&root, "cargo", &["generate-lockfile", "--quiet"]);
    run(
        &root,
        "cargo",
        &["update", "--quiet", "serde_json", "--precise", "1.0.150"],
    );
    run(
        &root,
        "cargo",
        &["update", "--quiet", "serde", "--precise", "1.0.228"],
    );

    fs::create_dir_all(root.join("xtask/src")).expect("xtask dir");
    fs::write(
            root.join("xtask/Cargo.toml"),
            format!(
                "[package]\nname = \"xtask\"\nversion = \"0.0.0\"\nedition = \"2024\"\n\n[dependencies]\nneeds-newer = {{ git = \"file://{}\" }}\n",
                dep.display()
            ),
        )
        .expect("xtask manifest");
    fs::write(root.join("xtask/src/main.rs"), "fn main() {}\n").expect("xtask source");
    fs::write(
        root.join("Cargo.toml"),
        "[workspace]\nmembers = [\"app\", \"xtask\"]\nresolver = \"3\"\n",
    )
    .expect("root manifest");

    update_lock(&root).expect("the lock takes the new dependency");

    assert_eq!(locked(&root, "serde_json"), "1.0.154");
    assert_eq!(locked(&root, "serde"), "1.0.228");
    assert_eq!(locked(&root, "needs-newer"), "0.1.0");
    drop(fs::remove_dir_all(&base));
}

#[test]
fn keeps_project_targets_in_the_toolchain() {
    let old = "[toolchain]\nchannel = \"1.80\"\ntargets = [\"thumbv6m-none-eabi\"]\n";
    let updated = toolchain(Some(old), &["wasm32-unknown-unknown"]);
    assert!(updated.contains("channel = \"1.99.1\""));
    assert!(updated.contains("targets = [\"thumbv6m-none-eabi\", \"wasm32-unknown-unknown\"]"));
    assert!(!toolchain(None, &[]).contains("targets"));
}

#[test]
fn gates_integration_tests_after_their_docs() {
    assert_eq!(
        gate_integration_test("//! Checks the CLI.\nuse x;\n").as_deref(),
        Some("//! Checks the CLI.\n#![cfg(test)]\n\nuse x;\n")
    );
    assert_eq!(
        gate_integration_test("use x;\n").as_deref(),
        Some(
            "//! Integration tests run against the public API, as another crate would.\n#![cfg(test)]\n\nuse x;\n"
        )
    );
    assert_eq!(gate_integration_test("#![cfg(test)]\n"), None);
}

#[test]
fn pins_xtask_to_this_release() {
    let manifest = Source::Release.xtask_manifest();
    assert!(manifest.contains(&format!(
        "tag = \"rust-bedrock-v{}\"",
        env!("CARGO_PKG_VERSION")
    )));
    assert!(
        Source::Path("../bedrock".to_owned())
            .xtask_manifest()
            .contains("path = \"../bedrock\"")
    );
}

#[test]
fn a_mod_rs_rename_keeps_its_path_attributes_and_includes_pointing_at_the_same_files() {
    let root = std::env::temp_dir().join(format!("rust-bedrock-modrs-{}", std::process::id()));
    if root.exists() {
        fs::remove_dir_all(&root).unwrap();
    }
    let src = root.join("src");
    fs::create_dir_all(src.join("session/matrix")).unwrap();
    fs::create_dir_all(src.join("odd")).unwrap();
    fs::write(
        src.join("session/mod.rs"),
        "#[cfg(test)]\n#[path = \"matrix/tests.rs\"]\nmod matrix;\ninclude!(\"data.rs\");\n",
    )
    .unwrap();
    fs::write(src.join("session/matrix/tests.rs"), "").unwrap();
    fs::write(src.join("session/data.rs"), "").unwrap();
    fs::write(
        src.join("odd/mod.rs"),
        "include!(concat!(env!(\"OUT_DIR\"), \"/x.rs\"));",
    )
    .unwrap();
    let mut moves = Vec::new();
    mod_files(&src, &mut moves).unwrap();
    assert_eq!(moves.len(), 1, "the file it cannot rewrite is skipped");
    let (from, to, content) = moves.remove(0);
    assert_eq!(to, src.join("session.rs"));
    fs::rename(&from, &to).unwrap();
    fs::write(&to, content.expect("the paths change")).unwrap();
    let text = fs::read_to_string(&to).unwrap();
    let module = modules(&text).remove(0);
    assert_eq!(
        child_file(&to, &module),
        Some(src.join("session/matrix/tests.rs"))
    );
    assert!(
        src.join("session/data.rs").is_file() && text.contains("include!(\"session/data.rs\")")
    );
    fs::remove_dir_all(root).unwrap();
}
