// split borrow: x.f 與 x.g 無路徑衝突 → sh/mut 可共存
fn ex14() {
  let x
  let a = &x.f
  let b = &mut x.g
  use a
  use b
}
