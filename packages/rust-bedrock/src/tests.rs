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
