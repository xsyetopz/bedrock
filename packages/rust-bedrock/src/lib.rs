//! Adopts and guards the bedrock setup in Rust repositories: one justfile, git
//! hooks, strict workspace lints, and unit tests inline in the file they test.
#![expect(
    clippy::print_stdout,
    clippy::print_stderr,
    reason = "a command-line tool reports on stdout and stderr"
)]

mod adopt;
mod blocks;
mod files;
mod guard;
mod inline_tests;
mod lexer;
mod manifest;
mod plan;

use std::env;
use std::fs;
use std::io::{self, BufRead, IsTerminal, Write};
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};

use adopt::Source;
use files::KINDS;
use plan::{Step, pending};

const USAGE: &str = "usage:
  rust-bedrock new <dir> [--kind lib|bin|no-std|ffi|wasm]... [--name <name>]
  rust-bedrock adopt [<dir>] [--dry-run] [--yes] [--no-run]
  rust-bedrock guard [--fix]

new and adopt also take --bedrock-path <path>: depend on a local rust-bedrock,
given relative to xtask/, instead of the tagged release.";

#[derive(Debug, Default)]
struct Options {
    positional: Vec<String>,
    kinds: Vec<String>,
    name: Option<String>,
    bedrock_path: Option<String>,
    dry_run: bool,
    yes: bool,
    no_run: bool,
    fix: bool,
}

fn parse(args: &[String]) -> Result<Options, String> {
    let mut options = Options::default();
    let mut args = args.iter();
    while let Some(arg) = args.next() {
        let mut value = || {
            args.next()
                .cloned()
                .ok_or_else(|| format!("{arg} needs a value"))
        };
        match arg.as_str() {
            "--kind" => options.kinds.push(value()?),
            "--name" => options.name = Some(value()?),
            "--bedrock-path" => options.bedrock_path = Some(value()?),
            "--dry-run" => options.dry_run = true,
            "--yes" => options.yes = true,
            "--no-run" => options.no_run = true,
            "--fix" => options.fix = true,
            flag if flag.starts_with('-') => return Err(format!("unknown option {flag}")),
            _ => options.positional.push(arg.clone()),
        }
    }
    Ok(options)
}

/// Runs the command line in `env::args`, for the binary and for `xtask`.
pub fn run() -> ExitCode {
    let args: Vec<String> = env::args().skip(1).collect();
    let result = match args.split_first() {
        Some((command, rest)) => parse(rest).and_then(|options| match command.as_str() {
            "new" => new(&options),
            "adopt" => run_adopt(&options),
            "guard" => run_guard(&options),
            "help" | "--help" | "-h" => {
                println!("{USAGE}");
                Ok(())
            }
            other => Err(format!("unknown command {other}\n{USAGE}")),
        }),
        None => Err(USAGE.to_owned()),
    };
    match result {
        Ok(()) => ExitCode::SUCCESS,
        Err(message) => {
            eprintln!("{message}");
            ExitCode::FAILURE
        }
    }
}

fn source(options: &Options) -> Source {
    options
        .bedrock_path
        .clone()
        .map_or(Source::Release, Source::Path)
}

fn run_guard(options: &Options) -> Result<(), String> {
    let root = env::current_dir().map_err(|error| error.to_string())?;
    let problems =
        guard::guard(&root, &source(options), options.fix).map_err(|error| error.to_string())?;
    if problems.is_empty() {
        return Ok(());
    }
    Err(problems.join("\n"))
}

fn confirm() -> bool {
    print!("Apply these changes? [y/N] ");
    let mut answer = String::new();
    io::stdout().flush().is_ok()
        && io::stdin().lock().read_line(&mut answer).is_ok()
        && answer.trim().eq_ignore_ascii_case("y")
}

fn run_adopt(options: &Options) -> Result<(), String> {
    let root = options
        .positional
        .first()
        .map_or_else(env::current_dir, |path| Ok(PathBuf::from(path)))
        .map_err(|error| error.to_string())?;
    adopt_in(&root, options, &[])
}

fn adopt_in(root: &Path, options: &Options, targets: &[&str]) -> Result<(), String> {
    let failed = |error: io::Error| error.to_string();
    let steps = pending(
        root,
        adopt::plan(root, &source(options), targets).map_err(failed)?,
    );
    for step in &steps {
        println!("{}", step.describe(root));
    }
    if options.dry_run {
        return Ok(());
    }
    if !(options.yes || io::stdin().is_terminal() && confirm()) {
        return Err("Nothing changed. Rerun with --yes to apply.".to_owned());
    }
    for step in &steps {
        step.apply(root).map_err(failed)?;
    }
    if !options.no_run {
        adopt::migrate_editions(root).map_err(failed)?;
        let problems = guard::guard(root, &source(options), true).map_err(failed)?;
        adopt::cargo(root, &["update", "--workspace", "--quiet"]).map_err(failed)?;
        adopt::cargo(root, &["fmt", "--all"]).map_err(failed)?;
        for problem in problems {
            println!("{problem}");
        }
    }
    println!(
        "Done. Run `just setup`, then `just verify`, and fix what it reports. Compare each *.bak file with its replacement and delete it; move what a removed Makefile or hook did into project.just."
    );
    Ok(())
}

