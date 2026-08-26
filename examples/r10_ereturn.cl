// E10 回傳指向局部值的參考 (弧逸出 ROOT 圓)
fn r10() {
  let x
  let a = &x
  ret a
}
