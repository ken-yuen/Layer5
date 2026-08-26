// E21 把指向槽自身的參考存回槽 (別名環)
fn r21() {
  slot s
  let a = &s
  store s = a
}
