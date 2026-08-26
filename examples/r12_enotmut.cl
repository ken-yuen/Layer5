// E12 對不可變地方作 mut 出借
fn r12() {
  let imm x
  let a = &mut x
  use a
}
