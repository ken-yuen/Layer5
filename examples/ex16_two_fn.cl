// 多 fn 檔案: 各 fn 獨立檢查 (同名 place 不互相干擾)
fn ok() {
  let x
  let a = &x
  use a
}
fn bad() {
  let x
  let a = &x
  set x
  use a
}
