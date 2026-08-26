// E09 別名層衝突: 參考 a 使用期間自身被借用活躍
fn r09() {
  let x
  let a = &mut x
  let b = &a
  use a
  use b
}
