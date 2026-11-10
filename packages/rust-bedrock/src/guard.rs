//! Reports what Clippy cannot: test modules in other files, and files that differ
//! from what adopt writes, such as a member that skips the workspace lints.

use std::fs;
use std::io;
use std::path::{Component, Path};
use std::process::Command;

use crate::adopt::{Source, plan};
use crate::inline_tests::{declarations, inline};
use crate::plan::{Step, pending};

/// The Rust files git tracks or would track.
fn rust_files(root: &Path) -> io::Result<Vec<String>> {
    let output = Command::new("git")
        .args([
            "ls-files",
            "-z",
            "--cached",
            "--others",
            "--exclude-standard",
            "--",
            "*.rs",
        ])
        .current_dir(root)
        .output()?;
    if !output.status.success() {
        return Err(io::Error::other("guard runs in a git repository"));
    }
    Ok(String::from_utf8_lossy(&output.stdout)
        .split('\0')
        .filter(|path| !path.is_empty() && root.join(path).is_file())
        .map(str::to_owned)
        .collect())
}

/// A `tests.rs` or `tests/` inside `src/` holds unit tests outside the file they test.
fn is_test_file_in_src(path: &str) -> bool {
    let parts: Vec<_> = Path::new(path).components().collect();
    let Some(src) = parts
        .iter()
        .position(|part| *part == Component::Normal("src".as_ref()))
    else {
        return false;
    };
    parts[src + 1..].iter().any(|part| {
        *part == Component::Normal("tests".as_ref())
            || *part == Component::Normal("tests.rs".as_ref())
    })
}

pub(crate) fn guard(root: &Path, source: &Source, fix: bool) -> io::Result<Vec<String>> {
    let mut problems = Vec::new();
    for path in rust_files(root)? {
        let file = root.join(&path);
        // Inlining an earlier file can remove a later one in the list.
        if !file.exists() {
            continue;
        }
        if fix {
            problems.extend(inline(&file)?);
        } else {
            let text = fs::read_to_string(&file)?;
            for found in declarations(&text)
                .into_iter()
                .filter(|found| found.test_only)
            {
                problems.push(format!(
                    "{path}:{}: mod {} keeps its tests in another file; move them here as `mod {} {{ … }}` (just lint-fix does it)",
                    found.line, found.name, found.name
                ));
            }
        }
        if is_test_file_in_src(&path) && file.exists() {
            problems.push(format!(
                "{path}: unit tests belong inline in the file they test, in #[cfg(test)] mod tests {{ … }}"
            ));
        }
    }
    for step in pending(root, plan(root, source, &[])?) {
        problems.push(match step {
            Step::Move { from, path, .. } => {
                format!("{from}: name it {path}, beside its directory (cargo bedrock adopt does it)")
            }
            step => format!(
                "{}: differs from what rust-bedrock {} writes; run cargo bedrock adopt --dry-run to see the change",
                step.path(),
                env!("CARGO_PKG_VERSION")
            ),
        });
    }
    Ok(problems)
}

#[cfg(test)]
mod tests {
    use super::is_test_file_in_src;

    #[test]
    fn finds_test_files_inside_src_only() {
        assert!(is_test_file_in_src("src/parser/tests.rs"));
        assert!(is_test_file_in_src("crates/a/src/tests/mod.rs"));
        assert!(!is_test_file_in_src("crates/a/tests/cli.rs"));
        assert!(!is_test_file_in_src("src/parser.rs"));
    }
}
