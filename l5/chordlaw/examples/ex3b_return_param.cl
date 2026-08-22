// 回傳參考至參數 p (來自呼叫者, ROOT 範圍) → PASS
fn ex3b(p) {
  let a = &p
  ret a
}
