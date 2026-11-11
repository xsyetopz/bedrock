use std::fs;

use super::{Step, pending};

#[test]
fn keeps_what_it_replaces_and_skips_what_matches() {
    let root = std::env::temp_dir().join(format!("rust-bedrock-plan-{}", std::process::id()));
    if root.exists() {
        fs::remove_dir_all(&root).unwrap();
    }
    fs::create_dir_all(&root).unwrap();
    fs::write(root.join("a.toml"), "old").unwrap();
    fs::write(root.join("same"), "x").unwrap();
    fs::write(root.join("Makefile"), "all:").unwrap();
    fs::write(root.join("mod.rs"), "fn a() {}").unwrap();
    let steps = pending(
        &root,
        vec![
            Step::write("a.toml", "new"),
            Step::write("same", "x"),
            Step::Remove {
                path: "Makefile".to_owned(),
            },
            Step::Link {
                path: "CLAUDE.md".to_owned(),
                target: "AGENTS.md",
            },
            Step::Move {
                from: "mod.rs".to_owned(),
                path: "a.rs".to_owned(),
                content: None,
            },
        ],
    );
    assert_eq!(steps.len(), 4);
    for step in &steps {
        step.apply(&root).unwrap();
    }
    assert_eq!(fs::read_to_string(root.join("a.toml")).unwrap(), "new");
    assert_eq!(fs::read_to_string(root.join("a.toml.bak")).unwrap(), "old");
    assert!(!root.join("Makefile").exists());
    assert!(root.join("Makefile.bak").exists());
    assert_eq!(fs::read_to_string(root.join("a.rs")).unwrap(), "fn a() {}");
    assert!(!root.join("mod.rs.bak").exists());
    assert_eq!(
        fs::read_link(root.join("CLAUDE.md")).unwrap().to_str(),
        Some("AGENTS.md")
    );
    assert_eq!(pending(&root, steps), []);
    Step::write("a.toml", "newer").apply(&root).unwrap();
    assert_eq!(fs::read_to_string(root.join("a.toml.bak")).unwrap(), "old");
    fs::remove_dir_all(root).unwrap();
}
