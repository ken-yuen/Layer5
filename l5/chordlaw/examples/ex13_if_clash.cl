// 分支內交越: a(&x) 在分支內仍被使用 → b(&mut x) 產生時 a 活躍 → E01
fn ex13() {
  let x
  let a = &x
  if {
    let b = &mut x
    use a
    use b
  }
}
