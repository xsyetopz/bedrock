//! Plans the files and manifest edits that put a repository on the bedrock setup.

use std::fs;
use std::io;
use std::path::{Path, PathBuf};
use std::process::Command;

use toml_edit::{Array, DocumentMut, value};

use crate::blocks::{Syntax, upsert};
use crate::files::{
    AGENTS, BASE, CARGO_CONFIG, DEPENDABOT, EDITORCONFIG, GITATTRIBUTES, GITIGNORE, MARKDOWN,
    Managed, RUST, STARTERS, TOOLCHAIN,
};
use crate::manifest;
use crate::plan::Step;

/// Files whose job a managed file took over: hook managers, the old toolchain and
/// formatter file names, and Make.
const REPLACED: &[&str] = &[
    ".cargo/config",
    ".clippy.toml",
    ".husky",
    ".lefthook.yml",
    ".pre-commit-config.yaml",
    ".rustfmt.toml",
    ".simple-git-hooks.json",
    "Makefile",
    "lefthook.yaml",
    "lefthook.yml",
    "rust-toolchain",
];

/// Where xtask gets rust-bedrock from.
#[derive(Debug)]
pub(crate) enum Source {
    /// The tagged release of this version.
    Release,
    /// A local checkout, as a path relative to `xtask/`.
    Path(String),
}

impl Source {
    pub(crate) fn xtask_manifest(&self) -> String {
        let dependency = match self {
            Self::Release => format!(
                "{{ git = \"{}\", tag = \"rust-bedrock-v{}\" }}",
                env!("CARGO_PKG_REPOSITORY"),
                env!("CARGO_PKG_VERSION")
            ),
            Self::Path(path) => format!("{{ path = \"{path}\" }}"),
        };
        format!(
            "[package]\nname = \"xtask\"\nversion = \"0.0.0\"\ndescription = \"Runs repository tasks: cargo bedrock guard.\"\nedition = \"2024\"\npublish = false\n\n[dependencies]\nrust-bedrock = {dependency}\n\n[lints]\nworkspace = true\n"
        )
    }
}

#[derive(Debug, serde::Deserialize)]
struct Metadata {
    packages: Vec<Package>,
}

#[derive(Debug, serde::Deserialize)]
pub(crate) struct Package {
    pub name: String,
    pub manifest_path: PathBuf,
}

/// The workspace members, from `cargo metadata`.
pub(crate) fn members(root: &Path) -> io::Result<Vec<Package>> {
    let output = Command::new("cargo")
        .args([
            "metadata",
            "--no-deps",
            "--format-version",
            "1",
            "--offline",
        ])
        .current_dir(root)
        .output()?;
    if !output.status.success() {
        return Err(io::Error::other(format!(
            "cargo metadata failed:\n{}",
            String::from_utf8_lossy(&output.stderr)
        )));
    }
    let metadata: Metadata = serde_json::from_slice(&output.stdout).map_err(io::Error::other)?;
    Ok(metadata.packages)
}

fn read(root: &Path, path: &str) -> Option<String> {
    fs::read_to_string(root.join(path)).ok()
}

fn relative(root: &Path, path: &Path) -> String {
    path.strip_prefix(root)
        .unwrap_or(path)
        .to_string_lossy()
        .into_owned()
}

fn managed(file: &Managed) -> Step {
    Step::Write {
        path: file.path.to_owned(),
        content: file.content.to_owned(),
        executable: file.executable,
    }
}

