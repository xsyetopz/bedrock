//! Edits to Cargo manifests that keep their formatting and comments.

use toml_edit::{Array, DocumentMut, Item, Table, TomlError, value};

use crate::files::WORKSPACE;

/// Dev-dependencies whose job the justfile and `.githooks` took over.
const REPLACED_DEV_DEPENDENCIES: &[&str] = &["cargo-husky"];

fn managed() -> DocumentMut {
    WORKSPACE
        .parse()
        .expect("files/rust/workspace.toml is valid TOML, which the tests check")
}

/// Points the package at the workspace lints, which replace its own.
fn use_workspace_lints(document: &mut DocumentMut) {
    let mut lints = Table::new();
    lints.insert("workspace", value(true));
    document.insert("lints", Item::Table(lints));
    if let Some(dev) = document
        .get_mut("dev-dependencies")
        .and_then(Item::as_table_like_mut)
    {
        for name in REPLACED_DEV_DEPENDENCIES {
            dev.remove(name);
        }
        if dev.is_empty() {
            document.remove("dev-dependencies");
        }
    }
}

/// The member manifest with the workspace lints in place of its own.
pub(crate) fn member(current: &str) -> Result<String, TomlError> {
    let mut document: DocumentMut = current.parse()?;
    use_workspace_lints(&mut document);
    Ok(document.to_string())
}

/// The root manifest with the managed lints, resolver, release profile, and the
/// `xtask` member. A single-package root gains a `[workspace]` table.
pub(crate) fn root(current: &str) -> Result<String, TomlError> {
    let mut document: DocumentMut = current.parse()?;
    let managed = managed();
    if document.contains_key("package") {
        use_workspace_lints(&mut document);
    }
    let workspace = document
        .entry("workspace")
        .or_insert_with(|| Item::Table(Table::new()))
        .as_table_mut()
        .expect("[workspace] is a table in a manifest Cargo accepts");
    workspace.insert("resolver", value("3"));
    let members = workspace
        .entry("members")
        .or_insert_with(|| value(Array::new()))
        .as_array_mut()
        .expect("workspace.members is an array in a manifest Cargo accepts");
    if !members
        .iter()
        .any(|member| member.as_str() == Some("xtask"))
    {
        members.push("xtask");
    }
    let kept = workspace
        .get("lints")
        .and_then(Item::as_table)
        .and_then(first_position);
    let mut lints = managed["workspace"]["lints"].clone();
    workspace.insert("lints", Item::None);
    let end = last_position(document.as_table()) + 1;
    if let Some(table) = lints.as_table_mut() {
        place(table, kept.unwrap_or(end));
    }
    document["workspace"]["lints"] = lints;
    let release = document
        .entry("profile")
        .or_insert_with(implicit_table)
        .as_table_mut()
        .and_then(|profile| {
            profile
                .entry("release")
                .or_insert_with(|| {
                    let mut table = Table::new();
                    table.set_position(Some(end + 1));
                    Item::Table(table)
                })
                .as_table_mut()
        })
        .expect("[profile.release] is a table in a manifest Cargo accepts");
    for (key, setting) in managed["profile"]["release"]
        .as_table()
        .into_iter()
        .flat_map(Table::iter)
    {
        release.insert(key, setting.clone());
    }
    Ok(document.to_string())
}

/// Tables keep the parse positions of the document they came from, so moved
/// tables need new ones to print where they belong.
fn place(table: &mut Table, position: isize) {
    table.set_position(Some(position));
    for (_, item) in table.iter_mut() {
        if let Some(child) = item.as_table_mut() {
            place(child, position);
        }
    }
}

fn first_position(table: &Table) -> Option<isize> {
    table
        .iter()
        .filter_map(|(_, item)| item.as_table())
        .filter_map(|child| child.position().or_else(|| first_position(child)))
        .min()
}

fn last_position(table: &Table) -> isize {
    table
        .iter()
        .filter_map(|(_, item)| item.as_table())
        .map(|child| child.position().unwrap_or(0).max(last_position(child)))
        .max()
        .unwrap_or(0)
}

fn implicit_table() -> Item {
    let mut table = Table::new();
    table.set_implicit(true);
    Item::Table(table)
}

/// Adds the `bedrock` alias to `.cargo/config.toml`, keeping its other settings.
pub(crate) fn cargo_config(current: Option<&str>, alias: &str) -> Result<String, TomlError> {
    let mut document: DocumentMut = current.unwrap_or_default().parse()?;
    let managed: DocumentMut = alias.parse()?;
    let aliases = document
        .entry("alias")
        .or_insert_with(implicit_table)
        .as_table_mut()
        .expect("[alias] is a table in a config Cargo accepts");
    for (key, command) in managed["alias"]
        .as_table()
        .into_iter()
        .flat_map(Table::iter)
    {
        aliases.insert(key, command.clone());
    }
    Ok(document.to_string())
}

/// Every `edition` value in the manifest, at the package and workspace level.
pub(crate) fn editions(current: &str) -> Vec<String> {
    let Ok(document) = current.parse::<DocumentMut>() else {
        return Vec::new();
    };
    [
        document
            .get("package")
            .and_then(|package| package.get("edition")),
        document
            .get("workspace")
            .and_then(|workspace| workspace.get("package"))
            .and_then(|package| package.get("edition")),
    ]
    .into_iter()
    .flatten()
    .filter_map(|edition| edition.as_str().map(str::to_owned))
    .collect()
}

/// Moves every literal `edition` from `from` to `to`.
pub(crate) fn bump_edition(current: &str, from: &str, to: &str) -> Result<String, TomlError> {
    let mut document: DocumentMut = current.parse()?;
    let step = |package: Option<&mut Item>| {
        if let Some(edition) = package
            .and_then(Item::as_table_like_mut)
            .and_then(|package| package.get_mut("edition"))
            && edition.as_str() == Some(from)
        {
            *edition = value(to);
        }
    };
    step(document.get_mut("package"));
    step(
        document
            .as_table_mut()
            .get_mut("workspace")
            .and_then(Item::as_table_like_mut)
            .and_then(|workspace| workspace.get_mut("package")),
    );
    Ok(document.to_string())
}

#[cfg(test)]
mod tests {
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
}
