// E27 借用活躍期間呼叫消耗被借者
fn r27() {
  let x
  let a = &x
  callmv take(x)
  use a
}
