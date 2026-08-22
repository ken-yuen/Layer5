// 迴圈: 外部借用在迴圈內使用, 被借者活得比迴圈久 → PASS
fn ex4() {
  let v
  let r = &v
  loop {
    use r
  }
}
