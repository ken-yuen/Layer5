// E23 對不可變字段作 mut 出借
fn r23() {
  let imm x
  let a = &mut x.f
  use a
}
