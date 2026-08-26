// E07 借用活躍期間 move 被借者
fn r07() {
  let x
  let a = &x
  mv x
  use a
}
