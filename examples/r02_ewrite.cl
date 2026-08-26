// E02 借用活躍期間寫入被借者
fn r02() {
  let x
  let a = &x
  set x
  use a
}
