// E15 暫存借用被非緊鄰陳述使用 (保守 E0716)
fn r15() {
  tmp t
  let a = &t
  let x
  use a
}
