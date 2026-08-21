//! demo-cli 的函式庫目標：讓 doc tests 可以被 `cargo test` 執行。

/// Adds one to the input.
///
/// # Examples
///
/// ```
/// assert_eq!(demo_cli::add_one(2), 3);
/// ```
pub fn add_one(x: i64) -> i64 {
    x + 1
}

#[cfg(test)]
mod tests {
    #[test]
    fn smoke_add_one() {
        assert_eq!(super::add_one(1), 2);
    }
}