/// rust-toolchain.toml with the pinned channel, keeping the targets the project added.
fn toolchain(current: Option<&str>, targets: &[&str]) -> String {
    let mut document: DocumentMut = TOOLCHAIN
        .parse()
        .expect("files/rust/rust-toolchain.toml is valid TOML, which the tests check");
    let mut wanted: Vec<String> = current
        .and_then(|text| text.parse::<DocumentMut>().ok())
        .and_then(|old| {
            old.get("toolchain")?
                .get("targets")?
                .as_array()
                .map(|list| {
                    list.iter()
                        .filter_map(|target| target.as_str().map(str::to_owned))
                        .collect()
                })
        })
        .unwrap_or_default();
    wanted.extend(targets.iter().map(|&target| target.to_owned()));
    wanted.sort();
    wanted.dedup();
    if !wanted.is_empty() {
        document["toolchain"]["targets"] = value(wanted.into_iter().collect::<Array>());
    }
    document.to_string()
}

/// Puts `#![cfg(test)]` after the leading `//!` lines, so Clippy treats the whole
/// integration test file as test code. A file without them gets the template's,
/// since each test file is a crate that `missing_docs` checks.
fn gate_integration_test(source: &str) -> Option<String> {
    if source.contains("#![cfg(test)]") {
        return None;
    }
    let docs: usize = source
        .lines()
        .take_while(|line| line.starts_with("//!"))
        .map(|line| line.len() + 1)
        .sum();
    let (head, tail) = source.split_at(docs.min(source.len()));
    let head = if head.is_empty() {
        "//! Integration tests run against the public API, as another crate would.\n"
    } else {
        head
    };
    Some(format!("{head}#![cfg(test)]\n\n{tail}"))
}

/// Every `foo/mod.rs` under a `src/` directory, with the `foo.rs` it moves to.
fn mod_files(directory: &Path, found: &mut Vec<(PathBuf, PathBuf)>) -> io::Result<()> {
    for entry in fs::read_dir(directory)? {
        let path = entry?.path();
        if path.is_dir() {
            let module = path.join("mod.rs");
            let sibling = path.with_extension("rs");
            if module.is_file() && !sibling.exists() {
                found.push((module, sibling));
            }
            mod_files(&path, found)?;
        }
    }
    Ok(())
}

/// AGENTS.md, its links, and the files every bedrock writes a block into.
fn shared(root: &Path) -> Vec<Step> {
    let mut steps = Vec::new();
    let agents = read(root, "AGENTS.md").or_else(|| {
        fs::symlink_metadata(root.join("CLAUDE.md"))
            .is_ok_and(|meta| meta.is_file())
            .then(|| read(root, "CLAUDE.md"))
            .flatten()
    });
    steps.push(Step::write(
        "AGENTS.md",
        upsert(agents.as_deref(), "# Agents\n", Syntax::Markdown, &AGENTS),
    ));
    for agent in ["CLAUDE.md", "GEMINI.md"] {
        steps.push(Step::Link {
            path: agent.to_owned(),
            target: "AGENTS.md",
        });
    }
    for (path, header, blocks) in [
        (".gitignore", "", &GITIGNORE),
        (".gitattributes", "", &GITATTRIBUTES),
        (".editorconfig", "root = true\n", &EDITORCONFIG),
    ] {
        steps.push(Step::write(
            path,
            upsert(read(root, path).as_deref(), header, Syntax::Hash, blocks),
        ));
    }
    let dependabot =
        read(root, ".github/dependabot.yml").filter(|text| text.contains("# >>> bedrock"));
    steps.push(Step::write(
        ".github/dependabot.yml",
        upsert(
            dependabot.as_deref(),
            "version: 2\nupdates:\n",
            Syntax::Hash,
            &DEPENDABOT,
        ),
    ));
    steps
}

