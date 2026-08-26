// E01 紅弧交越: b(&mut x) 產生於 a(&x) 的活躍區間內
fn r01() {
  let x
  let a = &x
  use a
  let b = &mut x
  use a
  use b
}
