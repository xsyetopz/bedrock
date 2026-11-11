//! Reports what Clippy cannot: unit tests that sit in the file they test, and files that differ
//! from what adopt writes, such as a member that skips the workspace lints.

use std::fs;
use std::io;
use std::path::Path;
use std::process::Command;

use crate::adopt::{Source, plan};
use crate::plan::{Step, pending};
use crate::test_modules::{inline_tests, move_out, path_tests, tests_file};

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

pub(crate) fn guard(root: &Path, source: &Source, fix: bool) -> io::Result<Vec<String>> {
    let mut problems = Vec::new();
    for path in rust_files(root)? {
        let file = root.join(&path);
        // Moving the tests of an earlier file can remove a later one in the list.
        if !file.exists() {
            continue;
        }
        if fix {
            problems.extend(move_out(&file));
            continue;
        }
        let text = fs::read_to_string(&file)?;
        let destination = tests_file(&file);
        let destination = destination.strip_prefix(root).unwrap_or(&destination);
        for found in inline_tests(&text) {
            problems.push(format!(
                "{path}:{}: mod {} keeps its tests in this file; move them to {} and declare `#[cfg(test)] mod tests;` (just lint-fix does it)",
                found.line,
                found.name,
                destination.display()
            ));
        }
        for found in path_tests(&text) {
            problems.push(format!(
                "{path}:{}: mod {} sets its file with #[path]; drop the attribute and keep the tests in {} (just lint-fix does it)",
                found.line,
                found.name,
                destination.display()
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