/// Every change adopt makes in `root`, including ones already in place.
pub(crate) fn plan(root: &Path, source: &Source, targets: &[&str]) -> io::Result<Vec<Step>> {
    // `cargo metadata` reports resolved paths, such as /private/tmp for /tmp.
    let root = &root.canonicalize()?;
    let mut steps: Vec<Step> = BASE.iter().chain(RUST).map(managed).collect();
    if !root.join("package.json").exists() {
        steps.push(managed(&MARKDOWN));
    }
    steps.extend(
        STARTERS
            .iter()
            .filter(|file| !root.join(file.path).exists())
            .map(managed),
    );
    if !root.join("xtask/Cargo.toml").exists() {
        steps.push(Step::write("xtask/Cargo.toml", source.xtask_manifest()));
    }
    steps.push(Step::write(
        "rust-toolchain.toml",
        toolchain(read(root, "rust-toolchain.toml").as_deref(), targets),
    ));
    steps.extend(
        REPLACED
            .iter()
            .filter(|path| fs::symlink_metadata(root.join(path)).is_ok())
            .map(|path| Step::Remove {
                path: (*path).to_owned(),
            }),
    );

    steps.extend(shared(root));
    steps.push(Step::write(
        ".cargo/config.toml",
        manifest::cargo_config(read(root, ".cargo/config.toml").as_deref(), CARGO_CONFIG)
            .map_err(io::Error::other)?,
    ));

    let root_manifest = read(root, "Cargo.toml")
        .ok_or_else(|| io::Error::other("Cargo.toml is missing: run rust-bedrock new instead"))?;
    steps.push(Step::write(
        "Cargo.toml",
        manifest::root(&root_manifest).map_err(io::Error::other)?,
    ));
    for package in members(root)? {
        let directory = package.manifest_path.parent().unwrap_or(root);
        if package.manifest_path != root.join("Cargo.toml") {
            let current = fs::read_to_string(&package.manifest_path)?;
            steps.push(Step::write(
                relative(root, &package.manifest_path),
                manifest::member(&current).map_err(io::Error::other)?,
            ));
        }
        if let Ok(entries) = fs::read_dir(directory.join("tests")) {
            for entry in entries {
                let path = entry?.path();
                if path.extension().is_some_and(|extension| extension == "rs")
                    && let Some(gated) = gate_integration_test(&fs::read_to_string(&path)?)
                {
                    steps.push(Step::write(relative(root, &path), gated));
                }
            }
        }
        let mut moves = Vec::new();
        if directory.join("src").is_dir() {
            mod_files(&directory.join("src"), &mut moves)?;
        }
        steps.extend(moves.into_iter().map(|(from, to)| Step::Move {
            from: relative(root, &from),
            path: relative(root, &to),
        }));
    }
    Ok(steps)
}

const EDITIONS: [&str; 4] = ["2015", "2018", "2021", "2024"];

/// Runs `cargo fix --edition` and raises every member one edition at a time, until
/// all of them use the newest one.
pub(crate) fn migrate_editions(root: &Path) -> io::Result<()> {
    for _ in 0..EDITIONS.len() {
        let mut stale = Vec::new();
        for package in members(root)? {
            let text = fs::read_to_string(&package.manifest_path)?;
            if manifest::editions(&text)
                .iter()
                .any(|edition| edition != "2024")
            {
                stale.push((package, text));
            }
        }
        if stale.is_empty() {
            return Ok(());
        }
        let mut args = vec!["fix", "--edition", "--all-targets", "--allow-dirty"];
        for (package, _) in &stale {
            args.extend(["--package", package.name.as_str()]);
        }
        cargo(root, &args)?;
        for (package, mut text) in stale {
            for pair in EDITIONS.windows(2) {
                if manifest::editions(&text)
                    .iter()
                    .any(|edition| edition == pair[0])
                {
                    text = manifest::bump_edition(&text, pair[0], pair[1])
                        .map_err(io::Error::other)?;
                    break;
                }
            }
            fs::write(&package.manifest_path, text)?;
        }
    }
    Ok(())
}

pub(crate) fn cargo(root: &Path, args: &[&str]) -> io::Result<()> {
    let status = Command::new("cargo")
        .args(args)
        .current_dir(root)
        .status()?;
    if status.success() {
        Ok(())
    } else {
        Err(io::Error::other(format!("cargo {} failed", args.join(" "))))
    }
}

#[cfg(test)]
mod tests {
    use super::{Source, gate_integration_test, toolchain};

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
}
