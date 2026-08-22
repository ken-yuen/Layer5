// 別名層衝突: a(&mut x) 被 b=&a 借用活躍期間, 又排他方式使用 a → E09 (~ rustc E0502)
fn ex7() {
  let x
  let a = &mut x
  let b = &a
  use a
  use b
}
