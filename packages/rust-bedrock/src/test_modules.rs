//! Finds unit tests that sit in the file they test, as `#[cfg(test)] mod name { … }`,
//! or behind a `#[path]` attribute, and moves them to `tests.rs` beside that file, so
//! the file declares `#[cfg(test)] mod tests;` and nothing more.

use std::fs;
use std::io;
use std::path::{Path, PathBuf};

use crate::mod_paths::prefix_paths;
use crate::modules::{Module, child_file, module_directory, modules};

/// The `#[cfg(test)] mod name { … }` modules of `source`.
pub(crate) fn inline_tests(source: &str) -> Vec<Module> {
    modules(source)
        .into_iter()
        .filter(|module| module.test_only && module.body.is_some())
        .collect()
}

/// The `#[cfg(test)] #[path = "…"] mod name;` modules of `source`.
pub(crate) fn path_tests(source: &str) -> Vec<Module> {
    modules(source)
        .into_iter()
        .filter(|module| module.test_only && module.body.is_none() && module.path.is_some())
        .collect()
}

/// Where the tests of `file` belong: `foo/tests.rs` for `foo.rs`, and `tests.rs`
/// beside a `lib.rs`, `main.rs`, or `mod.rs`.
pub(crate) fn tests_file(file: &Path) -> PathBuf {
    module_directory(file).join("tests.rs")
}

/// What moving one module needs: a new file, an edit to the parent, and a file to remove.
#[derive(Default)]
struct Plan {
    /// Byte range of the parent's text and what replaces it.
    edits: Vec<(usize, usize, String)>,
    writes: Vec<(PathBuf, String)>,
    removals: Vec<PathBuf>,
    problems: Vec<String>,
}

fn same_file(left: &Path, right: &Path) -> bool {
    match (left.canonicalize(), right.canonicalize()) {
        (Ok(left), Ok(right)) => left == right,
        _ => false,
    }
}

/// The byte just past `end` and the whitespace that follows it.
fn past_space(source: &str, end: usize) -> usize {
    end + source[end..]
        .find(|c: char| !c.is_whitespace())
        .unwrap_or(source.len() - end)
}

fn plan(file: &Path, source: &str) -> Plan {
    let mut plan = Plan::default();
    let destination = tests_file(file);
    let place = file.display();
    for module in modules(source)
        .into_iter()
        .filter(|module| module.test_only)
    {
        if module.body.is_none() && module.path.is_none() {
            continue;
        }
        let line = module.line;
        let name = &module.name;
        if module.depth > 0 || name != "tests" {
            plan.problems.push(format!(
                "{place}:{line}: cannot move `mod {name}`; declare `#[cfg(test)] mod tests;` at the top level of the file"
            ));
            continue;
        }
        let taken =
            destination.exists() || plan.writes.iter().any(|(path, _)| *path == destination);
        if let Some((open, close)) = module.body {
            if taken {
                plan.problems.push(format!(
                    "{place}:{line}: cannot move the tests: {} exists",
                    destination.display()
                ));
                continue;
            }
            plan.writes.push((
                destination.clone(),
                source[open + 1..close]
                    .trim_start_matches(['\r', '\n'])
                    .to_owned(),
            ));
            plan.edits
                .push((module.start, close + 1, "mod tests;".to_owned()));
            continue;
        }
        let (Some((from, to)), Some(current)) = (module.path_attribute, child_file(file, &module))
        else {
            plan.problems.push(format!(
                "{place}:{line}: cannot find the file named by `#[path]`"
            ));
            continue;
        };
        let drop_attribute = (from, past_space(source, to), String::new());
        if same_file(&current, &destination) {
            plan.edits.push(drop_attribute);
            continue;
        }
        let text = match fs::read_to_string(&current) {
            Ok(text) => text,
            Err(error) => {
                plan.problems.push(failure("read", &current, &error));
                continue;
            }
        };
        if taken {
            plan.problems.push(format!(
                "{place}:{line}: cannot move {}: {} exists",
                current.display(),
                destination.display()
            ));
        } else if prefix_paths(&text, "moved/").as_deref() != Some(text.as_str()) {
            plan.problems.push(format!(
                "{place}:{line}: cannot move {}: it names files by relative path, which would stop resolving",
                current.display()
            ));
        } else {
            plan.edits.push(drop_attribute);
            plan.writes.push((destination.clone(), text));
            plan.removals.push(current);
        }
    }
    plan
}

fn failure(operation: &str, path: &Path, error: &io::Error) -> String {
    format!("{}: cannot {operation}: {error}", path.display())
}

fn write_new(path: &Path, text: &str) -> io::Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    fs::write(path, text)
}

/// Moves each test module of `file` to the `tests.rs` beside it and leaves
/// `#[cfg(test)] mod tests;`, dropping a `#[path]` attribute on the way. Returns one
/// message per module it cannot move; the others still move.
pub(crate) fn move_out(file: &Path) -> Vec<String> {
    let mut source = match fs::read_to_string(file) {
        Ok(source) => source,
        Err(error) => return vec![failure("read", file, &error)],
    };
    let Plan {
        mut edits,
        writes,
        removals,
        mut problems,
    } = plan(file, &source);
    if edits.is_empty() {
        return problems;
    }
    for (path, text) in &writes {
        if let Err(error) = write_new(path, text) {
            problems.push(failure("write", path, &error));
            return problems;
        }
    }
    edits.sort_by_key(|edit| std::cmp::Reverse(edit.0));
    for (start, end, text) in edits {
        source.replace_range(start..end, &text);
    }
    if let Err(error) = fs::write(file, source) {
        problems.push(failure("write", file, &error));
        return problems;
    }
    for old in removals {
        if let Err(error) = fs::remove_file(&old) {
            problems.push(failure("remove", &old, &error));
        }
    }
    problems
}

#[cfg(test)]
mod tests;
