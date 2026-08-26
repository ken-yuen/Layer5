// E17 把指向暫存的參考存入槽
fn r17() {
  slot s
  tmp t
  let a = &t
  store s = a
}
