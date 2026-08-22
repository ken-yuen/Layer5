// 移動: a(&x) 活躍期間移動 x → E07; 移動後使用 a → E06 (~ rustc E0382)
fn ex6() {
  let x
  let a = &x
  mv x
  use a
}
