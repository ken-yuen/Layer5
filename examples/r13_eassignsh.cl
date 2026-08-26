// E13 經共享參考寫入
fn r13() {
  let x
  let a = &x
  set *a
}
