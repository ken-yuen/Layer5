// 紅弧交越: b(&mut x) 產生於 a(&x) 的活跃區間內 → E01
fn ex1() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}
