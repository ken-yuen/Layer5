// E18 if 分支內 move 後於分支外使用
fn r18() {
  let x
  if {
    mv x
  }
  use x
}
