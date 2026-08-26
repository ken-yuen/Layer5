// E16 把指向局部的參考存入槽
fn r16() {
  slot s
  let y
  let a = &y
  store s = a
}
