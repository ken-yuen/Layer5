// 分支 NLL 益處: 分支內使用不延長活度過分支 → 分支後寫入合法
fn ex12() {
  let x
  let a = &x
  if {
    use a
  }
  set x
}
