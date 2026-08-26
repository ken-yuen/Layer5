// E24 else 分支內 move 後於分支外使用
fn r24() {
  let x
  if {
    set x
  } else {
    mv x
  }
  use x
}
