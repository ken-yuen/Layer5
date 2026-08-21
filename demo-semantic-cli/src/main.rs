fn main() {
    let name = "world".to_string();
    greet(name);        // E0425：找不到函式 greet → 無機械修復，cargo fix 也救不了
}