/// The crate directory and package name for each kind, `<name>` alone or `<name>-<kind>`.
fn crates(name: &str, kinds: &[&'static files::Kind]) -> Vec<(String, &'static files::Kind)> {
    kinds
        .iter()
        .map(|kind| {
            let package = if kinds.len() == 1 {
                name.to_owned()
            } else {
                format!("{name}-{}", kind.name)
            };
            (package, *kind)
        })
        .collect()
}

fn new(options: &Options) -> Result<(), String> {
    let [directory] = options.positional.as_slice() else {
        return Err(format!("new takes one directory\n{USAGE}"));
    };
    let root = PathBuf::from(directory);
    if root.join("Cargo.toml").exists() {
        return Err(format!(
            "{directory} has a Cargo.toml already: run rust-bedrock adopt"
        ));
    }
    let wanted = if options.kinds.is_empty() {
        vec!["lib".to_owned()]
    } else {
        options.kinds.clone()
    };
    let mut kinds = Vec::new();
    for wanted in &wanted {
        let kind = KINDS
            .iter()
            .find(|kind| kind.name == wanted)
            .ok_or_else(|| {
                let known: Vec<String> = KINDS
                    .iter()
                    .map(|kind| format!("  {:<7}{}", kind.name, kind.summary))
                    .collect();
                format!(
                    "unknown kind {wanted}; the kinds are:\n{}",
                    known.join("\n")
                )
            })?;
        kinds.push(kind);
    }
    let fallback = fs::canonicalize(&root)
        .unwrap_or_else(|_| root.clone())
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_default();
    let name = options.name.clone().unwrap_or(fallback);
    if name.is_empty()
        || !name
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
    {
        return Err(format!("{name:?} is not a package name: pass --name"));
    }

    let manifest = manifest::root(files::ROOT_MANIFEST).map_err(|error| error.to_string())?;
    let mut steps = vec![
        Step::write("Cargo.toml", manifest),
        Step::write("xtask/Cargo.toml", source(options).xtask_manifest()),
        Step::write(files::XTASK_MAIN.path, files::XTASK_MAIN.content),
    ];
    for (package, kind) in crates(&name, &kinds) {
        let crate_name = package.replace('-', "_");
        for (path, content) in kind.files {
            steps.push(Step::write(
                format!("crates/{package}/{path}"),
                content
                    .replace("__name__", &package)
                    .replace("__crate__", &crate_name),
            ));
        }
        if kind.name == "ffi" && root.join("package.json").exists() {
            for (path, content) in files::BUN_FFI {
                steps.push(Step::write(
                    path.replace("__name__", &package),
                    content
                        .replace("__name__", &package)
                        .replace("__crate__", &crate_name),
                ));
            }
        }
    }
    if !root.join("README.md").exists() {
        steps.push(Step::write(
            "README.md",
            files::README.replace("__name__", &name),
        ));
    }
    let failed = |error: io::Error| error.to_string();
    for step in &steps {
        println!("{}", step.describe(&root));
        step.apply(&root).map_err(failed)?;
    }
    if !root.join(".git").exists() {
        Command::new("git")
            .args(["init", "--quiet"])
            .current_dir(&root)
            .status()
            .map_err(failed)?;
    }
    let targets: Vec<&str> = kinds.iter().filter_map(|kind| kind.target).collect();
    adopt_in(
        &root,
        &Options {
            yes: true,
            bedrock_path: options.bedrock_path.clone(),
            no_run: options.no_run,
            ..Options::default()
        },
        &targets,
    )
}

#[cfg(test)]
mod tests {
    use super::{crates, parse};
    use crate::files::KINDS;

    #[test]
    fn parses_flags_and_values() {
        let args = ["x", "--kind", "ffi", "--dry-run"].map(str::to_owned);
        let options = parse(&args).unwrap();
        assert_eq!(options.positional, ["x"]);
        assert_eq!(options.kinds, ["ffi"]);
        assert!(options.dry_run);
        assert!(parse(&["--name".to_owned()]).is_err());
        assert!(parse(&["--force".to_owned()]).is_err());
    }

    #[test]
    fn names_one_crate_after_the_project_and_several_after_their_kinds() {
        let one: Vec<String> = crates("demo", &[&KINDS[0]])
            .into_iter()
            .map(|(name, _)| name)
            .collect();
        assert_eq!(one, ["demo"]);
        let two: Vec<String> = crates("demo", &[&KINDS[0], &KINDS[1]])
            .into_iter()
            .map(|(name, _)| name)
            .collect();
        assert_eq!(two, ["demo-lib", "demo-bin"]);
    }
}
