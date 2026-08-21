fn main() {
    let y = 5;
    println!("y = {}", y);

    let mut x = 5;
    x += 1;
    println!("x = {}", x);

    let s = String::from("hi");
    let t = s.clone();
    println!("{} {}", s, t);
}
