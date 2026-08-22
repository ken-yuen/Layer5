// 別名鏈活度: b=&a 活著 → a(&mut x) 之 loan 仍活躍 → c=&mut x 衝突 (E01)
// 且 use a 於 b 活躍期間 (E09)
fn ex11() {
  let x
  let a = &mut x
  let b = &a
  use a
  let c = &mut x
  use c
  use b
}
