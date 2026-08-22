// move 後重新初始化並使用 (寫入 = 重新初始化, 非「使用」) → PASS (rustc 合法)
fn ex8() {
  let x
  mv x
  set x
  use x
}
