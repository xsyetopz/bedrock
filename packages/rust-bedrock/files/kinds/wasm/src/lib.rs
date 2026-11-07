//! WebAssembly exports. Build with `cargo build --release --target wasm32-unknown-unknown`.
#![expect(unsafe_code, reason = "wasm exports need #[unsafe(no_mangle)]")]

/// The `n`th Fibonacci number, saturating at `u64::MAX`.
#[unsafe(no_mangle)]
pub extern "C" fn fibonacci(n: u32) -> u64 {
    let (mut current, mut next) = (0_u64, 1_u64);
    for _ in 0..n {
        (current, next) = (next, current.saturating_add(next));
    }
    current
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn counts_and_saturates() {
        assert_eq!(fibonacci(10), 55);
        assert_eq!(fibonacci(200), u64::MAX);
    }
}
