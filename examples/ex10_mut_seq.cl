// mut 借用依序使用 (前一個用完再借) → PASS (rustc 合法)
fn ex10() {
  let x
  let a = &mut x
  use a
  let b = &mut x
  use b
}
