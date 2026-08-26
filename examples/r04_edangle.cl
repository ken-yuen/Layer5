// E04 經槽使用已結束作用域的被借者 (弧端點掉出圓)
fn r04() {
  slot s
  {
    let y
    let a = &y
    store s = a
  }
  use s
}
