// E32 對不可變參數作 mut 出借
fn r32(imm p) {
  let a = &mut p
  use a
}
