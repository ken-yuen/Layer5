// 路徑衝突: 整體 x(&mut) 與 x.f(&) 為字段前綴關係 → E01
fn ex15() {
  let x
  let a = &x.f
  let b = &mut x
  use a
  use b
}
