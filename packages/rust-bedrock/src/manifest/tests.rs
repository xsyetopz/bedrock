use super::{bump_edition, cargo_config, editions, member, root};
use crate::files::CARGO_CONFIG;

#[test]
fn turns_a_single_package_into_a_workspace_with_the_managed_lints() {
    let current = "[package]\nname = \"app\" # the binary\nedition = \"2024\"\n\n[lints.clippy]\npedantic = \"warn\"\n\n[dev-dependencies]\ncargo-husky = \"1\"\n\n[profile.release]\ndebug = true\n";
    let updated = root(current).unwrap();
    assert!(updated.contains("name = \"app\" # the binary"));
    assert!(updated.contains("[lints]\nworkspace = true"));
    assert!(!updated.contains("pedantic = \"warn\""));
    assert!(!updated.contains("cargo-husky"));
    assert!(!updated.contains("[dev-dependencies]"));
    assert!(updated.contains("members = [\"xtask\"]"));
    assert!(updated.contains("debug = true"));
    assert!(updated.contains("lto = \"thin\""));
    assert!(updated.contains("allow_attributes = \"deny\""));
    assert_eq!(root(&updated).unwrap(), updated);
}

#[test]
fn keeps_release_profile_keys_the_repo_already_set() {
    let current = "[package]\nname = \"app\"\nedition = \"2024\"\n\n[profile.release]\npanic = \"unwind\"\nlto = \"fat\"\n";
    let updated = root(current).unwrap();
    assert!(updated.contains("panic = \"unwind\""));
    assert!(updated.contains("lto = \"fat\""));
    assert!(!updated.contains("panic = \"abort\""));
    assert!(!updated.contains("lto = \"thin\""));
    assert_eq!(root(&updated).unwrap(), updated);
}

#[test]
fn adds_xtask_to_existing_members_once() {
    let updated = root("[workspace]\nmembers = [\"crates/*\"]\n").unwrap();
    assert!(updated.contains("members = [\"crates/*\", \"xtask\"]"));
    assert_eq!(root(&updated).unwrap(), updated);
}

#[test]
fn puts_the_lints_after_the_existing_workspace_tables() {
    let current = "[workspace]\nmembers = []\n\n[workspace.package]\nedition = \"2024\"\n";
    let updated = root(current).unwrap();
    let package = updated.find("[workspace.package]").unwrap();
    let rust = updated.find("[workspace.lints.rust]").unwrap();
    let clippy = updated.find("[workspace.lints.clippy]").unwrap();
    let release = updated.find("[profile.release]").unwrap();
    assert!(package < rust && rust < clippy && clippy < release);
    assert_eq!(root(&updated).unwrap(), updated);
}

#[test]
fn replaces_member_lints() {
    let updated =
        member("[package]\nname = \"a\"\n\n[lints.rust]\nunsafe_code = \"forbid\"\n").unwrap();
    assert_eq!(
        updated,
        "[package]\nname = \"a\"\n\n[lints]\nworkspace = true\n"
    );
}

#[test]
fn merges_the_alias_into_cargo_config() {
    let updated = cargo_config(Some("[build]\njobs = 4\n"), CARGO_CONFIG).unwrap();
    assert!(updated.starts_with("[build]\njobs = 4\n"));
    assert!(updated.contains("bedrock = \"run --quiet --package xtask --\""));
    assert_eq!(cargo_config(None, CARGO_CONFIG).unwrap(), CARGO_CONFIG);
}

#[test]
fn steps_editions_forward() {
    let current = "[package]\nedition = \"2018\"\n\n[workspace.package]\nedition = \"2018\"\n";
    assert_eq!(editions(current), ["2018", "2018"]);
    let bumped = bump_edition(current, "2018", "2021").unwrap();
    assert_eq!(editions(&bumped), ["2021", "2021"]);
    let single = bump_edition(
        "[package]\nedition = \"2021\"\n\n[workspace]\n",
        "2021",
        "2024",
    )
    .unwrap();
    assert_eq!(single, "[package]\nedition = \"2024\"\n\n[workspace]\n");
}
