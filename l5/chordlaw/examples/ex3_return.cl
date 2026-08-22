// 逸出: 回傳參考之被借者 x 不活得比呼叫者久 → E10 (~ rustc E0106)
fn ex3() {
  let x
  let a = &x
  ret a
}
