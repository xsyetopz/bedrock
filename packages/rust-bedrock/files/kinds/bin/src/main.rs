//! Prints the sum of its arguments: `__name__ 1 2 3`.
#![expect(
    clippy::print_stdout,
    clippy::print_stderr,
    reason = "a command-line tool reports on stdout and stderr"
)]

use std::env;
use std::process::ExitCode;

fn sum(args: &[String]) -> Result<i64, String> {
    args.iter().try_fold(0_i64, |total, arg| {
        let value = arg.parse::<i64>().map_err(|error| format!("{arg}: {error}"))?;
        total.checked_add(value).ok_or_else(|| "the sum overflows".to_owned())
    })
}

fn main() -> ExitCode {
    let args: Vec<String> = env::args().skip(1).collect();
    match sum(&args) {
        Ok(total) => {
            println!("{total}");
            ExitCode::SUCCESS
        }
        Err(message) => {
            eprintln!("error: {message}");
            ExitCode::FAILURE
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn sums_numbers() {
        assert_eq!(sum(&["1", "-2", "4"].map(str::to_owned)), Ok(3));
    }

    #[test]
    fn rejects_text_and_overflow() {
        assert!(sum(&["x".to_owned()]).is_err());
        assert!(sum(&[i64::MAX.to_string(), "1".to_owned()]).is_err());
    }
}
