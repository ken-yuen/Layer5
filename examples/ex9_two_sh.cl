// 兩條共享借用可重疊共存 → PASS (rustc 合法)
fn ex9() {
  let x
  let a = &x
  let b = &x
  use a
  use b
}
