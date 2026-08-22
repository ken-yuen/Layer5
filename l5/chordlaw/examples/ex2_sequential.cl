// NLL 語義: a 的最終使用在前, b 在其後 → 無交越, PASS
fn ex2() {
  let x
  let a = &x
  use a
  let b = &mut x
  use b
}
