// 迴圈後向邊: r 的區間經後向邊向前閉包, 覆蓋 set v 點 → E02
fn ex5() {
  let v
  let r = &v
  loop {
    use r
    set v
  }
}
