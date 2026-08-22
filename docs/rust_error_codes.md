# Rust 編譯錯誤碼參考（全部 518 條：錯誤範例 + 正解）

> 來源：rustc 官方錯誤索引（doc.rust-lang.org/error_codes/）。
> 本檔由 YKC 知識庫內嵌資料（`internal/kb/data/errcodes.json.gz`）產生，供離線查閱；
> 程式內以 `ykc-know code E0382` 或 `ykc-know search ...` 精準檢索。


---

## E0001 — This error suggests that the expression arm corresponding to the noted pattern
will never be reached as for all possible values of the expression being
matched, one of the preceding patterns will match.

Note: this error code is no longer emitted by the compiler. This error suggests that the expression arm corresponding to the noted pattern will never be reached as for all possible values of the expression being matched, one of the preceding patterns will match. This means that perhaps some of the preceding patterns are too general, this one is too specific or the ordering is incorrect. For example, the following match block has too many arms: match blocks have their patterns matched in order, so, for example, putting a wildcard arm above a more specific arm will make the latter arm irrelevant. Ensure the ordering of the match arm is correct and remove any superfluous arms.

**正解**
```rust
#![allow(unused)]
fn main() {
match Some(0) {
    Some(bar) => {/* ... */}
    x => {/* ... */} // This handles the `None` case
    _ => {/* ... */} // All possible cases have already been handled
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0001.html


---

## E0002 — This error indicates that an empty match expression is invalid because the type
it is matching on is non-empty (there exist values of this type).

Note: this error code is no longer emitted by the compiler. This error indicates that an empty match expression is invalid because the type it is matching on is non-empty (there exist values of this type). In safe code it is impossible to create an instance of an empty type, so empty match expressions are almost never desired. This error is typically fixed by adding one or more cases to the match expression. An example of an empty type is enum Empty { }. So, the following will work: However, this won’t:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo(x: Option<String>) {
    match x {
        // empty
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Empty {}

fn foo(x: Empty) {
    match x {
        // empty
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0002.html


---

## E0004 — This error indicates that the compiler cannot guarantee a matching pattern for
one or more possible inputs to a match expression. Guaranteed matches are
required in order to assign values to match expressions, or alternatively,
determine the flow of execution.

This error indicates that the compiler cannot guarantee a matching pattern for one or more possible inputs to a match expression. Guaranteed matches are required in order to assign values to match expressions, or alternatively, determine the flow of execution. If you encounter this error you must alter your patterns so that every possible value of the input type is matched. For types with a small number of variants (like enums) you should probably cover all cases explicitly. Alternatively, the underscore _ wildcard pattern can be added after all other patterns to match “anything else”. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Terminator {
    HastaLaVistaBaby,
    TalkToMyHand,
}

let x = Terminator::HastaLaVistaBaby;

match x { // error: non-exhaustive patterns: `HastaLaVistaBaby` not covered
    Terminator::TalkToMyHand => {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Terminator {
    HastaLaVistaBaby,
    TalkToMyHand,
}

let x = Terminator::HastaLaVistaBaby;

match x {
    Terminator::TalkToMyHand => {}
    Terminator::HastaLaVistaBaby => {}
}

// or:

match x {
    Terminator::TalkToMyHand => {}
    _ => {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0004.html


---

## E0005 — Patterns used to bind names must be irrefutable, that is, they must guarantee
that a name will be extracted in all cases.

Patterns used to bind names must be irrefutable, that is, they must guarantee that a name will be extracted in all cases. If you encounter this error you probably need to use a match or if let to deal with the possibility of failure. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = Some(1);
let Some(y) = x;
// error: refutable pattern in local binding: `None` not covered
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = Some(1);

match x {
    Some(y) => {
        // do something
    },
    None => {}
}

// or:

if let Some(y) = x {
    // do something
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0005.html


---

## E0007 — This error indicates that the bindings in a match arm would require a value to
be moved into more than one location, thus violating unique ownership.

Note: this error code is no longer emitted by the compiler. This error indicates that the bindings in a match arm would require a value to be moved into more than one location, thus violating unique ownership. Code like the following is invalid as it requires the entire Option<String> to be moved into a variable called op_string while simultaneously requiring the inner String to be moved into a variable called s. See also the error E0303.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(bindings_after_at)]

fn main() {
let x = Some("s".to_string());

match x {
    op_string @ Some(s) => {}, // error: use of moved value
    None => {},
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0007.html


---

## E0009 — In a pattern, all values that don’t implement the Copy trait have to be bound
the same way.

Note: this error code is no longer emitted by the compiler. In a pattern, all values that don’t implement the Copy trait have to be bound the same way. The goal here is to avoid binding simultaneously by-move and by-ref. This limitation may be removed in a future version of Rust. You have two solutions: Solution #1: Bind the pattern’s values the same way. Solution #2: Implement the Copy trait for the X structure. However, please keep in mind that the first solution should be preferred.

**正解**
```rust
#![allow(unused)]
#![feature(move_ref_pattern)]

fn main() {
struct X { x: (), }

let x = Some((X { x: () }, X { x: () }));
match x {
    Some((y, ref z)) => {}, // error: cannot bind by-move and by-ref in the
                            //        same pattern
    None => panic!()
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0009.html


---

## E0010 — The value of statics and constants must be known at compile time, and they live
for the entire lifetime of a program.

Note: this error code is no longer emitted by the compiler. The value of statics and constants must be known at compile time, and they live for the entire lifetime of a program. Creating a boxed value allocates memory on the heap at runtime, and therefore cannot be done at compile time.

**正解**
```rust
const CON : Vec<i32> = vec![1, 2, 3];
```

出處：https://doc.rust-lang.org/error_codes/E0010.html


---

## E0013 — Note: this error code is no longer emitted by the compiler
Static and const variables can refer to other const variables.

Note: this error code is no longer emitted by the compiler Static and const variables can refer to other const variables. But a const variable cannot refer to a static variable. In this example, Y cannot refer to X. To fix this, the value can be extracted as a const and then used:

**正解**
```rust
#![allow(unused)]
fn main() {
static X: i32 = 42;
const Y: i32 = X;
}
```

出處：https://doc.rust-lang.org/error_codes/E0013.html


---

## E0014 — Constants can only be initialized by a constant value or, in a future
version of Rust, a call to a const function.

Note: this error code is no longer emitted by the compiler. Constants can only be initialized by a constant value or, in a future version of Rust, a call to a const function. This error indicates the use of a path (like a::b, or x) denoting something other than one of these allowed items. To avoid it, you have to replace the non-constant value:

**正解**
```rust
#![allow(unused)]
fn main() {
const FOO: i32 = { let x = 0; x }; // 'x' isn't a constant nor a function!
}
```

出處：https://doc.rust-lang.org/error_codes/E0014.html


---

## E0015 — A non-const function was called in a const context.

A non-const function was called in a const context. All functions used in a const context (constant or static expression) must be marked const. To fix this error, you can declare create_some as a constant function:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn create_some() -> Option<u8> {
    Some(1)
}

// error: cannot call non-const function `create_some` in constants
const FOO: Option<u8> = create_some();
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
// declared as a `const` function:
const fn create_some() -> Option<u8> {
    Some(1)
}

const FOO: Option<u8> = create_some(); // no error!
}
```

出處：https://doc.rust-lang.org/error_codes/E0015.html


---

## E0023 — A pattern attempted to extract an incorrect number of fields from a variant.

A pattern attempted to extract an incorrect number of fields from a variant. A pattern used to match against an enum variant must provide a sub-pattern for each field of the enum variant. Here the Apple variant has two fields, and should be matched against like so: Matching with the wrong number of fields has no sensible interpretation: Check how many fields the enum was declared with and ensure that your pattern uses the same number.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Fruit {
    Apple(String, String),
    Pear(u32),
}

let x = Fruit::Apple(String::new(), String::new());

match x {
    Fruit::Apple(a) => {}, // error!
    _ => {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Fruit {
    Apple(String, String),
    Pear(u32),
}

let x = Fruit::Apple(String::new(), String::new());

// Correct.
match x {
    Fruit::Apple(a, b) => {},
    _ => {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0023.html


---

## E0025 — Each field of a struct can only be bound once in a pattern.

Each field of a struct can only be bound once in a pattern. Each occurrence of a field name binds the value of that field, so to fix this error you will have to remove or alter the duplicate uses of the field name. Perhaps you misspelled another field name? Example:

**錯誤範例**
```rust
struct Foo {
    a: u8,
    b: u8,
}

fn main(){
    let x = Foo { a:1, b:2 };

    let Foo { a: x, a: y } = x;
    // error: field `a` bound multiple times in the pattern
}
```

**正解**
```rust
struct Foo {
    a: u8,
    b: u8,
}

fn main(){
    let x = Foo { a:1, b:2 };

    let Foo { a: x, b: y } = x; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0025.html


---

## E0026 — A struct pattern attempted to extract a nonexistent field from a struct.

A struct pattern attempted to extract a nonexistent field from a struct. If you are using shorthand field patterns but want to refer to the struct field by a different name, you should rename it explicitly. Struct fields are identified by the name used before the colon : so struct patterns should resemble the declaration of the struct type being matched.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Thing {
    x: u32,
    y: u32,
}

let thing = Thing { x: 0, y: 0 };

match thing {
    Thing { x, z } => {} // error: `Thing::z` field doesn't exist
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Thing {
    x: u32,
    y: u32,
}

let thing = Thing { x: 0, y: 0 };

match thing {
    Thing { x, y: z } => {} // we renamed `y` to `z`
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0026.html


---

## E0027 — A pattern for a struct fails to specify a sub-pattern for every one of the
struct’s fields.

A pattern for a struct fails to specify a sub-pattern for every one of the struct’s fields. To fix this error, ensure that each field from the struct’s definition is mentioned in the pattern, or use .. to ignore unwanted fields. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Dog {
    name: String,
    age: u32,
}

let d = Dog { name: "Rusty".to_string(), age: 8 };

// This is incorrect.
match d {
    Dog { age: x } => {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Dog {
    name: String,
    age: u32,
}

let d = Dog { name: "Rusty".to_string(), age: 8 };

match d {
    Dog { name: ref n, age: x } => {}
}

// This is also correct (ignore unused fields).
match d {
    Dog { age: x, .. } => {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0027.html


---

## E0029 — Something other than numbers and characters has been used for a range.

Something other than numbers and characters has been used for a range. In a match expression, only numbers and characters can be matched against a range. This is because the compiler checks that the range is non-empty at compile-time, and is unable to evaluate arbitrary comparison functions. If you want to capture values of an orderable type between two end-points, you can use a guard.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let string = "salutations !";

// The ordering relation for strings cannot be evaluated at compile time,
// so this doesn't work:
match string {
    "hello" ..= "world" => {}
    _ => {}
}

// This is a more general version, using a guard:
match string {
    s if s >= "hello" && s <= "world" => {}
    _ => {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0029.html


---

## E0030 — When matching against a range, the compiler verifies that the range is
non-empty. Range patterns include both end-points, so this is equivalent to
requiring the start of the range to be less than or equal to the end of the
range.

When matching against a range, the compiler verifies that the range is non-empty. Range patterns include both end-points, so this is equivalent to requiring the start of the range to be less than or equal to the end of the range.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
match 5u32 {
    // This range is ok, albeit pointless.
    1 ..= 1 => {}
    // This range is empty, and the compiler can tell.
    1000 ..= 5 => {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0030.html


---

## E0033 — A trait type has been dereferenced.

A trait type has been dereferenced. A pointer to a trait type cannot be implicitly dereferenced by a pattern. Every trait defines a type, but because the size of trait implementers isn’t fixed, this type has no compile-time size. Therefore, all accesses to trait types must be through pointers. If you encounter this error you should try to avoid dereferencing the pointer. You can read more about trait objects in the Trait Objects section of the Reference.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait SomeTrait { fn method_one(&self){} fn method_two(&self){} }
impl<T> SomeTrait for T {}
let trait_obj: &SomeTrait = &"some_value";

// This tries to implicitly dereference to create an unsized local variable.
let &invalid = trait_obj;

// You can call methods without binding to the value being pointed at.
trait_obj.method_one();
trait_obj.method_two();
}
```

出處：https://doc.rust-lang.org/error_codes/E0033.html


---

## E0034 — The compiler doesn’t know what method to call because more than one method
has the same prototype.

The compiler doesn’t know what method to call because more than one method has the same prototype. To avoid this error, you have to keep only one of them and remove the others. So let’s take our example and fix it: However, a better solution would be using fully explicit naming of type and trait: One last example:

**錯誤範例**
```rust
struct Test;

trait Trait1 {
    fn foo();
}

trait Trait2 {
    fn foo();
}

impl Trait1 for Test { fn foo() {} }
impl Trait2 for Test { fn foo() {} }

fn main() {
    Test::foo() // error, which foo() to call?
}
```

**正解**
```rust
struct Test;

trait Trait1 {
    fn foo();
}

impl Trait1 for Test { fn foo() {} }

fn main() {
    Test::foo() // and now that's good!
}
```

出處：https://doc.rust-lang.org/error_codes/E0034.html


---

## E0038 — For any given trait Trait there may be a related type called the trait
object type which is typically written as dyn Trait. In earlier editions of
Rust, trait object types were written as plain Trait (just the name of the
trait, written in type positions) but this was a bit too confusing, so we now


For any given trait Trait there may be a related type called the trait object type which is typically written as dyn Trait. In earlier editions of Rust, trait object types were written as plain Trait (just the name of the trait, written in type positions) but this was a bit too confusing, so we now write dyn Trait. Some traits are not allowed to be used as trait object types. The traits that are allowed to be used as trait object types are called “dyn-compatible”1 traits. Attempting to use a trait object type for a trait that is not dyn-compatible will trigger error E0038. Two general aspects of trait object types give rise to the restrictions: Trait object types are dynamically sized types (DSTs), and trait objects of these types can only be accessed through pointers, such as &dyn Trait or Box<dyn Trait>. The size of such a pointer is known, but the size of the dyn Trait object pointed-to by the pointer is opaque to code working with it, and different trait objects with the same trait object type may have different sizes. The pointer used to access a trait object is paired with an extra pointer to a “virtual method table” or “vtable”, which is used to implement dynamic dispatch to the object’s implementations of the trait’s methods. There is a single such vtable for each trait implementation, but different trait objects with the same trait object type may point to vtables from different implementations. The specific conditions that violate dyn-compatibility follow, most of which relate to missing size information and vtable polymorphism arising from these aspects. The trait requires Self: Sized Traits that are declared as Trait: Sized or which otherwise inherit a constraint of Self:Sized are not dyn-compatible. The reasoning behind this is somewhat subtle. It derives from the fact that Rust requires (and defines) that every trait object type dyn Trait automatically implements Trait. Rust does this to simplify error reporting and ease interoperation between static a

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Trait {
    fn foo(&self) -> Self;
}

fn call_foo(x: Box<dyn Trait>) {
    let y = x.foo(); // What type is y?
    // ...
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Trait {
}

fn static_foo<T:Trait + ?Sized>(b: &T) {
}

fn dynamic_bar(a: &dyn Trait) {
    static_foo(a)
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0038.html


---

## E0040 — It is not allowed to manually call destructors in Rust.

It is not allowed to manually call destructors in Rust. It is unnecessary to do this since drop is called automatically whenever a value goes out of scope. However, if you really need to drop a value by hand, you can use the std::mem::drop function:

**錯誤範例**
```rust
struct Foo {
    x: i32,
}

impl Drop for Foo {
    fn drop(&mut self) {
        println!("kaboom");
    }
}

fn main() {
    let mut x = Foo { x: -7 };
    x.drop(); // error: explicit use of destructor method
}
```

**正解**
```rust
struct Foo {
    x: i32,
}
impl Drop for Foo {
    fn drop(&mut self) {
        println!("kaboom");
    }
}
fn main() {
    let mut x = Foo { x: -7 };
    drop(x); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0040.html


---

## E0044 — You cannot use type or const parameters on foreign items.

You cannot use type or const parameters on foreign items. Example of erroneous code: To fix this, replace the generic parameter with the specializations that you need:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
extern "C" { fn some_func<T>(x: T); }
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
extern "C" { fn some_func_i32(x: i32); }
extern "C" { fn some_func_i64(x: i64); }
}
```

出處：https://doc.rust-lang.org/error_codes/E0044.html


---

## E0045 — Variadic parameters have been used on a non-C ABI function.

Variadic parameters have been used on a non-C ABI function. Rust only supports variadic parameters for interoperability with C code in its FFI. As such, variadic parameters can only be used with functions which are using the C ABI. To fix such code, put them in an extern “C” block:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
extern "Rust" {
    fn foo(x: u8, ...); // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
extern "C" {
    fn foo (x: u8, ...);
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0045.html


---

## E0046 — Items are missing in a trait implementation.

Items are missing in a trait implementation. When trying to make some type implement a trait Foo, you must, at minimum, provide implementations for all of Foo’s required methods (meaning the methods that do not have default implementations), as well as any required trait items like associated types or constants. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo();
}

struct Bar;

impl Foo for Bar {}
// error: not all trait items implemented, missing: `foo`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo();
}

struct Bar;

impl Foo for Bar {
    fn foo() {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0046.html


---

## E0049 — An attempted implementation of a trait method has the wrong number of type or
const parameters.

An attempted implementation of a trait method has the wrong number of type or const parameters. For example, the Foo trait has a method foo with a type parameter T, but the implementation of foo for the type Bar is missing this parameter. To fix this error, they must have the same type parameters:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo<T: Default>(x: T) -> Self;
}

struct Bar;

// error: method `foo` has 0 type parameters but its trait declaration has 1
// type parameter
impl Foo for Bar {
    fn foo(x: bool) -> Self { Bar }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo<T: Default>(x: T) -> Self;
}

struct Bar;

impl Foo for Bar {
    fn foo<T: Default>(x: T) -> Self { // ok!
        Bar
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0049.html


---

## E0050 — An attempted implementation of a trait method has the wrong number of function
parameters.

An attempted implementation of a trait method has the wrong number of function parameters. For example, the Foo trait has a method foo with two function parameters (&self and u8), but the implementation of foo for the type Bar omits the u8 parameter. To fix this error, they must have the same parameters:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo(&self, x: u8) -> bool;
}

struct Bar;

// error: method `foo` has 1 parameter but the declaration in trait `Foo::foo`
// has 2
impl Foo for Bar {
    fn foo(&self) -> bool { true }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo(&self, x: u8) -> bool;
}

struct Bar;

impl Foo for Bar {
    fn foo(&self, x: u8) -> bool { // ok!
        true
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0050.html


---

## E0053 — The parameters of any trait method must match between a trait implementation
and the trait definition.

The parameters of any trait method must match between a trait implementation and the trait definition.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo(x: u16);
    fn bar(&self);
}

struct Bar;

impl Foo for Bar {
    // error, expected u16, found i16
    fn foo(x: i16) { }

    // error, types differ in mutability
    fn bar(&mut self) { }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0053.html


---

## E0054 — It is not allowed to cast to a bool.

It is not allowed to cast to a bool. If you are trying to cast a numeric type to a bool, you can compare it with zero instead:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = 5;

// Not allowed, won't compile
let x_is_nonzero = x as bool;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = 5;

// Ok
let x_is_nonzero = x != 0;
}
```

出處：https://doc.rust-lang.org/error_codes/E0054.html


---

## E0055 — During a method call, a value is automatically dereferenced as many times as
needed to make the value’s type match the method’s receiver. The catch is that
the compiler will only attempt to dereference a number of times up to the
recursion limit (which can be set via the recursion_limit attribute).

During a method call, a value is automatically dereferenced as many times as needed to make the value’s type match the method’s receiver. The catch is that the compiler will only attempt to dereference a number of times up to the recursion limit (which can be set via the recursion_limit attribute). For a somewhat artificial example: One fix may be to increase the recursion limit. Note that it is possible to create an infinite recursion of dereferencing, in which case the only fix is to somehow break the recursion.

**錯誤範例**
```rust
#![recursion_limit="4"]

struct Foo;

impl Foo {
    fn foo(&self) {}
}

fn main() {
    let foo = Foo;
    let ref_foo = &&&&&Foo;

    // error, reached the recursion limit while auto-dereferencing `&&&&&Foo`
    ref_foo.foo();
}
```

出處：https://doc.rust-lang.org/error_codes/E0055.html


---

## E0057 — An invalid number of arguments was given when calling a closure.

An invalid number of arguments was given when calling a closure. When invoking closures or other implementations of the function traits Fn, FnMut or FnOnce using call notation, the number of parameters passed to the function must match its definition. A generic function must be treated similarly:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let f = |x| x * 3;
let a = f();        // invalid, too few parameters
let b = f(4);       // this works!
let c = f(2, 3);    // invalid, too many parameters
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo<F: Fn()>(f: F) {
    f(); // this is valid, but f(3) would not work
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0057.html


---

## E0059 — The built-in function traits are generic over a tuple of the function arguments.
If one uses angle-bracket notation (Fn<(T,), Output=U>) instead of parentheses
(Fn(T) -> U) to denote the function trait, the type parameter should be a
tuple. Otherwise function call notation cannot be used and the tra

The built-in function traits are generic over a tuple of the function arguments. If one uses angle-bracket notation (Fn<(T,), Output=U>) instead of parentheses (Fn(T) -> U) to denote the function trait, the type parameter should be a tuple. Otherwise function call notation cannot be used and the trait will not be implemented by closures. The most likely source of this error is using angle-bracket notation without wrapping the function argument type into a tuple, for example: It can be fixed by adjusting the trait bound like this: Note that (T,) always denotes the type of a 1-tuple containing an element of type T. The comma is necessary for syntactic disambiguation.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(unboxed_closures)]

fn main() {
fn foo<F: Fn<i32>>(f: F) -> F::Output { f(3) }
}
```

**正解**
```rust
#![allow(unused)]
#![feature(unboxed_closures)]

fn main() {
fn foo<F: Fn<(i32,)>>(f: F) -> F::Output { f(3) }
}
```

出處：https://doc.rust-lang.org/error_codes/E0059.html


---

## E0060 — External C functions are allowed to be variadic. However, a variadic function
takes a minimum number of arguments. For example, consider C’s variadic printf
function:

External C functions are allowed to be variadic. However, a variadic function takes a minimum number of arguments. For example, consider C’s variadic printf function: Using this declaration, it must be called with at least one argument, so simply calling printf() is invalid. But the following uses are allowed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::os::raw::{c_char, c_int};

extern "C" {
    fn printf(_: *const c_char, ...) -> c_int;
}

unsafe { printf(); } // error!
}
```

**正解**
```rust
use std::os::raw::{c_char, c_int};
#[cfg_attr(all(windows, target_env = "msvc"),
           link(name = "legacy_stdio_definitions",
                kind = "static", modifiers = "-bundle"))]
extern "C" { fn printf(_: *const c_char, ...) -> c_int; }
fn main() {
unsafe {
    printf(c"test\n".as_ptr());

    printf(c"number = %d\n".as_ptr(), 3);

    printf(c"%d, %d\n".as_ptr(), 10, 5);
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0060.html


---

## E0061 — An invalid number of arguments was passed when calling a function.

An invalid number of arguments was passed when calling a function. The number of arguments passed to a function must match the number of arguments specified in the function signature. For example, a function like: Must always be called with exactly two arguments, e.g., f(2, "test"). Note that Rust does not have a notion of optional function arguments or variadic functions (except for its C-FFI).

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn f(u: i32) {}

f(); // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn f(a: u16, b: &str) {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0061.html


---

## E0062 — A struct’s or struct-like enum variant’s field was specified more than once.

A struct’s or struct-like enum variant’s field was specified more than once. This error indicates that during an attempt to build a struct or struct-like enum variant, one of the fields was specified more than once. Each field should be specified exactly one time. Example:

**錯誤範例**
```rust
struct Foo {
    x: i32,
}

fn main() {
    let x = Foo {
                x: 0,
                x: 0, // error: field `x` specified more than once
            };
}
```

**正解**
```rust
struct Foo {
    x: i32,
}

fn main() {
    let x = Foo { x: 0 }; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0062.html


---

## E0063 — A struct’s or struct-like enum variant’s field was not provided.

A struct’s or struct-like enum variant’s field was not provided. Each field should be specified exactly once. Example:

**錯誤範例**
```rust
struct Foo {
    x: i32,
    y: i32,
}

fn main() {
    let x = Foo { x: 0 }; // error: missing field: `y`
}
```

**正解**
```rust
struct Foo {
    x: i32,
    y: i32,
}

fn main() {
    let x = Foo { x: 0, y: 0 }; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0063.html


---

## E0067 — An invalid left-hand side expression was used on an assignment operation.

An invalid left-hand side expression was used on an assignment operation. You need to have a place expression to be able to assign it something. For example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
12 += 1; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut x: i8 = 12;
x += 1; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0067.html


---

## E0069 — The compiler found a function whose body contains a return; statement but
whose return type is not ().

The compiler found a function whose body contains a return; statement but whose return type is not (). Since return; is just like return ();, there is a mismatch between the function’s return type and the value being returned.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// error
fn foo() -> u8 {
    return;
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0069.html


---

## E0070 — An assignment operator was used on a non-place expression.

An assignment operator was used on a non-place expression. Erroneous code examples: The left-hand side of an assignment operator must be a place expression. A place expression represents a memory location and can be a variable (with optional namespacing), a dereference, an indexing expression or a field reference. More details can be found in the Expressions section of the Reference. And now let’s give working examples:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct SomeStruct {
    x: i32,
    y: i32,
}

const SOME_CONST: i32 = 12;

fn some_other_func() {}

fn some_function() {
    SOME_CONST = 14; // error: a constant value cannot be changed!
    1 = 3; // error: 1 isn't a valid place!
    some_other_func() = 4; // error: we cannot assign value to a function!
    SomeStruct::x = 12; // error: SomeStruct a structure name but it is used
                        //        like a variable!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct SomeStruct {
    x: i32,
    y: i32,
}
let mut s = SomeStruct { x: 0, y: 0 };

s.x = 3; // that's good !

// ...

fn some_func(x: &mut i32) {
    *x = 12; // that's good !
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0070.html


---

## E0071 — A structure-literal syntax was used to create an item that is not a structure
or enum variant.

A structure-literal syntax was used to create an item that is not a structure or enum variant. Example of erroneous code: To fix this, ensure that the name was correctly spelled, and that the correct form of initializer was used. For example, the code above can be fixed to: or:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
type U32 = u32;
let t = U32 { value: 4 }; // error: expected struct, variant or union type,
                          // found builtin type `u32`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
type U32 = u32;
let t: U32 = 4;
}
```

出處：https://doc.rust-lang.org/error_codes/E0071.html


---

## E0072 — A recursive type has infinite size because it doesn’t have an indirection.

A recursive type has infinite size because it doesn’t have an indirection. When defining a recursive struct or enum, any use of the type being defined from inside the definition must occur behind a pointer (like Box, & or Rc). This is because structs and enums must have a well-defined size, and without the pointer, the size of the type would need to be unbounded. In the example, the type cannot have a well-defined size, because it needs to be arbitrarily large (since we would be able to nest ListNodes to any depth). Specifically, One way to fix this is by wrapping ListNode in a Box, like so: This works because Box is a pointer, so its size is well-known.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct ListNode {
    head: u8,
    tail: Option<ListNode>, // error: no indirection here so impossible to
                            //        compute the type's size
}
}
```

**正解**
```rust
size of `ListNode` = 1 byte for `head`
                   + 1 byte for the discriminant of the `Option`
                   + size of `ListNode`
```

出處：https://doc.rust-lang.org/error_codes/E0072.html


---

## E0073 — You cannot define a struct (or enum) Foo that requires an instance of Foo
in order to make a new Foo value.

Note: this error code is no longer emitted by the compiler. You cannot define a struct (or enum) Foo that requires an instance of Foo in order to make a new Foo value. This is because there would be no way a first instance of Foo could be made to initialize another instance! Here’s an example of a struct that has this problem: One fix is to use Option, like so: Now it’s possible to create at least one instance of Foo: Foo { x: None }.

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo { x: Box<Foo> } // error
}
```

出處：https://doc.rust-lang.org/error_codes/E0073.html


---

## E0074 — When using the #[simd] attribute on a tuple struct, the components of the
tuple struct must all be of a concrete, nongeneric type so the compiler can
reason about how to use SIMD with them.

Note: this error code is no longer emitted by the compiler. When using the #[simd] attribute on a tuple struct, the components of the tuple struct must all be of a concrete, nongeneric type so the compiler can reason about how to use SIMD with them. This error will occur if the types are generic. This will cause an error: This will not:

**正解**
```rust
#![allow(unused)]
#![feature(repr_simd)]

fn main() {
#[repr(simd)]
struct Bad<T>([T; 4]);
}
```

出處：https://doc.rust-lang.org/error_codes/E0074.html


---

## E0075 — A #[simd] attribute was applied to an empty or multi-field struct.

A #[simd] attribute was applied to an empty or multi-field struct. Erroneous code examples: The #[simd] attribute can only be applied to a single-field struct, because the one field must be the array of values in the vector. Fixed example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(repr_simd)]

fn main() {
#[repr(simd)]
struct Bad; // error!
}
```

**正解**
```rust
#![allow(unused)]
#![feature(repr_simd)]

fn main() {
#[repr(simd)]
struct Good([u32; 2]); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0075.html


---

## E0076 — The type of the field in a tuple struct isn’t an array when using the #[simd]
attribute.

The type of the field in a tuple struct isn’t an array when using the #[simd] attribute. When using the #[simd] attribute to automatically use SIMD operations in tuple structs, if you want a single-lane vector then the field must be a 1-element array, or the compiler will trigger this error. Fixed example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(repr_simd)]

fn main() {
#[repr(simd)]
struct Bad(u16); // error!
}
```

**正解**
```rust
#![allow(unused)]
#![feature(repr_simd)]

fn main() {
#[repr(simd)]
struct Good([u16; 1]); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0076.html


---

## E0077 — A tuple struct’s element isn’t a machine type when using the #[simd]
attribute.

A tuple struct’s element isn’t a machine type when using the #[simd] attribute. When using the #[simd] attribute on a tuple struct, the elements in the tuple must be machine types so SIMD operations can be applied to them. Fixed example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(repr_simd)]

fn main() {
#[repr(simd)]
struct Bad([String; 2]); // error!
}
```

**正解**
```rust
#![allow(unused)]
#![feature(repr_simd)]

fn main() {
#[repr(simd)]
struct Good([u32; 4]); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0077.html


---

## E0080 — A constant value failed to get evaluated.

A constant value failed to get evaluated. This error indicates that the compiler was unable to sensibly evaluate a constant expression that had to be evaluated. Attempting to divide by 0 or causing an integer overflow are two ways to induce this error. Ensure that the expressions given can be evaluated as the desired integer type. See the Discriminants section of the Reference for more information about setting custom integer types on enums using the repr attribute.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Enum {
    X = (1 << 500),
    Y = (1 / 0),
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0080.html


---

## E0081 — A discriminant value is present more than once.

A discriminant value is present more than once. Enum discriminants are used to differentiate enum variants stored in memory. This error indicates that the same value was used for two or more variants, making it impossible to distinguish them. Note that variants without a manually specified discriminant are numbered from top to bottom starting from 0, so clashes can occur with seemingly unrelated variants. Here X will have already been specified the discriminant 0 by the time Y is encountered, so a conflict occurs.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Enum {
    P = 3,
    X = 3, // error!
    Y = 5,
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Enum {
    P,
    X = 3, // ok!
    Y = 5,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0081.html


---

## E0084 — An unsupported representation was attempted on a zero-variant enum.

An unsupported representation was attempted on a zero-variant enum. It is impossible to define an integer type to be used to represent zero-variant enum values because there are no zero-variant enum values. There is no way to construct an instance of the following type using only safe code. So you have two solutions. Either you add variants in your enum: or you remove the integer representation of your enum:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(i32)]
enum NightsWatch {} // error: unsupported representation for zero-variant enum
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(i32)]
enum NightsWatch {
    JonSnow,
    Commander,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0084.html


---

## E0087 — Too many type arguments were supplied for a function.

Note: this error code is no longer emitted by the compiler. Too many type arguments were supplied for a function. For example: The number of supplied arguments must exactly match the number of defined type parameters.

**錯誤範例**
```rust
fn foo<T>() {}

fn main() {
    foo::<f64, bool>(); // error: wrong number of type arguments:
                        //        expected 1, found 2
}
```

出處：https://doc.rust-lang.org/error_codes/E0087.html


---

## E0088 — You gave too many lifetime arguments.

Note: this error code is no longer emitted by the compiler. You gave too many lifetime arguments. Please check you give the right number of lifetime arguments. Example: It’s also important to note that the Rust compiler can generally determine the lifetime by itself. Example:

**錯誤範例**
```rust
fn f() {}

fn main() {
    f::<'static>() // error: wrong number of lifetime arguments:
                   //        expected 0, found 1
}
```

**正解**
```rust
fn f() {}

fn main() {
    f() // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0088.html


---

## E0089 — Too few type arguments were supplied for a function.

Note: this error code is no longer emitted by the compiler. Too few type arguments were supplied for a function. For example: Note that if a function takes multiple type arguments but you want the compiler to infer some of them, you can use type placeholders:

**錯誤範例**
```rust
fn foo<T, U>() {}

fn main() {
    foo::<f64>(); // error: wrong number of type arguments: expected 2, found 1
}
```

出處：https://doc.rust-lang.org/error_codes/E0089.html


---

## E0090 — You gave too few lifetime arguments.

Note: this error code is no longer emitted by the compiler. You gave too few lifetime arguments. Example: Please check you give the right number of lifetime arguments. Example:

**錯誤範例**
```rust
fn foo<'a: 'b, 'b: 'a>() {}

fn main() {
    foo::<'static>(); // error: wrong number of lifetime arguments:
                      //        expected 2, found 1
}
```

**正解**
```rust
fn foo<'a: 'b, 'b: 'a>() {}

fn main() {
    foo::<'static, 'static>();
}
```

出處：https://doc.rust-lang.org/error_codes/E0090.html


---

## E0091 — An unnecessary type parameter was given in a type alias.

An unnecessary type parameter was given in a type alias. Please check you didn’t write too many parameters. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
type Foo<T> = u32; // error: type parameter `T` is never used
// or:
type Foo<A, B> = Box<A>; // error: type parameter `B` is never used
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
type Foo = u32; // ok!
type Foo2<A> = Box<A>; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0091.html


---

## E0092 — An undefined atomic operation function was declared.

Note: this error code is no longer emitted by the compiler. An undefined atomic operation function was declared. Please check you didn’t make a mistake in the function’s name. All intrinsic functions are defined in library/core/src/intrinsics in the Rust source code.

**正解**
```rust
#![feature(intrinsics)]
#![allow(internal_features)]

#[rustc_intrinsic]
unsafe fn atomic_foo(); // error: unrecognized atomic operation
                        //        function
```

出處：https://doc.rust-lang.org/error_codes/E0092.html


---

## E0093 — An unknown intrinsic function was declared.

An unknown intrinsic function was declared. Please check you didn’t make a mistake in the function’s name. All intrinsic functions are defined in library/core/src/intrinsics in the Rust source code.

**錯誤範例**
```rust
#![feature(intrinsics)]
#![allow(internal_features)]

#[rustc_intrinsic]
unsafe fn foo(); // error: unrecognized intrinsic function: `foo`

fn main() {
    unsafe {
        foo();
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0093.html


---

## E0094 — An invalid number of generic parameters was passed to an intrinsic function.

An invalid number of generic parameters was passed to an intrinsic function. Please check that you provided the right number of type parameters and verify with the function declaration in the Rust source code. Example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(intrinsics)]
#![allow(internal_features)]

fn main() {
#[rustc_intrinsic]
fn size_of<T, U>() -> usize; // error: intrinsic has wrong number
                             //        of type parameters
}
```

**正解**
```rust
#![allow(unused)]
#![feature(intrinsics)]
#![allow(internal_features)]

fn main() {
#[rustc_intrinsic]
fn size_of<T>() -> usize; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0094.html


---

## E0106 — This error indicates that a lifetime is missing from a type. If it is an error
inside a function signature, the problem may be with failing to adhere to the
lifetime elision rules (see below).

This error indicates that a lifetime is missing from a type. If it is an error inside a function signature, the problem may be with failing to adhere to the lifetime elision rules (see below). Erroneous code examples: Lifetime elision is a special, limited kind of inference for lifetimes in function signatures which allows you to leave out lifetimes in certain cases. For more background on lifetime elision see the book. The lifetime elision rules require that any function signature with an elided output lifetime must either have: exactly one input lifetime or, multiple input lifetimes, but the function must also be a method with a &self or &mut self receiver In the first case, the output lifetime is inferred to be the same as the unique input lifetime. In the second case, the lifetime is instead inferred to be the same as the lifetime on &self or &mut self. Here are some examples of elision errors:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo1 { x: &bool }
              // ^ expected lifetime parameter
struct Foo2<'a> { x: &'a bool } // correct

struct Bar1 { x: Foo2 }
              // ^^^^ expected lifetime parameter
struct Bar2<'a> { x: Foo2<'a> } // correct

enum Baz1 { A(u8), B(&bool), }
                  // ^ expected lifetime parameter
enum Baz2<'a> { A(u8), B(&'a bool), } // correct

type MyStr1 = &str;
           // ^ expected lifetime parameter
type MyStr2<'a> = &'a str; // correct
}
```

出處：https://doc.rust-lang.org/error_codes/E0106.html


---

## E0107 — An incorrect number of generic arguments was provided.

An incorrect number of generic arguments was provided. When using/declaring an item with generic arguments, you must provide the exact same number:

**錯誤範例**
```rust
struct Foo<T> { x: T }

struct Bar { x: Foo }             // error: wrong number of type arguments:
                                  //        expected 1, found 0
struct Baz<S, T> { x: Foo<S, T> } // error: wrong number of type arguments:
                                  //        expected 1, found 2

fn foo<T, U>(x: T, y: U) {}
fn f() {}

fn main() {
    let x: bool = true;
    foo::<bool>(x);                 // error: wrong number of type arguments:
                                    //        expected 2, found 1
    foo::<bool, i32, i32>(x, 2, 4); // error: wrong number of type arguments:
                                    //        expected 2, found 3
    f::<'static>();                 // error: wrong number of lifetime arguments
                                    //        expected 0, found 1
}
```

**正解**
```rust
struct Foo<T> { x: T }

struct Bar<T> { x: Foo<T> }               // ok!
struct Baz<S, T> { x: Foo<S>, y: Foo<T> } // ok!

fn foo<T, U>(x: T, y: U) {}
fn f() {}

fn main() {
    let x: bool = true;
    foo::<bool, u32>(x, 12);              // ok!
    f();                                  // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0107.html


---

## E0109 — You tried to provide a generic argument to a type which doesn’t need it.

You tried to provide a generic argument to a type which doesn’t need it. Check that you used the correct argument and that the definition is correct. Example: Note that generic arguments for enum variant constructors go after the variant, not after the enum. For example, you would write Option::None::<u32>, rather than Option::<u32>::None.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
type X = u32<i32>; // error: type arguments are not allowed for this type
type Y = bool<'static>; // error: lifetime parameters are not allowed on
                        //        this type
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
type X = u32; // ok!
type Y = bool; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0109.html


---

## E0110 — You tried to provide a lifetime to a type which doesn’t need it.

Note: this error code is no longer emitted by the compiler. You tried to provide a lifetime to a type which doesn’t need it. See E0109 for more details.

出處：https://doc.rust-lang.org/error_codes/E0110.html


---

## E0116 — An inherent implementation was defined for a type outside the current crate.

An inherent implementation was defined for a type outside the current crate. You can only define an inherent implementation for a type in the same crate where the type was defined. For example, an impl block as above is not allowed since Vec is defined in the standard library. To fix this problem, you can either: define a trait that has the desired associated functions/types/constants and implement the trait for the type in question define a new type wrapping the type and define an implementation on the new type Note that using the type keyword does not work here because type only introduces a type alias:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
impl Vec<u8> { } // error
}
```

出處：https://doc.rust-lang.org/error_codes/E0116.html


---

## E0117 — Only traits defined in the current crate can be implemented for arbitrary types.

Only traits defined in the current crate can be implemented for arbitrary types. This error indicates a violation of one of Rust’s orphan rules for trait implementations. The rule prohibits any implementation of a foreign trait (a trait defined in another crate) where the type that is implementing the trait is foreign all of the parameters being passed to the trait (if there are any) are also foreign. To avoid this kind of error, ensure that at least one local type is referenced by the impl: Alternatively, define a trait locally and implement that instead: For information on the design of the orphan rules, see RFC 1023.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
impl Drop for u32 {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
pub struct Foo; // you define your type in your crate

impl Drop for Foo { // and you can implement the trait on it!
    // code of trait implementation here
  fn drop(&mut self) { }
}

impl From<Foo> for i32 { // or you use a type from your crate as
                         // a type parameter
    fn from(i: Foo) -> i32 {
        0
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0117.html


---

## E0118 — An inherent implementation was defined for something which isn’t a struct,
enum, union, or trait object.

An inherent implementation was defined for something which isn’t a struct, enum, union, or trait object. To fix this error, please implement a trait on the type or wrap it in a struct. Example: Alternatively, you can create a newtype. A newtype is a wrapping tuple-struct. For example, NewType is a newtype over Foo in struct NewType(Foo). Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
impl<T> T { // error: no nominal type found for inherent implementation
    fn get_state(&self) -> String {
        // ...
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
// we create a trait here
trait LiveLongAndProsper {
    fn get_state(&self) -> String;
}

// and now you can implement it on T
impl<T> LiveLongAndProsper for T {
    fn get_state(&self) -> String {
        "He's dead, Jim!".to_owned()
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0118.html


---

## E0119 — There are conflicting trait implementations for the same type.

There are conflicting trait implementations for the same type. When looking for the implementation for the trait, the compiler finds both the impl<T> MyTrait for T where T is all types and the impl MyTrait for Foo. Since a trait cannot be implemented multiple times, this is an error. So, when you write: This makes the trait implemented on all types in the scope. So if you try to implement it on another one after that, the implementations will conflict. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait MyTrait {
    fn get(&self) -> usize;
}

impl<T> MyTrait for T {
    fn get(&self) -> usize { 0 }
}

struct Foo {
    value: usize
}

impl MyTrait for Foo { // error: conflicting implementations of trait
                       //        `MyTrait` for type `Foo`
    fn get(&self) -> usize { self.value }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait MyTrait {
    fn get(&self) -> usize;
}

impl<T> MyTrait for T {
    fn get(&self) -> usize { 0 }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0119.html


---

## E0120 — Drop was implemented on a trait object or reference, which is not allowed;
only structs, enums, and unions can implement Drop.

Drop was implemented on a trait object or reference, which is not allowed; only structs, enums, and unions can implement Drop. Erroneous code examples: A workaround for traits is to create a wrapper struct with a generic type, add a trait bound to the type, and implement Drop on the wrapper: Alternatively, the Drop wrapper can contain the trait object:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait MyTrait {}

impl Drop for MyTrait {
    fn drop(&mut self) {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait MyTrait {}
struct MyWrapper<T: MyTrait> { foo: T }

impl <T: MyTrait> Drop for MyWrapper<T> {
    fn drop(&mut self) {}
}

}
```

出處：https://doc.rust-lang.org/error_codes/E0120.html


---

## E0121 — The type placeholder _ was used within a type on an item’s signature.

The type placeholder _ was used within a type on an item’s signature. In those cases, you need to provide the type explicitly: The type placeholder _ can be used outside item’s signature as follows:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo() -> _ { 5 } // error

static BAR: _ = "test"; // error
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo() -> i32 { 5 } // ok!

static BAR: &str = "test"; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0121.html


---

## E0124 — A struct was declared with two fields having the same name.

A struct was declared with two fields having the same name. Please verify that the field names have been correctly spelled. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    field1: i32,
    field1: i32, // error: field is already declared
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    field1: i32,
    field2: i32, // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0124.html


---

## E0128 — A type parameter with default value is using forward declared identifier.

A type parameter with default value is using forward declared identifier. Type parameter defaults can only use parameters that occur before them. Since type parameters are evaluated in-order, this issue could be fixed by doing: Please also verify that this wasn’t because of a name-clash and rename the type parameter if so.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<T = U, U = ()> {
    field1: T,
    field2: U,
}
// error: generic parameters with a default cannot use forward declared
//        identifiers
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<U = (), T = U> {
    field1: T,
    field2: U,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0128.html


---

## E0130 — A pattern was declared as an argument in a foreign function declaration.

A pattern was declared as an argument in a foreign function declaration. To fix this error, replace the pattern argument with a regular one. Example: Or:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
extern "C" {
    fn foo((a, b): (u32, u32)); // error: patterns aren't allowed in foreign
                                //        function declarations
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct SomeStruct {
    a: u32,
    b: u32,
}

extern "C" {
    fn foo(s: SomeStruct); // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0130.html


---

## E0131 — The main function was defined with generic parameters.

The main function was defined with generic parameters. It is not possible to define the main function with generic parameters. It must not take any arguments.

**錯誤範例**
```rust
fn main<T>() { // error: main function is not allowed to have generic parameters
}
```

出處：https://doc.rust-lang.org/error_codes/E0131.html


---

## E0132 — A function with the start attribute was declared with type parameters.

Note: this error code is no longer emitted by the compiler. A function with the start attribute was declared with type parameters.

出處：https://doc.rust-lang.org/error_codes/E0132.html


---

## E0133 — Unsafe code was used outside of an unsafe block.

Unsafe code was used outside of an unsafe block. Using unsafe functionality is potentially dangerous and disallowed by safety checks. Examples: Dereferencing raw pointers Calling functions via FFI Calling functions marked unsafe These safety checks can be relaxed for a section of the code by wrapping the unsafe instructions with an unsafe block. For instance: See the unsafe section of the Book for more details. Unsafe code in functions Unsafe code is currently accepted in unsafe functions, but that is being phased out in favor of requiring unsafe blocks here too. Linting against this is controlled via the unsafe_op_in_unsafe_fn lint, which is warn by default in the 2024 edition and allow by default in earlier editions.

**錯誤範例**
```rust
unsafe fn f() { return; } // This is the unsafe code

fn main() {
    f(); // error: call to unsafe function requires unsafe function or block
}
```

**正解**
```rust
unsafe fn f() { return; }

fn main() {
    unsafe { f(); } // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0133.html


---

## E0136 — More than one main function was found.

Note: this error code is no longer emitted by the compiler. More than one main function was found. A binary can only have one entry point, and by default that entry point is the main() function. If there are multiple instances of this function, please rename one of them.

**錯誤範例**
```rust
fn main() {
    // ...
}

// ...

fn main() { // error!
    // ...
}
```

出處：https://doc.rust-lang.org/error_codes/E0136.html


---

## E0137 — More than one function was declared with the #[main] attribute.

Note: this error code is no longer emitted by the compiler. More than one function was declared with the #[main] attribute. This error indicates that the compiler found multiple functions with the #[main] attribute. This is an error because there must be a unique entry point into a Rust program. Example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(main)]

fn main() {
#[main]
fn foo() {}

#[main]
fn f() {} // error: multiple functions with a `#[main]` attribute
}
```

出處：https://doc.rust-lang.org/error_codes/E0137.html


---

## E0138 — More than one function was declared with the #[start] attribute.

Note: this error code is no longer emitted by the compiler. More than one function was declared with the #[start] attribute.

出處：https://doc.rust-lang.org/error_codes/E0138.html


---

## E0139 — There are various restrictions on transmuting between types in Rust; for example
types being transmuted must have the same size.

Note: this error code is no longer emitted by the compiler. There are various restrictions on transmuting between types in Rust; for example types being transmuted must have the same size. To apply all these restrictions, the compiler must know the exact types that may be transmuted. When type parameters are involved, this cannot always be done. So, for example, the following is not allowed: In this specific case there’s a good chance that the transmute is harmless (but this is not guaranteed by Rust). However, when alignment and enum optimizations come into the picture, it’s quite likely that the sizes may or may not match with different type parameter instantiations. It’s not possible to check this for all possible types, so transmute() simply only accepts types without any uninstantiated type parameters. If you need this, there’s a good chance you’re doing something wrong. Keep in mind that Rust doesn’t guarantee much about the layout of different structs (even two structs with identical declarations may have different layouts). If there is a solution that avoids the transmute entirely, try it instead. If it’s possible, hand-monomorphize the code by writing the function for each possible type instantiation. It’s possible to use traits to do this cleanly, for example: Each impl will be checked for a size match in the transmute as usual, and since there are no unbound type parameters involved, this should compile unless there is a size mismatch in one of the impls. It is also possible to manually transmute: Note that this does not move v (unlike transmute), and may need a call to mem::forget(v) in case you want to avoid destructors being called.

**正解**
```rust
#![allow(unused)]
fn main() {
use std::mem::transmute;

struct Foo<T>(Vec<T>);

fn foo<T>(x: Vec<T>) {
    // we are transmuting between Vec<T> and Foo<F> here
    let y: Foo<T> = unsafe { transmute(x) };
    // do something with y
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0139.html


---

## E0152 — A lang item was redefined.

A lang item was redefined. Lang items are already implemented in the standard library. Unless you are writing a free-standing application (e.g., a kernel), you do not need to provide them yourself. You can build a free-standing crate by adding #![no_std] to the crate attributes: See also this section of the Rustonomicon.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(lang_items)]

fn main() {
#[lang = "owned_box"]
struct Foo<T>(T); // error: duplicate lang item found: `owned_box`
}
```

**正解**
```rust
#![no_std]
```

出處：https://doc.rust-lang.org/error_codes/E0152.html


---

## E0154 — Imports (use statements) are not allowed after non-item statements, such as
variable declarations and expression statements.

Note: this error code is no longer emitted by the compiler. Imports (use statements) are not allowed after non-item statements, such as variable declarations and expression statements. Here is an example that demonstrates the error: The solution is to declare the imports at the top of the block, function, or file. Here is the previous example again, with the correct order: See the Declaration Statements section of the reference for more information about what constitutes an item declaration and what does not.

**正解**
```rust
#![allow(unused)]
fn main() {
fn f() {
    // Variable declaration before import
    let x = 0;
    use std::io::Read;
    // ...
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0154.html


---

## E0158 — A generic parameter or static has been referenced in a pattern.

A generic parameter or static has been referenced in a pattern. Generic parameters cannot be referenced in patterns because it is impossible for the compiler to prove exhaustiveness (that some pattern will always match). Take the above example, because Rust does type checking in the generic method, not the monomorphized specific instance. So because Bar could have theoretically arbitrary implementations, there’s no way to always be sure that A::X is Foo::One. So this code must be rejected. Even if code can be proven exhaustive by a programmer, the compiler cannot currently prove this. The same holds true of statics. If you want to match against a const that depends on a generic parameter or a static, consider using a guard instead:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Foo {
    One,
    Two
}

trait Bar {
    const X: Foo;
}

fn test<A: Bar>(arg: Foo) {
    match arg {
        A::X => println!("A::X"), // error: E0158: constant pattern depends
                                  //        on a generic parameter
        Foo::Two => println!("Two")
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Trait {
    const X: char;
}

static FOO: char = 'j';

fn test<A: Trait, const Y: char>(arg: char) {
    match arg {
        c if c == A::X => println!("A::X"),
        c if c == Y => println!("Y"),
        c if c == FOO => println!("FOO"),
        _ => ()
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0158.html


---

## E0161 — A value was moved whose size was not known at compile time.

A value was moved whose size was not known at compile time. In Rust, you can only move a value when its size is known at compile time. To work around this restriction, consider “hiding” the value behind a reference: either &x or &mut x. Since a reference has a fixed size, this lets you move it around as usual. Example:

**錯誤範例**
```rust
trait Bar {
    fn f(self);
}

impl Bar for i32 {
    fn f(self) {}
}

fn main() {
    let b: Box<dyn Bar> = Box::new(0i32);
    b.f();
    // error: cannot move a value of type dyn Bar: the size of dyn Bar cannot
    //        be statically determined
}
```

**正解**
```rust
trait Bar {
    fn f(&self);
}

impl Bar for i32 {
    fn f(&self) {}
}

fn main() {
    let b: Box<dyn Bar> = Box::new(0i32);
    b.f();
    // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0161.html


---

## E0162 — An if let pattern attempts to match the pattern, and enters the body if the
match was successful.

Note: this error code is no longer emitted by the compiler. An if let pattern attempts to match the pattern, and enters the body if the match was successful. If the match is irrefutable (when it cannot fail to match), use a regular let-binding instead. For instance: Try this instead:

**正解**
```rust
#![allow(unused)]
fn main() {
struct Irrefutable(i32);
let irr = Irrefutable(0);

// This fails to compile because the match is irrefutable.
if let Irrefutable(x) = irr {
    // This body will always be executed.
    // ...
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0162.html


---

## E0164 — Something which is neither a tuple struct nor a tuple variant was used as a
pattern.

Something which is neither a tuple struct nor a tuple variant was used as a pattern. This error means that an attempt was made to match something which is neither a tuple struct nor a tuple variant. Only these two elements are allowed as a pattern:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum A {
    B,
    C,
}

impl A {
    fn new() {}
}

fn bar(foo: A) {
    match foo {
        A::new() => (), // error!
        _ => {}
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum A {
    B,
    C,
}

impl A {
    fn new() {}
}

fn bar(foo: A) {
    match foo {
        A::B => (), // ok!
        _ => {}
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0164.html


---

## E0165 — A while let pattern attempts to match the pattern, and enters the body if the
match was successful.

Note: this error code is no longer emitted by the compiler. A while let pattern attempts to match the pattern, and enters the body if the match was successful. If the match is irrefutable (when it cannot fail to match), use a regular let-binding inside a loop instead. For instance: Try this instead:

**正解**
```rust
struct Irrefutable(i32);
let irr = Irrefutable(0);

// This fails to compile because the match is irrefutable.
while let Irrefutable(x) = irr {
    // ...
}
```

出處：https://doc.rust-lang.org/error_codes/E0165.html


---

## E0170 — A pattern binding is using the same name as one of the variants of a type.

A pattern binding is using the same name as one of the variants of a type. Enum variants are qualified by default. For example, given this type: You would match it using: If you don’t qualify the names, the code will bind new variables named “GET” and “POST” instead. This behavior is likely not what you want, so rustc warns when that happens. Qualified names are good practice, and most code works well with them. But if you prefer them unqualified, you can import the variants into scope: If you want others to be able to import variants from your module directly, use pub use:

**錯誤範例**
```rust
#![deny(warnings)]
enum Method {
    GET,
    POST,
}

fn is_empty(s: Method) -> bool {
    match s {
        GET => true,
        _ => false
    }
}

fn main() {}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Method {
    GET,
    POST,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0170.html


---

## E0178 — The + type operator was used in an ambiguous context.

The + type operator was used in an ambiguous context. In types, the + type operator has low precedence, so it is often necessary to use parentheses: More details can be found in RFC 438.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

struct Bar<'a> {
    x: &'a Foo + 'a,     // error!
    y: &'a mut Foo + 'a, // error!
    z: fn() -> Foo + 'a, // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

struct Bar<'a> {
    x: &'a (Foo + 'a),     // ok!
    y: &'a mut (Foo + 'a), // ok!
    z: fn() -> (Foo + 'a), // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0178.html


---

## E0183 — Manual implementation of a Fn* trait.

Manual implementation of a Fn* trait. Manually implementing Fn, FnMut or FnOnce is unstable and requires #![feature(fn_traits, unboxed_closures)]. The arguments must be a tuple representing the argument list. For more info, see the tracking issue:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct MyClosure {
    foo: i32
}

impl FnOnce<()> for MyClosure {  // error
    type Output = ();
    extern "rust-call" fn call_once(self, args: ()) -> Self::Output {
        println!("{}", self.foo);
    }
}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(fn_traits, unboxed_closures)]

fn main() {
struct MyClosure {
    foo: i32
}

impl FnOnce<()> for MyClosure {  // ok!
    type Output = ();
    extern "rust-call" fn call_once(self, args: ()) -> Self::Output {
        println!("{}", self.foo);
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0183.html


---

## E0184 — The Copy trait was implemented on a type with a Drop implementation.

The Copy trait was implemented on a type with a Drop implementation. Explicitly implementing both Drop and Copy trait on a type is currently disallowed. This feature can make some sense in theory, but the current implementation is incorrect and can lead to memory unsafety (see issue #20126), so it has been disabled for now.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[derive(Copy)]
struct Foo; // error!

impl Drop for Foo {
    fn drop(&mut self) {
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0184.html


---

## E0185 — An associated function for a trait was defined to be static, but an
implementation of the trait declared the same function to be a method (i.e., to
take a self parameter).

An associated function for a trait was defined to be static, but an implementation of the trait declared the same function to be a method (i.e., to take a self parameter). When a type implements a trait’s associated function, it has to use the same signature. So in this case, since Foo::foo does not take any argument and does not return anything, its implementation on Bar should be the same:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo();
}

struct Bar;

impl Foo for Bar {
    // error, method `foo` has a `&self` declaration in the impl, but not in
    // the trait
    fn foo(&self) {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo();
}

struct Bar;

impl Foo for Bar {
    fn foo() {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0185.html


---

## E0186 — An associated function for a trait was defined to be a method (i.e., to take a
self parameter), but an implementation of the trait declared the same function
to be static.

An associated function for a trait was defined to be a method (i.e., to take a self parameter), but an implementation of the trait declared the same function to be static. When a type implements a trait’s associated function, it has to use the same signature. So in this case, since Foo::foo takes self as argument and does not return anything, its implementation on Bar should be the same:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo(&self);
}

struct Bar;

impl Foo for Bar {
    // error, method `foo` has a `&self` declaration in the trait, but not in
    // the impl
    fn foo() {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo(&self);
}

struct Bar;

impl Foo for Bar {
    fn foo(&self) {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0186.html


---

## E0191 — An associated type wasn’t specified for a trait object.

An associated type wasn’t specified for a trait object. Trait objects need to have all associated types specified. Please verify that all associated types of the trait were specified and the correct trait was used. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Trait {
    type Bar;
}

type Foo = dyn Trait; // error: the value of the associated type `Bar` (from
                      //        the trait `Trait`) must be specified
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Trait {
    type Bar;
}

type Foo = dyn Trait<Bar=i32>; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0191.html


---

## E0192 — A negative impl was added on a trait implementation.

Note: this error code is no longer emitted by the compiler. A negative impl was added on a trait implementation. Negative impls are only allowed for auto traits. For more information see the opt-in builtin traits RFC.

**錯誤範例**
```rust
trait Trait {
    type Bar;
}

struct Foo;

impl !Trait for Foo { } //~ ERROR

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0192.html


---

## E0193 — where clauses must use generic type parameters: it does not make sense to use
them otherwise.

Note: this error code is no longer emitted by the compiler. where clauses must use generic type parameters: it does not make sense to use them otherwise. An example causing this error: This use of a where clause is strange - a more common usage would look something like the following: Here, we’re saying that the implementation exists on Wrapper only when the wrapped type T implements Clone. The where clause is important because some types will not implement Clone, and thus will not get this method. In our erroneous example, however, we’re referencing a single concrete type. Since we know for certain that Wrapper<u32> implements Clone, there’s no reason to also specify it in a where clause.

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn bar(&self);
}

#[derive(Copy,Clone)]
struct Wrapper<T> {
    Wrapped: T
}

impl Foo for Wrapper<u32> where Wrapper<u32>: Clone {
    fn bar(&self) { }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0193.html


---

## E0195 — The lifetime parameters of the method do not match the trait declaration.

The lifetime parameters of the method do not match the trait declaration. The lifetime constraint 'b for bar() implementation does not match the trait declaration. Ensure lifetime declarations match exactly in both trait declaration and implementation. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Trait {
    fn bar<'a,'b:'a>(x: &'a str, y: &'b str);
}

struct Foo;

impl Trait for Foo {
    fn bar<'a,'b>(x: &'a str, y: &'b str) {
    // error: lifetime parameters or bounds on method `bar`
    // do not match the trait declaration
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Trait {
    fn t<'a,'b:'a>(x: &'a str, y: &'b str);
}

struct Foo;

impl Trait for Foo {
    fn t<'a,'b:'a>(x: &'a str, y: &'b str) { // ok!
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0195.html


---

## E0197 — An inherent implementation was marked unsafe.

An inherent implementation was marked unsafe. Inherent implementations (one that do not implement a trait but provide methods associated with a type) are always safe because they are not implementing an unsafe trait. Removing the unsafe keyword from the inherent implementation will resolve this error.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

unsafe impl Foo { } // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Foo { } // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0197.html


---

## E0198 — A negative implementation was marked as unsafe.

A negative implementation was marked as unsafe. A negative implementation is one that excludes a type from implementing a particular trait. Not being able to use a trait is always a safe operation, so negative implementations are always safe and never need to be marked as unsafe. This will compile: Please note that negative impls are only allowed for auto traits.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

unsafe impl !Clone for Foo { } // error!
}
```

**正解**
```rust
#![feature(auto_traits)]

struct Foo;

auto trait Enterprise {}

impl !Enterprise for Foo { }
```

出處：https://doc.rust-lang.org/error_codes/E0198.html


---

## E0199 — A trait implementation was marked as unsafe while the trait is safe.

A trait implementation was marked as unsafe while the trait is safe. Safe traits should not have unsafe implementations, therefore marking an implementation for a safe trait unsafe will cause a compiler error. Removing the unsafe marker on the trait noted in the error will resolve this problem:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

trait Bar { }

unsafe impl Bar for Foo { } // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;

trait Bar { }

impl Bar for Foo { } // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0199.html


---

## E0200 — An unsafe trait was implemented without an unsafe implementation.

An unsafe trait was implemented without an unsafe implementation. Unsafe traits must have unsafe implementations. This error occurs when an implementation for an unsafe trait isn’t marked as unsafe. This may be resolved by marking the unsafe implementation as unsafe.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

unsafe trait Bar { }

impl Bar for Foo { } // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;

unsafe trait Bar { }

unsafe impl Bar for Foo { } // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0200.html


---

## E0201 — Two associated items (like methods, associated types, associated functions,
etc.) were defined with the same identifier.

Two associated items (like methods, associated types, associated functions, etc.) were defined with the same identifier. Note, however, that items with the same name are allowed for inherent impl blocks that don’t overlap:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo(u8);

impl Foo {
    fn bar(&self) -> bool { self.0 > 5 }
    fn bar() {} // error: duplicate associated function
}

trait Baz {
    type Quux;
    fn baz(&self) -> bool;
}

impl Baz for Foo {
    type Quux = u32;

    fn baz(&self) -> bool { true }

    // error: duplicate method
    fn baz(&self) -> bool { self.0 > 5 }

    // error: duplicate associated type
    type Quux = u32;
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<T>(T);

impl Foo<u8> {
    fn bar(&self) -> bool { self.0 > 5 }
}

impl Foo<bool> {
    fn bar(&self) -> bool { self.0 }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0201.html


---

## E0203 — Having duplicate relaxed default bounds is unsupported.

Having duplicate relaxed default bounds is unsupported. Here the type parameter T cannot have duplicate relaxed bounds for default trait Sized. This can be fixed by only using one relaxed bound:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Bad<T: ?Sized + ?Sized>{
    inner: T,
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Good<T: ?Sized>{
    inner: T
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0203.html


---

## E0204 — The Copy trait was implemented on a type which contains a field that doesn’t
implement the Copy trait.

The Copy trait was implemented on a type which contains a field that doesn’t implement the Copy trait. The Copy trait is implemented by default only on primitive types. If your type only contains primitive types, you’ll be able to implement Copy on it. Otherwise, it won’t be possible. Here’s another example that will fail: This fails because &mut T is not Copy, even when T is Copy (this differs from the behavior for &T, which is always Copy).

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    foo: Vec<u32>,
}

impl Copy for Foo { } // error!
}
```

出處：https://doc.rust-lang.org/error_codes/E0204.html


---

## E0205 — An attempt to implement the Copy trait for an enum failed because one of the
variants does not implement Copy.

Note: this error code is no longer emitted by the compiler. An attempt to implement the Copy trait for an enum failed because one of the variants does not implement Copy. To fix this, you must implement Copy for the mentioned variant. Note that this may not be possible, as in the example of This fails because Vec<T> does not implement Copy for any T. Here’s another example that will fail: This fails because &mut T is not Copy, even when T is Copy (this differs from the behavior for &T, which is always Copy).

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Foo {
    Bar(Vec<u32>),
    Baz,
}

impl Copy for Foo { }
}
```

出處：https://doc.rust-lang.org/error_codes/E0205.html


---

## E0206 — The Copy trait was implemented on a type which is neither a struct, an
enum, nor a union.

The Copy trait was implemented on a type which is neither a struct, an enum, nor a union. You can only implement Copy for a struct, an enum, or a union. The previous example will fail because &'static mut Bar is not a struct, an enum, or a union.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[derive(Copy, Clone)]
struct Bar;

impl Copy for &'static mut Bar { } // error!
}
```

出處：https://doc.rust-lang.org/error_codes/E0206.html


---

## E0207 — A type, const or lifetime parameter that is specified for impl is not
constrained.

A type, const or lifetime parameter that is specified for impl is not constrained. Any type or const parameter of an impl must meet at least one of the following criteria: it appears in the implementing type of the impl, e.g. impl<T> Foo<T> for a trait impl, it appears in the implemented trait, e.g. impl<T> SomeTrait<T> for Foo it is bound as an associated type, e.g. impl<T, U> SomeTrait for T where T: AnotherTrait<AssocType=U> Any unconstrained lifetime parameter of an impl is not supported if the lifetime parameter is used by an associated type. Error example 1 Suppose we have a struct Foo and we would like to define some methods for it. The previous code example has a definition which leads to a compiler error: The problem is that the parameter T does not appear in the implementing type (Foo) of the impl. In this case, we can fix the error by moving the type parameter from the impl to the method get: Error example 2 As another example, suppose we have a Maker trait and want to establish a type FooMaker that makes Foos: This fails to compile because T does not appear in the trait or in the implementing type. One way to work around this is to introduce a phantom type parameter into FooMaker, like so: Another way is to do away with the associated type in Maker and use an input type parameter instead: Error example 3 Suppose we have a struct Foo and we would like to define some methods for it. The following code example has a definition which leads to a compiler error: The problem is that the const parameter T does not appear in the implementing type (Foo) of the impl. In this case, we can fix the error by moving the type parameter from the impl to the method get: Error example 4 Suppose we have a struct Foo and a struct Bar that uses lifetime 'a. We would like to implement trait Contains for Foo. The trait Contains have the associated type B. The following code example has a definition which leads to a compiler error: Please note that unconstrained lifetime paramete

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl<T: Default> Foo {
    // error: the type parameter `T` is not constrained by the impl trait, self
    // type, or predicates [E0207]
    fn get(&self) -> T {
        <T as Default>::default()
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;

// Move the type parameter from the impl to the method
impl Foo {
    fn get<T: Default>(&self) -> T {
        <T as Default>::default()
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0207.html


---

## E0208 — This error code is internal to the compiler and will not be emitted with normal Rust code.

This error code is internal to the compiler and will not be emitted with normal Rust code. Note: this error code is no longer emitted by the compiler.

出處：https://doc.rust-lang.org/error_codes/E0208.html


---

## E0210 — This error indicates a violation of one of Rust’s orphan rules for trait
implementations. The rule concerns the use of type parameters in an
implementation of a foreign trait (a trait defined in another crate), and
states that type parameters must be “covered” by a local type.

This error indicates a violation of one of Rust’s orphan rules for trait implementations. The rule concerns the use of type parameters in an implementation of a foreign trait (a trait defined in another crate), and states that type parameters must be “covered” by a local type. When implementing a foreign trait for a foreign type, the trait must have one or more type parameters. A type local to your crate must appear before any use of any type parameters. To understand what this means, it is perhaps easier to consider a few examples. If ForeignTrait is a trait defined in some external crate foo, then the following trait impl is an error: To work around this, it can be covered with a local type, MyType: Please note that a type alias is not sufficient. For another example of an error, suppose there’s another trait defined in foo named ForeignTrait2 that takes two type parameters. Then this impl results in the same rule violation: The reason for this is that there are two appearances of type parameter T in the impl header, both as parameters for ForeignTrait2. The first appearance is uncovered, and so runs afoul of the orphan rule. Consider one more example: This only differs from the previous impl in that the parameters T and MyType<T> for ForeignTrait2 have been swapped. This example does not violate the orphan rule; it is permitted. To see why that last example was allowed, you need to understand the general rule. Unfortunately this rule is a bit tricky to state. Consider an impl: where P1, ..., Pm are the type parameters of the impl and T0, ..., Tn are types. One of the types T0, ..., Tn must be a local type (this is another orphan rule, see the explanation for E0117). Both of the following must be true: At least one of the types T0..=Tn must be a local type. Let Ti be the first such type. No uncovered type parameters P1..=Pm may appear in T0..Ti (excluding Ti). For information on the design of the orphan rules, see RFC 2451 and RFC 1023.

**錯誤範例**
```rust
#[cfg(for_demonstration_only)]
extern crate foo;
#[cfg(for_demonstration_only)]
use foo::ForeignTrait;
use std::panic::UnwindSafe as ForeignTrait;

impl<T> ForeignTrait for T { } // error
fn main() {}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::panic::UnwindSafe as ForeignTrait;
struct MyType<T>(T);
impl<T> ForeignTrait for MyType<T> { } // Ok
}
```

出處：https://doc.rust-lang.org/error_codes/E0210.html


---

## E0211 — You used a function or type which doesn’t fit the requirements for where it was
used.

Note: this error code is no longer emitted by the compiler. You used a function or type which doesn’t fit the requirements for where it was used. Erroneous code examples: For the first code example, please check the function definition. Example: The second case example is a bit particular: the main function must always have this definition: They never take parameters and never return types. For the third example, when you match, all patterns must have the same type as the type you’re matching on. Example: And finally, for the last example, only Box<Self>, &Self, Self, or &mut Self work as explicit self parameters. Example:

**錯誤範例**
```rust
#![feature(intrinsics)]
#![allow(internal_features)]

#[rustc_intrinsic]
unsafe fn unreachable(); // error: intrinsic has wrong type

// or:

fn main() -> i32 { 0 }
// error: main function expects type: `fn() {main}`: expected (), found i32

// or:

let x = 1u8;
match x {
    0u8..=3i8 => (),
    // error: mismatched types in range: expected u8, found i8
    _ => ()
}

// or:

use std::rc::Rc;
struct Foo;

impl Foo {
    fn x(self: Rc<Foo>) {}
    // error: mismatched self type: expected `Foo`: expected struct
    //        `Foo`, found struct `alloc::rc::Rc`
}
```

**正解**
```rust
#![allow(unused)]
#![feature(intrinsics)]
#![allow(internal_features)]

fn main() {
#[rustc_intrinsic]
unsafe fn unreachable() -> !; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0211.html


---

## E0212 — Cannot use the associated type of
a trait with uninferred generic parameters.

Cannot use the associated type of a trait with uninferred generic parameters. In this example, we have to instantiate 'x, and we don’t know what lifetime to instantiate it with. To fix this, spell out the precise lifetimes involved. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
pub trait Foo<T> {
    type A;

    fn get(&self, t: T) -> Self::A;
}

fn foo2<I : for<'x> Foo<&'x isize>>(
    field: I::A) {} // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
pub trait Foo<T> {
    type A;

    fn get(&self, t: T) -> Self::A;
}

fn foo3<I : for<'x> Foo<&'x isize>>(
    x: <I as Foo<&isize>>::A) {} // ok!


fn foo4<'a, I : for<'x> Foo<&'x isize>>(
    x: <I as Foo<&'a isize>>::A) {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0212.html


---

## E0214 — A generic type was described using parentheses rather than angle brackets.

A generic type was described using parentheses rather than angle brackets. This is not currently supported: v should be defined as Vec<&str>. Parentheses are currently only used with generic types when defining parameters for Fn-family traits. The previous code example fixed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let v: Vec(&str) = vec!["foo"];
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let v: Vec<&str> = vec!["foo"];
}
```

出處：https://doc.rust-lang.org/error_codes/E0214.html


---

## E0220 — The associated type used was not defined in the trait.

The associated type used was not defined in the trait. Make sure that you have defined the associated type in the trait body. Also, verify that you used the right trait or you didn’t misspell the associated type name. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait T1 {
    type Bar;
}

type Foo = T1<F=i32>; // error: associated type `F` not found for `T1`

// or:

trait T2 {
    type Bar;

    // error: Baz is used but not declared
    fn return_bool(&self, _: &Self::Bar, _: &Self::Baz) -> bool;
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait T1 {
    type Bar;
}

type Foo = T1<Bar=i32>; // ok!

// or:

trait T2 {
    type Bar;
    type Baz; // we declare `Baz` in our trait.

    // and now we can use it here:
    fn return_bool(&self, _: &Self::Bar, _: &Self::Baz) -> bool;
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0220.html


---

## E0221 — An attempt was made to retrieve an associated type, but the type was ambiguous.

An attempt was made to retrieve an associated type, but the type was ambiguous. In this example, Foo defines an associated type A. Bar inherits that type from Foo, and defines another associated type of the same name. As a result, when we attempt to use Self::A, it’s ambiguous whether we mean the A defined by Foo or the one defined by Bar. There are two options to work around this issue. The first is simply to rename one of the types. Alternatively, one can specify the intended type using the following syntax:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait T1 {}
trait T2 {}

trait Foo {
    type A: T1;
}

trait Bar : Foo {
    type A: T2;
    fn do_something() {
        let _: Self::A;
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait T1 {}
trait T2 {}

trait Foo {
    type A: T1;
}

trait Bar : Foo {
    type A: T2;
    fn do_something() {
        let _: <Self as Bar>::A;
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0221.html


---

## E0222 — An attempt was made to constrain an associated type.

An attempt was made to constrain an associated type. In this example, BoxCar has two supertraits: Vehicle and Box. Both of these traits define an associated type Color. BoxCar inherits two types with that name from both supertraits. Because of this, we need to use the fully qualified path syntax to refer to the appropriate Color associated type, either <BoxCar as Vehicle>::Color or <BoxCar as Box>::Color, but this syntax is not allowed to be used in a function signature. In order to encode this kind of constraint, a where clause and a new type parameter are needed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
pub trait Vehicle {
    type Color;
}

pub trait Box {
    type Color;
}

pub trait BoxCar : Box + Vehicle {}

fn dent_object<COLOR>(c: dyn BoxCar<Color=COLOR>) {} // Invalid constraint
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
pub trait Vehicle {
    type Color;
}

pub trait Box {
    type Color;
}

pub trait BoxCar : Box + Vehicle {}

// Introduce a new `CAR` type parameter
fn foo<CAR, COLOR>(
    c: CAR,
) where
    // Bind the type parameter `CAR` to the trait `BoxCar`
    CAR: BoxCar,
    // Further restrict `<BoxCar as Vehicle>::Color` to be the same as the
    // type parameter `COLOR`
    CAR: Vehicle<Color = COLOR>,
    // We can also simultaneously restrict the other trait's associated type
    CAR: Box<Color = COLOR>
{}
}
```

出處：https://doc.rust-lang.org/error_codes/E0222.html


---

## E0223 — An attempt was made to retrieve an associated type, but the type was ambiguous.

An attempt was made to retrieve an associated type, but the type was ambiguous. The problem here is that we’re attempting to take the associated type of X from Trait. Unfortunately, the type of X is not defined, because it’s only made concrete in implementations of the trait. A working version of this code might look like: This syntax specifies that we want the associated type X from Struct’s implementation of Trait. Due to internal limitations of the current compiler implementation we cannot simply use Struct::X.

**錯誤範例**
```rust
trait Trait { type X; }

fn main() {
    let foo: Trait::X;
}
```

**正解**
```rust
trait Trait { type X; }

struct Struct;
impl Trait for Struct {
    type X = u32;
}

fn main() {
    let foo: <Struct as Trait>::X;
}
```

出處：https://doc.rust-lang.org/error_codes/E0223.html


---

## E0224 — A trait object was declared with no traits.

A trait object was declared with no traits. Rust does not currently support this. To solve, ensure that the trait object has at least one trait:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
type Foo = dyn 'static +;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
type Foo = dyn 'static + Copy;
}
```

出處：https://doc.rust-lang.org/error_codes/E0224.html


---

## E0225 — Multiple types were used as bounds for a closure or trait object.

Multiple types were used as bounds for a closure or trait object. Rust does not currently support this. Auto traits such as Send and Sync are an exception to this rule: It’s possible to have bounds of one non-builtin trait, plus any number of auto traits. For example, the following compiles correctly:

**錯誤範例**
```rust
fn main() {
    let _: Box<dyn std::io::Read + std::io::Write>;
}
```

**正解**
```rust
fn main() {
    let _: Box<dyn std::io::Read + Send + Sync>;
}
```

出處：https://doc.rust-lang.org/error_codes/E0225.html


---

## E0226 — More than one explicit lifetime bound was used on a trait object.

More than one explicit lifetime bound was used on a trait object. Example of erroneous code: Here T is a trait object with two explicit lifetime bounds, ’a and ’b. Only a single explicit lifetime bound is permitted on trait objects. To fix this error, consider removing one of the lifetime bounds:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

type T<'a, 'b> = dyn Foo + 'a + 'b; // error: Trait object `arg` has two
                                    //        lifetime bound, 'a and 'b.
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

type T<'a> = dyn Foo + 'a;
}
```

出處：https://doc.rust-lang.org/error_codes/E0226.html


---

## E0227 — This error indicates that the compiler is unable to determine whether there is
exactly one unique region in the set of derived region bounds.

This error indicates that the compiler is unable to determine whether there is exactly one unique region in the set of derived region bounds. Example of erroneous code: Here, baz can have either 'foo or 'bar lifetimes. To resolve this error, provide an explicit lifetime:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo<'foo>: 'foo {}
trait Bar<'bar>: 'bar {}

trait FooBar<'foo, 'bar>: Foo<'foo> + Bar<'bar> {}

struct Baz<'foo, 'bar> {
    baz: dyn FooBar<'foo, 'bar>,
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo<'foo>: 'foo {}
trait Bar<'bar>: 'bar {}

trait FooBar<'foo, 'bar>: Foo<'foo> + Bar<'bar> {}

struct Baz<'foo, 'bar, 'baz>
where
    'baz: 'foo + 'bar,
{
    obj: dyn FooBar<'foo, 'bar> + 'baz,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0227.html


---

## E0228 — The lifetime bound for this object type cannot be deduced from context and must
be specified.

The lifetime bound for this object type cannot be deduced from context and must be specified. When a trait object is used as a type argument of a generic type, Rust will try to infer its lifetime if unspecified. However, this isn’t possible when the containing type has more than one lifetime bound. The above example can be resolved by either reducing the number of lifetime bounds to one or by making the trait object lifetime explicit, like so: For more information, see RFC 599 and its amendment RFC 1156.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Trait { }

struct TwoBounds<'a, 'b, T: Sized + 'a + 'b> {
    x: &'a i32,
    y: &'b i32,
    z: T,
}

type Foo<'a, 'b> = TwoBounds<'a, 'b, dyn Trait>;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Trait { }

struct TwoBounds<'a, 'b, T: Sized + 'a + 'b> {
    x: &'a i32,
    y: &'b i32,
    z: T,
}

type Foo<'a, 'b> = TwoBounds<'a, 'b, dyn Trait + 'b>;
}
```

出處：https://doc.rust-lang.org/error_codes/E0228.html


---

## E0229 — An associated item constraint was written in an unexpected context.

An associated item constraint was written in an unexpected context. To solve this error, please move the associated item constraints to the type parameter declaration: Or into the where-clause:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
pub trait Foo {
    type A;
    fn boo(&self) -> <Self as Foo>::A;
}

struct Bar;

impl Foo for isize {
    type A = usize;
    fn boo(&self) -> usize { 42 }
}

fn baz<I>(x: &<I as Foo<A = Bar>>::A) {}
// error: associated item constraint are not allowed here
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Bar;
trait Foo { type A; }
fn baz<I: Foo<A=Bar>>(x: &<I as Foo>::A) {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0229.html


---

## E0230 — The #[rustc_on_unimplemented] attribute used to raise this error for various
misuses of the attribute; these are now warnings.

Note: this error code is no longer emitted by the compiler. The #[rustc_on_unimplemented] attribute used to raise this error for various misuses of the attribute; these are now warnings.

出處：https://doc.rust-lang.org/error_codes/E0230.html


---

## E0231 — Note: this error code is no longer emitted by the compiler

Note: this error code is no longer emitted by the compiler

出處：https://doc.rust-lang.org/error_codes/E0231.html


---

## E0232 — The #[rustc_on_unimplemented] attribute lets you specify a custom error
message for when a particular trait isn’t implemented on a type placed in a
position that needs that trait. The attribute will let you filter on
various types, with on:

The #[rustc_on_unimplemented] attribute lets you specify a custom error message for when a particular trait isn’t implemented on a type placed in a position that needs that trait. The attribute will let you filter on various types, with on: For this to work a cfg-like predicate must be supplied. A malformed filter will not do anything.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(rustc_attrs)]
#![allow(internal_features)]

fn main() {
#[rustc_on_unimplemented(on(blah, message = "foo"))] // error!
trait BadAnnotation {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0232.html


---

## E0243 — This error indicates that not enough type parameters were found in a type or
trait.

Note: this error code is no longer emitted by the compiler. This error indicates that not enough type parameters were found in a type or trait. For example, the Foo struct below is defined to be generic in T, but the type parameter is missing in the definition of Bar:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<T> { x: T }

struct Bar { x: Foo }
}
```

出處：https://doc.rust-lang.org/error_codes/E0243.html


---

## E0244 — This error indicates that too many type parameters were found in a type or
trait.

Note: this error code is no longer emitted by the compiler. This error indicates that too many type parameters were found in a type or trait. For example, the Foo struct below has no type parameters, but is supplied with two in the definition of Bar:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo { x: bool }

struct Bar<S, T> { x: Foo<S, T> }
}
```

出處：https://doc.rust-lang.org/error_codes/E0244.html


---

## E0251 — Two items of the same name cannot be imported without rebinding one of the
items under a new local name.

Note: this error code is no longer emitted by the compiler. Two items of the same name cannot be imported without rebinding one of the items under a new local name. An example of this error:

**正解**
```rust
use foo::baz;
use bar::*; // error, do `use foo::baz as quux` instead on the previous line

fn main() {}

mod foo {
    pub struct baz;
}

mod bar {
    pub mod baz {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0251.html


---

## E0252 — Two items of the same name cannot be imported without rebinding one of the
items under a new local name.

Two items of the same name cannot be imported without rebinding one of the items under a new local name. You can use aliases in order to fix this error. Example: Or you can reference the item with its parent:

**錯誤範例**
```rust
use foo::baz;
use bar::baz; // error, do `use bar::baz as quux` instead

fn main() {}

mod foo {
    pub struct baz;
}

mod bar {
    pub mod baz {}
}
```

**正解**
```rust
use foo::baz as foo_baz;
use bar::baz; // ok!

fn main() {}

mod foo {
    pub struct baz;
}

mod bar {
    pub mod baz {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0252.html


---

## E0253 — Attempt was made to import an unimportable type.

Note: this error code is no longer emitted by the compiler. Attempt was made to import an unimportable type. This can happen when trying to import a type from a trait. It’s invalid to directly import types belonging to a trait.

**正解**
```rust
#![feature(import_trait_associated_functions)]

mod foo {
    pub trait MyTrait {
        type SomeType;
    }
}

use foo::MyTrait::SomeType;
// error: `SomeType` is not directly importable

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0253.html


---

## E0254 — Attempt was made to import an item whereas an extern crate with this name has
already been imported.

Attempt was made to import an item whereas an extern crate with this name has already been imported. To fix this issue, you have to rename at least one of the two imports. Example:

**錯誤範例**
```rust
extern crate core;

mod foo {
    pub trait core {
        fn do_something();
    }
}

use foo::core;  // error: an extern crate named `core` has already
                //        been imported in this module

fn main() {}
```

**正解**
```rust
extern crate core as libcore; // ok!

mod foo {
    pub trait core {
        fn do_something();
    }
}

use foo::core;

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0254.html


---

## E0255 — You can’t import a value whose name is the same as another value defined in the
module.

You can’t import a value whose name is the same as another value defined in the module. You can use aliases in order to fix this error. Example: Or you can reference the item with its parent:

**錯誤範例**
```rust
use bar::foo; // error: an item named `foo` is already in scope

fn foo() {}

mod bar {
     pub fn foo() {}
}

fn main() {}
```

**正解**
```rust
use bar::foo as bar_foo; // ok!

fn foo() {}

mod bar {
     pub fn foo() {}
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0255.html


---

## E0256 — You can’t import a type or module when the name of the item being imported is
the same as another type or submodule defined in the module.

Note: this error code is no longer emitted by the compiler. You can’t import a type or module when the name of the item being imported is the same as another type or submodule defined in the module. An example of this error:

**錯誤範例**
```rust
use foo::Bar; // error

type Bar = u32;

mod foo {
    pub mod Bar { }
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0256.html


---

## E0259 — The name chosen for an external crate conflicts with another external crate
that has been imported into the current module.

The name chosen for an external crate conflicts with another external crate that has been imported into the current module. The solution is to choose a different name that doesn’t conflict with any external crate imported into the current module. Correct example:

**錯誤範例**
```rust
extern crate core;
extern crate std as core;

fn main() {}
```

**正解**
```rust
extern crate core;
extern crate std as other_name;

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0259.html


---

## E0260 — The name for an item declaration conflicts with an external crate’s name.

The name for an item declaration conflicts with an external crate’s name. There are two possible solutions: Solution #1: Rename the item. Solution #2: Import the crate with a different name. See the Declaration Statements section of the reference for more information about what constitutes an item declaration and what does not.

**錯誤範例**
```rust
extern crate core;

struct core;

fn main() {}
```

**正解**
```rust
#![allow(unused)]
fn main() {
extern crate core;

struct xyz;
}
```

出處：https://doc.rust-lang.org/error_codes/E0260.html


---

## E0261 — An undeclared lifetime was used.

An undeclared lifetime was used. These can be fixed by declaring lifetime parameters: Impl blocks declare lifetime parameters separately. You need to add lifetime parameters to an impl block if you’re implementing a type that has a lifetime parameter of its own. For example: This is fixed by declaring the impl block like this:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// error, use of undeclared lifetime name `'a`
fn foo(x: &'a str) { }

struct Foo {
    // error, use of undeclared lifetime name `'a`
    x: &'a str,
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a> {
    x: &'a str,
}

fn foo<'a>(x: &'a str) {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0261.html


---

## E0262 — An invalid name was used for a lifetime parameter.

An invalid name was used for a lifetime parameter. Declaring certain lifetime names in parameters is disallowed. For example, because the 'static lifetime is a special built-in lifetime name denoting the lifetime of the entire program, this is an error:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// error, invalid lifetime parameter name `'static`
fn foo<'static>(x: &'static str) { }
}
```

出處：https://doc.rust-lang.org/error_codes/E0262.html


---

## E0263 — A lifetime was declared more than once in the same scope.

Note: this error code is no longer emitted by the compiler. A lifetime was declared more than once in the same scope. Two lifetimes cannot have the same name. To fix this example, change the second 'a lifetime into something else ('c for example):

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo<'a, 'b, 'a>(x: &'a str, y: &'b str, z: &'a str) { // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo<'a, 'b, 'c>(x: &'a str, y: &'b str, z: &'c str) { // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0263.html


---

## E0264 — An unknown external lang item was used.

An unknown external lang item was used. A list of available external lang items is available in compiler/rustc_hir/src/weak_lang_items.rs. Example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(lang_items)]
#![allow(internal_features)]

fn main() {
extern "C" {
    #[lang = "copy"] // error: unknown external lang item: `copy`
    fn copy();
}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(lang_items)]
#![allow(internal_features)]

fn main() {
extern "C" {
    #[lang = "panic_impl"] // ok!
    fn cake();
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0264.html


---

## E0267 — A loop keyword (break or continue) was used inside a closure but outside of
any loop.

A loop keyword (break or continue) was used inside a closure but outside of any loop. break and continue keywords can be used as normal inside closures as long as they are also contained within a loop. To halt the execution of a closure you should instead use a return statement. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let w = || { break; }; // error: `break` inside of a closure
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let w = || {
    for _ in 0..10 {
        break;
    }
};

w();
}
```

出處：https://doc.rust-lang.org/error_codes/E0267.html


---

## E0268 — A loop keyword (break or continue) was used outside of a loop.

A loop keyword (break or continue) was used outside of a loop. Without a loop to break out of or continue in, no sensible action can be taken. Please verify that you are using break and continue only in loops. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn some_func() {
    break; // error: `break` outside of a loop
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn some_func() {
    for _ in 0..10 {
        break; // ok!
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0268.html


---

## E0271 — A type mismatched an associated type of a trait.

A type mismatched an associated type of a trait. The issue can be resolved by changing the associated type: in the foo implementation: in the Trait implementation for i8:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Trait { type AssociatedType; }

fn foo<T>(t: T) where T: Trait<AssociatedType=u32> {
//                    ~~~~~~~~ ~~~~~~~~~~~~~~~~~~
//                        |            |
//         This says `foo` can         |
//           only be used with         |
//              some type that         |
//         implements `Trait`.         |
//                                     |
//                             This says not only must
//                             `T` be an impl of `Trait`
//                             but also that the impl
//                             must assign the type `u32`
//                             to the associated type.
    println!("in foo");
}

impl Trait for i8 { type AssociatedType = &'static str; }
//~~~~~~~~~~~~~~~   ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
//      |                             |
// `i8` does have                     |
// implementation                     |
// of `Trait`...                      |
//                     ... but it is an implementation
//                     that assigns `&'static str` to
//                     the associated type.

foo(3_i8);
// Here, we invoke `foo` with an `i8`, which does not satisfy
// the constraint `<i8 as Trait>::AssociatedType=u32`, and
// therefore the type-checker complains with this error code.
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Trait { type AssociatedType; }

fn foo<T>(t: T) where T: Trait<AssociatedType = &'static str> {
    println!("in foo");
}

impl Trait for i8 { type AssociatedType = &'static str; }

foo(3_i8);
}
```

出處：https://doc.rust-lang.org/error_codes/E0271.html


---

## E0275 — An evaluation of a trait requirement overflowed.

An evaluation of a trait requirement overflowed. This error occurs when there was a recursive trait requirement that overflowed before it could be evaluated. This often means that there is an unbounded recursion in resolving some type bounds. To determine if a T is Foo, we need to check if Bar<T> is Foo. However, to do this check, we need to determine that Bar<Bar<T>> is Foo. To determine this, we check if Bar<Bar<Bar<T>>> is Foo, and so on. This is clearly a recursive requirement that can’t be resolved directly. Consider changing your trait bounds so that they’re less self-referential.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

struct Bar<T>(T);

impl<T> Foo for T where Bar<T>: Foo {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0275.html


---

## E0276 — A trait implementation has stricter requirements than the trait definition.

A trait implementation has stricter requirements than the trait definition. Here, all types implementing Foo must have a method foo<T>(x: T) which can take any type T. However, in the impl for bool, we have added an extra bound that T is Copy, which isn’t compatible with the original trait. Consider removing the bound from the method or adding the bound to the original method definition in the trait.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo<T>(x: T);
}

impl Foo for bool {
    fn foo<T>(x: T) where T: Copy {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0276.html


---

## E0277 — You tried to use a type which doesn’t implement some trait in a place which
expected that trait.

You tried to use a type which doesn’t implement some trait in a place which expected that trait. In order to fix this error, verify that the type you’re using does implement the trait. Example: Or in a generic context, an erroneous code example would look like: Note that the error here is in the definition of the generic function. Although we only call it with a parameter that does implement Debug, the compiler still rejects the function. It must work with all possible input types. In order to make this example compile, we need to restrict the generic type we’re accepting: Rust only looks at the signature of the called function, as such it must already specify all requirements that will be used for every type parameter.

**錯誤範例**
```rust
// here we declare the Foo trait with a bar method
trait Foo {
    fn bar(&self);
}

// we now declare a function which takes an object implementing the Foo trait
fn some_func<T: Foo>(foo: T) {
    foo.bar();
}

fn main() {
    // we now call the method with the i32 type, which doesn't implement
    // the Foo trait
    some_func(5i32); // error: the trait bound `i32 : Foo` is not satisfied
}
```

**正解**
```rust
trait Foo {
    fn bar(&self);
}

// we implement the trait on the i32 type
impl Foo for i32 {
    fn bar(&self) {}
}

fn some_func<T: Foo>(foo: T) {
    foo.bar(); // we can now use this method since i32 implements the
               // Foo trait
}

fn main() {
    some_func(5i32); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0277.html


---

## E0281 — You tried to supply a type which doesn’t implement some trait in a location
which expected that trait.

Note: this error code is no longer emitted by the compiler. You tried to supply a type which doesn’t implement some trait in a location which expected that trait. This error typically occurs when working with Fn-based types. The issue in this case is that foo is defined as accepting a Fn with one argument of type String, but the closure we attempted to pass to it requires one arguments of type usize.

**錯誤範例**
```rust
fn foo<F: Fn(usize)>(x: F) { }

fn main() {
    // type mismatch: ... implements the trait `core::ops::Fn<(String,)>`,
    // but the trait `core::ops::Fn<(usize,)>` is required
    // [E0281]
    foo(|y: String| { });
}
```

出處：https://doc.rust-lang.org/error_codes/E0281.html


---

## E0282 — The compiler could not infer a type and asked for a type annotation.

The compiler could not infer a type and asked for a type annotation. This error indicates that type inference did not result in one unique possible type, and extra information is required. In most cases this can be provided by adding a type annotation. Sometimes you need to specify a generic type parameter manually. In the example above, type Vec has a type parameter T. When calling Vec::new, barring any other later usage of the variable x that allows the compiler to infer what type T is, the compiler needs to be told what it is. The type can be specified on the variable: The type can also be specified in the path of the expression: In cases with more complex types, it is not necessary to annotate the full type. Once the ambiguity is resolved, the compiler can infer the rest: Another way to provide the compiler with enough information, is to specify the generic type parameter: Again, you need not specify the full type if the compiler can infer it: Apart from a method or function with a generic type parameter, this error can occur when a type parameter of a struct or trait cannot be inferred. In that case it is not always possible to use a type annotation, because all candidates have the same return type. For instance: This will fail because the compiler does not know which instance of Foo to call bar on. Change Foo::bar() to Foo::<T>::bar() to resolve the error.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = Vec::new();
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x: Vec<i32> = Vec::new();
}
```

出處：https://doc.rust-lang.org/error_codes/E0282.html


---

## E0283 — The compiler could not infer a type and asked for a type annotation.

The compiler could not infer a type and asked for a type annotation. This error indicates that type inference did not result in one unique possible type, and extra information is required. In most cases this can be provided by adding a type annotation. Sometimes you need to specify a generic type parameter manually. A common example is the collect method on Iterator. It has a generic type parameter with a FromIterator bound, which for a char iterator is implemented by Vec and String among others. Consider the following snippet that reverses the characters of a string: In the first code example, the compiler cannot infer what the type of x should be: Vec<char> and String are both suitable candidates. To specify which type to use, you can use a type annotation on x: It is not necessary to annotate the full type. Once the ambiguity is resolved, the compiler can infer the rest: Another way to provide the compiler with enough information, is to specify the generic type parameter: Again, you need not specify the full type if the compiler can infer it: We can see a self-contained example below: This error can be solved by adding type annotations that provide the missing information to the compiler. In this case, the solution is to specify the trait’s type parameter:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = "hello".chars().rev().collect();
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x: Vec<char> = "hello".chars().rev().collect();
}
```

出處：https://doc.rust-lang.org/error_codes/E0283.html


---

## E0284 — This error occurs when the compiler is unable to unambiguously infer the
return type of a function or method which is generic on return type, such
as the collect method for Iterators.

This error occurs when the compiler is unable to unambiguously infer the return type of a function or method which is generic on return type, such as the collect method for Iterators. For example: Here we have an addition of d and n.into(). Hence, n.into() can return any type T where u64: Add<T>. On the other hand, the into method can return any type where u32: Into<T>. The author of this code probably wants into() to return a u64, but the compiler can’t be sure that there isn’t another type T where both u32: Into<T> and u64: Add<T>. To resolve this error, use a concrete type for the intermediate expression:

**錯誤範例**
```rust
fn main() {
    let n: u32 = 1;
    let mut d: u64 = 2;
    d = d + n.into();
}
```

**正解**
```rust
fn main() {
    let n: u32 = 1;
    let mut d: u64 = 2;
    let m: u64 = n.into();
    d = d + m;
}
```

出處：https://doc.rust-lang.org/error_codes/E0284.html


---

## E0297 — Patterns used to bind names must be irrefutable.

Note: this error code is no longer emitted by the compiler. Patterns used to bind names must be irrefutable. That is, they must guarantee that a name will be extracted in all cases. Instead of pattern matching the loop variable, consider using a match or if let inside the loop body. For instance: Match inside the loop instead: Or use if let:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let xs : Vec<Option<i32>> = vec![Some(1), None];

// This fails because `None` is not covered.
for Some(x) in xs {
    // ...
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let xs : Vec<Option<i32>> = vec![Some(1), None];

for item in xs {
    match item {
        Some(x) => {},
        None => {},
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0297.html


---

## E0301 — Mutable borrows are not allowed in pattern guards, because matching cannot have
side effects.

Note: this error code is no longer emitted by the compiler. Mutable borrows are not allowed in pattern guards, because matching cannot have side effects. Side effects could alter the matched object or the environment on which the match depends in such a way, that the match would not be exhaustive. For instance, the following would not match any arm if mutable borrows were allowed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
match Some(()) {
    None => { },
    option if option.take().is_none() => {
        /* impossible, option is `Some` */
    },
    Some(_) => { } // When the previous match failed, the option became `None`.
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0301.html


---

## E0302 — Assignments are not allowed in pattern guards, because matching cannot have
side effects.

Note: this error code is no longer emitted by the compiler. Assignments are not allowed in pattern guards, because matching cannot have side effects. Side effects could alter the matched object or the environment on which the match depends in such a way, that the match would not be exhaustive. For instance, the following would not match any arm if assignments were allowed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
match Some(()) {
    None => { },
    option if { option = None; false } => { },
    Some(_) => { } // When the previous match failed, the option became `None`.
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0302.html


---

## E0303 — Sub-bindings, e.g.

Note: this error code is no longer emitted by the compiler. Sub-bindings, e.g. ref x @ Some(ref y) are now allowed under #![feature(bindings_after_at)] and checked to make sure that memory safety is upheld. In certain cases it is possible for sub-bindings to violate memory safety. Updates to the borrow checker in a future version of Rust may remove this restriction, but for now patterns must be rewritten without sub-bindings. Before: After: The op_string_ref binding has type &Option<&String> in both cases. See also Issue 14587.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
match Some("hi".to_string()) {
    ref op_string_ref @ Some(s) => {},
    None => {},
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
match Some("hi".to_string()) {
    Some(ref s) => {
        let op_string_ref = &Some(s);
        // ...
    },
    None => {},
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0303.html


---

## E0307 — The self parameter in a method has an invalid “receiver type”.

The self parameter in a method has an invalid “receiver type”. Methods take a special first parameter, of which there are three variants: self, &self, and &mut self. These are syntactic sugar for self: Self, self: &Self, and self: &mut Self respectively. The type Self acts as an alias to the type of the current trait implementer, or “receiver type”. Besides the already mentioned Self, &Self and &mut Self valid receiver types, the following are also valid: self: Box<Self>, self: Rc<Self>, self: Arc<Self>, and self: Pin<P> (where P is one of the previous types except Self). Note that Self can also be the underlying implementing type, like Foo in the following example: This error will be emitted by the compiler when using an invalid receiver type, like in the following example: The nightly feature Arbitrary self types extends the accepted set of receiver types to also include any type that implements the Receiver trait and can follow its chain of Target types to Self. There’s a blanket implementation of Receiver for T: Deref, so any type which dereferences to Self can be used.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;
struct Bar;

trait Trait {
    fn foo(&self);
}

impl Trait for Foo {
    fn foo(self: &Bar) {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;
trait Trait {
    fn foo(&self);
//         ^^^^^ `self` here is a reference to the receiver object
}

impl Trait for Foo {
    fn foo(&self) {}
//         ^^^^^ the receiver type is `&Foo`
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0307.html


---

## E0308 — Expected type did not match the received type.

Expected type did not match the received type. Erroneous code examples: This error occurs when an expression was used in a place where the compiler expected an expression of a different type. It can occur in several cases, the most common being when calling a function and passing an argument which has a different type than the matching type in the function declaration.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn plus_one(x: i32) -> i32 {
    x + 1
}

plus_one("Not a number");
//       ^^^^^^^^^^^^^^ expected `i32`, found `&str`

if "Not a bool" {
// ^^^^^^^^^^^^ expected `bool`, found `&str`
}

let x: f32 = "Not a float";
//     ---   ^^^^^^^^^^^^^ expected `f32`, found `&str`
//     |
//     expected due to this
}
```

出處：https://doc.rust-lang.org/error_codes/E0308.html


---

## E0309 — A parameter type is missing an explicit lifetime bound and may not live long
enough.

A parameter type is missing an explicit lifetime bound and may not live long enough. The type definition contains some field whose type requires an outlives annotation. Outlives annotations (e.g., T: 'a) are used to guarantee that all the data in T is valid for at least the lifetime 'a. This scenario most commonly arises when the type contains an associated type reference like <T as SomeTrait<'a>>::Output, as shown in the previous code. There, the where clause T: 'a that appears on the impl is not known to be satisfied on the struct. To make this example compile, you have to add a where-clause like T: 'a to the struct definition:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// This won't compile because the applicable impl of
// `SomeTrait` (below) requires that `T: 'a`, but the struct does
// not have a matching where-clause.
struct Foo<'a, T> {
    foo: <T as SomeTrait<'a>>::Output,
}

trait SomeTrait<'a> {
    type Output;
}

impl<'a, T> SomeTrait<'a> for T
where
    T: 'a,
{
    type Output = u32;
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a, T>
where
    T: 'a,
{
    foo: <T as SomeTrait<'a>>::Output
}

trait SomeTrait<'a> {
    type Output;
}

impl<'a, T> SomeTrait<'a> for T
where
    T: 'a,
{
    type Output = u32;
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0309.html


---

## E0310 — A parameter type is missing a lifetime constraint or has a lifetime that
does not live long enough.

A parameter type is missing a lifetime constraint or has a lifetime that does not live long enough. Type parameters in type definitions have lifetimes associated with them that represent how long the data stored within them is guaranteed to live. This lifetime must be as long as the data needs to be alive, and missing the constraint that denotes this will cause this error. This will compile, because it has the constraint on the type parameter:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// This won't compile because T is not constrained to the static lifetime
// the reference needs
struct Foo<T> {
    foo: &'static T
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<T: 'static> {
    foo: &'static T
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0310.html


---

## E0311 — This error occurs when there is an unsatisfied outlives bound involving an
elided region and a generic type parameter or associated type.

This error occurs when there is an unsatisfied outlives bound involving an elided region and a generic type parameter or associated type. Why doesn’t this code compile? It helps to look at the lifetime bounds that are automatically added by the compiler. For more details see the documentation for lifetime elision. The compiler elides the lifetime of x and the return type to some arbitrary lifetime 'anon in no_restriction(). The only information available to the compiler is that 'anon is valid for the duration of the function. When calling with_restriction(), the compiler requires the completely unrelated type parameter T to outlive 'anon because of the T: 'a bound in with_restriction(). This causes an error because T is not required to outlive 'anon in no_restriction(). If no_restriction() were to use &T instead of &() as an argument, the compiler would have added an implied bound, causing this to compile. This error can be resolved by explicitly naming the elided lifetime for x and then explicitly requiring that the generic parameter T outlives that lifetime:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn no_restriction<T>(x: &()) -> &() {
    with_restriction::<T>(x)
}

fn with_restriction<'a, T: 'a>(x: &'a ()) -> &'a () {
    x
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn no_restriction<'a, T: 'a>(x: &'a ()) -> &'a () {
    with_restriction::<T>(x)
}

fn with_restriction<'a, T: 'a>(x: &'a ()) -> &'a () {
    x
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0311.html


---

## E0312 — Reference’s lifetime of borrowed content doesn’t match the expected lifetime.

Note: this error code is no longer emitted by the compiler. Reference’s lifetime of borrowed content doesn’t match the expected lifetime. To fix this error, either lessen the expected lifetime or find a way to not have to use this reference outside of its current scope (by running the code directly in the same block for example?):

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
pub fn opt_str<'a>(maybestr: &'a Option<String>) -> &'static str {
    if maybestr.is_none() {
        "(none)"
    } else {
        let s: &'a str = maybestr.as_ref().unwrap();
        s  // Invalid lifetime!
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
// In this case, we can fix the issue by switching from "static" lifetime to 'a
pub fn opt_str<'a>(maybestr: &'a Option<String>) -> &'a str {
    if maybestr.is_none() {
        "(none)"
    } else {
        let s: &'a str = maybestr.as_ref().unwrap();
        s  // Ok!
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0312.html


---

## E0316 — A where clause contains a nested quantification over lifetimes.

A where clause contains a nested quantification over lifetimes. Rust syntax allows lifetime quantifications in two places within where clauses: Quantifying over the trait bound only (as in Ty: for<'l> Trait<'l>) and quantifying over the whole clause (as in for<'l> &'l Ty: Trait<'l>). Using both in the same clause leads to a nested lifetime quantification, which is not supported. The following example compiles, because the clause with the nested quantification has been rewritten to use only one for<>:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Tr<'a, 'b> {}

fn foo<T>(t: T)
where
    for<'a> &'a T: for<'b> Tr<'a, 'b>, // error: nested quantification
{
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Tr<'a, 'b> {}

fn foo<T>(t: T)
where
    for<'a, 'b> &'a T: Tr<'a, 'b>, // ok
{
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0316.html


---

## E0317 — An if expression is missing an else block.

An if expression is missing an else block. This error occurs when an if expression without an else block is used in a context where a type other than () is expected. In the previous code example, the let expression was expecting a value but since there was no else, no value was returned. An if expression without an else block has the type (), so this is a type error. To resolve it, add an else block having the same type as the if block. So to fix the previous code example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = 5;
let a = if x == 5 {
    1
};
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = 5;
let a = if x == 5 {
    1
} else {
    2
};
}
```

出處：https://doc.rust-lang.org/error_codes/E0317.html


---

## E0320 — Recursion limit reached while creating drop-check rules.

Recursion limit reached while creating drop-check rules. Example of erroneous code: The Rust compiler must be able to reason about how a type is Dropped, and by extension the types of its fields, to be able to generate the glue to properly drop a value. The code example above shows a type where this inference is impossible because it is recursive. Note that this is not the same as E0072, where a type has an infinite size; the type here has a finite size but any attempt to Drop it would recurse infinitely. For more information, read the Drop docs. It is not possible to define a type with recursive drop-check rules. All such recursion must be removed.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum A<T> {
    B,
    C(T, Box<A<(T, T)>>)
}

fn foo<T>() {
    A::<T>::B; // error: overflow while adding drop-check rules for A<T>
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0320.html


---

## E0321 — A cross-crate opt-out trait was implemented on something which wasn’t a struct
or enum type.

A cross-crate opt-out trait was implemented on something which wasn’t a struct or enum type. Only structs and enums are permitted to impl Send, Sync, and other opt-out trait, and the struct or enum must be local to the current crate. So, for example, unsafe impl Send for Rc<Foo> is not allowed.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(auto_traits)]

fn main() {
struct Foo;

impl !Sync for Foo {}

unsafe impl Send for &'static Foo {}
// error: cross-crate traits with a default impl, like `core::marker::Send`,
//        can only be implemented for a struct/enum type, not
//        `&'static Foo`
}
```

出處：https://doc.rust-lang.org/error_codes/E0321.html


---

## E0322 — A built-in trait was implemented explicitly. All implementations of the trait
are provided automatically by the compiler.

A built-in trait was implemented explicitly. All implementations of the trait are provided automatically by the compiler. The Sized trait is a special trait built-in to the compiler for types with a constant size known at compile-time. This trait is automatically implemented for types as needed by the compiler, and it is currently disallowed to explicitly implement it for a type.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Sized for Foo {} // error!
}
```

出處：https://doc.rust-lang.org/error_codes/E0322.html


---

## E0323 — An associated const was implemented when another trait item was expected.

An associated const was implemented when another trait item was expected. Please verify that the associated const wasn’t misspelled and the correct trait was implemented. Example: Or:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    type N;
}

struct Bar;

impl Foo for Bar {
    const N : u32 = 0;
    // error: item `N` is an associated const, which doesn't match its
    //        trait `<Bar as Foo>`
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Bar;

trait Foo {
    type N;
}

impl Foo for Bar {
    type N = u32; // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0323.html


---

## E0324 — A method was implemented when another trait item was expected.

A method was implemented when another trait item was expected. To fix this error, please verify that the method name wasn’t misspelled and verify that you are indeed implementing the correct trait items. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Bar;

trait Foo {
    const N : u32;

    fn M();
}

impl Foo for Bar {
    fn N() {}
    // error: item `N` is an associated method, which doesn't match its
    //        trait `<Bar as Foo>`
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Bar;

trait Foo {
    const N : u32;

    fn M();
}

impl Foo for Bar {
    const N : u32 = 0;

    fn M() {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0324.html


---

## E0325 — An associated type was implemented when another trait item was expected.

An associated type was implemented when another trait item was expected. Please verify that the associated type name wasn’t misspelled and your implementation corresponds to the trait definition. Example: Or:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Bar;

trait Foo {
    const N : u32;
}

impl Foo for Bar {
    type N = u32;
    // error: item `N` is an associated type, which doesn't match its
    //        trait `<Bar as Foo>`
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Bar;

trait Foo {
    type N;
}

impl Foo for Bar {
    type N = u32; // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0325.html


---

## E0326 — An implementation of a trait doesn’t match the type constraint.

An implementation of a trait doesn’t match the type constraint. The types of any associated constants in a trait implementation must match the types in the trait definition.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    const BAR: bool;
}

struct Bar;

impl Foo for Bar {
    const BAR: u32 = 5; // error, expected bool, found u32
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0326.html


---

## E0328 — The Unsize trait should not be implemented directly. All implementations of
Unsize are provided automatically by the compiler.

The Unsize trait should not be implemented directly. All implementations of Unsize are provided automatically by the compiler. If you are defining your own smart pointer type and would like to enable conversion from a sized to an unsized type with the DST coercion system, use CoerceUnsized instead.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(unsize)]

fn main() {
use std::marker::Unsize;

pub struct MyType;

impl<T> Unsize<T> for MyType {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(coerce_unsized)]

fn main() {
use std::ops::CoerceUnsized;

pub struct MyType<T: ?Sized> {
    field_with_unsized_type: T,
}

impl<T, U> CoerceUnsized<MyType<U>> for MyType<T>
    where T: CoerceUnsized<U> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0328.html


---

## E0329 — An attempt was made to access an associated constant through either a generic
type parameter or Self.

Note: this error code is no longer emitted by the compiler. An attempt was made to access an associated constant through either a generic type parameter or Self. This is not supported yet. An example causing this error is shown below: Currently, the value of BAR for a particular type can only be accessed through a concrete type, as shown below:

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    const BAR: f64;
}

struct MyStruct;

impl Foo for MyStruct {
    const BAR: f64 = 0f64;
}

fn get_bar_bad<F: Foo>(t: F) -> f64 {
    F::BAR
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0329.html


---

## E0364 — Private items cannot be publicly re-exported. This error indicates that you
attempted to pub use a type or value that was not itself public.

Private items cannot be publicly re-exported. This error indicates that you attempted to pub use a type or value that was not itself public. The solution to this problem is to ensure that the items that you are re-exporting are themselves marked with pub: See the Use Declarations section of the reference for more information on this topic.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod a {
    fn foo() {}

    mod a {
        pub use super::foo; // error!
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
mod a {
    pub fn foo() {} // ok!

    mod a {
        pub use super::foo;
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0364.html


---

## E0365 — Private modules cannot be publicly re-exported. This error indicates that you
attempted to pub use a module that was not itself public.

Private modules cannot be publicly re-exported. This error indicates that you attempted to pub use a module that was not itself public. The solution to this problem is to ensure that the module that you are re-exporting is itself marked with pub: See the Use Declarations section of the reference for more information on this topic.

**錯誤範例**
```rust
mod foo {
    pub const X: u32 = 1;
}

pub use foo as foo2;

fn main() {}
```

**正解**
```rust
pub mod foo {
    pub const X: u32 = 1;
}

pub use foo as foo2;

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0365.html


---

## E0366 — An attempt was made to implement Drop on a concrete specialization of a
generic type. An example is shown below:

An attempt was made to implement Drop on a concrete specialization of a generic type. An example is shown below: This code is not legal: it is not possible to specialize Drop to a subset of implementations of a generic type. One workaround for this is to wrap the generic type, as shown below:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<T> {
    t: T
}

impl Drop for Foo<u32> {
    fn drop(&mut self) {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<T> {
    t: T
}

struct Bar {
    t: Foo<u32>
}

impl Drop for Bar {
    fn drop(&mut self) {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0366.html


---

## E0367 — An attempt was made to implement Drop on a specialization of a generic type.

An attempt was made to implement Drop on a specialization of a generic type. This code is not legal: it is not possible to specialize Drop to a subset of implementations of a generic type. In order for this code to work, MyStruct must also require that T implements Foo. Alternatively, another option is to wrap the generic type in another that specializes appropriately:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

struct MyStruct<T> {
    t: T
}

impl<T: Foo> Drop for MyStruct<T> {
    fn drop(&mut self) {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo{}

struct MyStruct<T> {
    t: T
}

struct MyStructWrapper<T: Foo> {
    t: MyStruct<T>
}

impl <T: Foo> Drop for MyStructWrapper<T> {
    fn drop(&mut self) {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0367.html


---

## E0368 — A binary assignment operator like += or ^= was applied to a type that
doesn’t support it.

A binary assignment operator like += or ^= was applied to a type that doesn’t support it. To fix this error, please check that this type implements this binary operation. Example: It is also possible to overload most operators for your own type by implementing the [OP]Assign traits from std::ops. Another problem you might be facing is this: suppose you’ve overloaded the + operator for some type Foo by implementing the std::ops::Add trait for Foo, but you find that using += does not work, as in this example: This is because AddAssign is not automatically implemented, so you need to manually implement it for your type.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let mut x = 12f32; // error: binary operation `<<` cannot be applied to
                   //        type `f32`

x <<= 2;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut x = 12u32; // the `u32` type does implement the `ShlAssign` trait

x <<= 2; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0368.html


---

## E0369 — A binary operation was attempted on a type which doesn’t support it.

A binary operation was attempted on a type which doesn’t support it. To fix this error, please check that this type implements this binary operation. Example: It is also possible to overload most operators for your own type by implementing traits from std::ops. String concatenation appends the string on the right to the string on the left and may require reallocation. This requires ownership of the string on the left. If something should be added to a string literal, move the literal to the heap by allocating it with to_owned() like in "Your text".to_owned().

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = 12f32; // error: binary operation `<<` cannot be applied to
               //        type `f32`

x << 2;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = 12u32; // the `u32` type does implement it:
               // https://doc.rust-lang.org/stable/std/ops/trait.Shl.html

x << 2; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0369.html


---

## E0370 — The maximum value of an enum was reached, so it cannot be automatically
set in the next enum value.

The maximum value of an enum was reached, so it cannot be automatically set in the next enum value. To fix this, please set manually the next enum value or put the enum variant with the maximum value at the end of the enum. Examples: Or:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(i64)]
enum Foo {
    X = 0x7fffffffffffffff,
    Y, // error: enum discriminant overflowed on value after
       //        9223372036854775807: i64; set explicitly via
       //        Y = -9223372036854775808 if that is desired outcome
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(i64)]
enum Foo {
    X = 0x7fffffffffffffff,
    Y = 0, // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0370.html


---

## E0371 — A trait was implemented on another which already automatically implemented it.

A trait was implemented on another which already automatically implemented it. Erroneous code examples: This is okay, because Bar does not implement Baz by definition: When Trait2 is a subtrait of Trait1 (for example, when Trait2 has a definition like trait Trait2: Trait1 { ... }), it is not allowed to implement Trait1 for Trait2. This is because Trait2 already implements Trait1 by definition, so it is not useful to do this.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo { fn foo(&self) { } }
trait Bar: Foo { }
trait Baz: Bar { }

impl Bar for dyn Baz { } // error, `Baz` implements `Bar` by definition
impl Foo for dyn Baz { } // error, `Baz` implements `Bar` which implements `Foo`
impl Baz for dyn Baz { } // error, `Baz` (trivially) implements `Baz`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo { fn foo(&self) { } }
trait Bar: Foo { }
trait Baz: Bar { }
impl Baz for dyn Bar { } // Note: This is OK
}
```

出處：https://doc.rust-lang.org/error_codes/E0371.html


---

## E0373 — A captured variable in a closure may not live long enough.

A captured variable in a closure may not live long enough. This error occurs when an attempt is made to use data captured by a closure, when that data may no longer exist. It’s most commonly seen when attempting to return a closure as shown in the previous code example. Notice that x is stack-allocated by foo(). By default, Rust captures closed-over data by reference. This means that once foo() returns, x no longer exists. An attempt to access x within the closure would thus be unsafe. Another situation where this might be encountered is when spawning threads: Since our new thread runs in parallel, the stack frame containing x and y may well have disappeared by the time we try to use them. Even if we call thr.join() within foo (which blocks until thr has completed, ensuring the stack frame won’t disappear), we will not succeed: the compiler cannot prove that this behavior is safe, and so won’t let us do it. The solution to this problem is usually to switch to using a move closure. This approach moves (or copies, where possible) data into the closure, rather than taking references to it. For example: Now that the closure has its own copy of the data, there’s no need to worry about safety. This error may also be encountered while using async blocks: Similarly to closures, async blocks are not executed immediately and may capture closed-over data by reference. For more information, see https://rust-lang.github.io/async-book/03_async_await/01_chapter.html.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo() -> Box<dyn Fn(u32) -> u32> {
    let x = 0u32;
    Box::new(|y| x + y)
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo() -> Box<dyn Fn(u32) -> u32> {
    let x = 0u32;
    Box::new(move |y| x + y)
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0373.html


---

## E0374 — CoerceUnsized or DispatchFromDyn was implemented on a struct which does not
contain a field that is being unsized.

CoerceUnsized or DispatchFromDyn was implemented on a struct which does not contain a field that is being unsized. Example of erroneous code: CoerceUnsized is used to coerce structs that have a field that can be unsized, like a custom MyBox<T> being unsized to MyBox<dyn Trait>. DispatchFromDyn is used to dispatch from MyBox<dyn Trait> to MyBox<Self> in a dyn-compatible trait. If the struct doesn’t have any fields of unsized types then there is no meaningful way to implement CoerceUnsized or DispatchFromDyn, since there is no coercion taking place. Note that CoerceUnsized and DispatchFromDyn is mainly used by smart pointers like Box, Rc and Arc to be able to mark that they can coerce unsized types that they are pointing at.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(coerce_unsized)]
fn main() {
use std::ops::CoerceUnsized;

struct Foo<T: ?Sized> {
    a: i32,
}

// error: Struct `Foo` has no unsized fields that need to be coerced.
impl<T, U> CoerceUnsized<Foo<U>> for Foo<T>
    where T: CoerceUnsized<U> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0374.html


---

## E0375 — CoerceUnsized or DispatchFromDyn was implemented on a struct which contains
more than one field that is being unsized.

CoerceUnsized or DispatchFromDyn was implemented on a struct which contains more than one field that is being unsized. CoerceUnsized is used to coerce structs that have a field that can be unsized, like a custom MyBox<T> being unsized to MyBox<dyn Trait>. DispatchFromDyn is used to dispatch from MyBox<dyn Trait> to MyBox<Self> in a dyn-compatible trait. If the struct has multiple fields that must be unsized, then the compiler has no way to generate a valid implementation of CoerceUnsized or DispatchFromDyn. Note that CoerceUnsized and DispatchFromDyn is mainly used by smart pointers like Box, Rc and Arc to be able to mark that they can coerce unsized types that they are pointing at.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(coerce_unsized)]
fn main() {
use std::ops::CoerceUnsized;

struct Foo<T: ?Sized, U: ?Sized> {
    a: i32,
    b: T,
    c: U,
}

// error: Struct `Foo` has more than one unsized field.
impl<T, U> CoerceUnsized<Foo<U, T>> for Foo<T, U> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0375.html


---

## E0376 — CoerceUnsized or DispatchFromDyn was implemented between two types that
are not structs.

Note: this error code is no longer emitted by the compiler. CoerceUnsized or DispatchFromDyn was implemented between two types that are not structs. CoerceUnsized or DispatchFromDyn can only be implemented between structs.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(coerce_unsized)]
fn main() {
use std::ops::CoerceUnsized;

struct Foo<T: ?Sized> {
    a: T,
}

// error: The type `U` is not a struct
impl<T, U> CoerceUnsized<U> for Foo<T> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0376.html


---

## E0377 — CoerceUnsized or DispatchFromDyn may only be implemented between structs
of the same type.

CoerceUnsized or DispatchFromDyn may only be implemented between structs of the same type. Example of erroneous code: CoerceUnsized is used to coerce structs that have a field that can be unsized, like a custom MyBox<T> being unsized to MyBox<dyn Trait>. DispatchFromDyn is used to dispatch from MyBox<dyn Trait> to MyBox<Self> in a dyn-compatible trait. The compiler cannot support coercions between structs of different types, so a valid implementation of CoerceUnsized or DispatchFromDyn should be implemented between the same struct with different generic parameters. Note that CoerceUnsized and DispatchFromDyn is mainly used by smart pointers like Box, Rc and Arc to be able to mark that they can coerce unsized types that they are pointing at.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(coerce_unsized)]
fn main() {
use std::ops::CoerceUnsized;

pub struct Foo<T: ?Sized> {
    field_with_unsized_type: T,
}

pub struct Bar<T: ?Sized> {
    field_with_unsized_type: T,
}

// error: the trait `CoerceUnsized` may only be implemented for a coercion
//        between structures with the same definition
impl<T, U> CoerceUnsized<Bar<U>> for Foo<T> where T: CoerceUnsized<U> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0377.html


---

## E0378 — The DispatchFromDyn trait was implemented on something which is not a pointer
or a newtype wrapper around a pointer.

The DispatchFromDyn trait was implemented on something which is not a pointer or a newtype wrapper around a pointer. The DispatchFromDyn trait currently can only be implemented for builtin pointer types and structs that are newtype wrappers around them — that is, the struct must have only one field (except for PhantomData), and that field must itself implement DispatchFromDyn. Another example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(dispatch_from_dyn)]
fn main() {
use std::ops::DispatchFromDyn;

struct WrapperExtraField<T> {
    ptr: T,
    extra_stuff: i32,
}

impl<T, U> DispatchFromDyn<WrapperExtraField<U>> for WrapperExtraField<T>
where
    T: DispatchFromDyn<U>,
{}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(dispatch_from_dyn, unsize)]
fn main() {
use std::{
    marker::Unsize,
    ops::DispatchFromDyn,
};

struct Ptr<T: ?Sized>(*const T);

impl<T: ?Sized, U: ?Sized> DispatchFromDyn<Ptr<U>> for Ptr<T>
where
    T: Unsize<U>,
{}
}
```

出處：https://doc.rust-lang.org/error_codes/E0378.html


---

## E0379 — A trait method was declared const.

A trait method was declared const. Trait methods cannot be declared const by design. For more information, see RFC 911.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    const fn bar() -> u32; // error!
}

impl Foo for () {
    const fn bar() -> u32 { 0 } // error!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0379.html


---

## E0380 — An auto trait was declared with a method or an associated item.

An auto trait was declared with a method or an associated item. Auto traits cannot have methods or associated items. For more information see the opt-in builtin traits RFC.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
unsafe auto trait Trait {
    type Output; // error!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0380.html


---

## E0381 — It is not allowed to use or capture an uninitialized variable.

It is not allowed to use or capture an uninitialized variable. To fix this, ensure that any declared variables are initialized before being used. Example:

**錯誤範例**
```rust
fn main() {
    let x: i32;
    let y = x; // error, use of possibly-uninitialized variable
}
```

**正解**
```rust
fn main() {
    let x: i32 = 0;
    let y = x; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0381.html


---

## E0382 — A variable was used after its contents have been moved elsewhere.

A variable was used after its contents have been moved elsewhere. Since MyStruct is a type that is not marked Copy, the data gets moved out of x when we set y. This is fundamental to Rust’s ownership system: outside of workarounds like Rc, a value cannot be owned by more than one variable. Sometimes we don’t need to move the value. Using a reference, we can let another function borrow the value without changing its ownership. In the example below, we don’t actually have to move our string to calculate_length, we can give it a reference to it with & instead. A mutable reference can be created with &mut. Sometimes we don’t want a reference, but a duplicate. All types marked Clone can be duplicated by calling .clone(). Subsequent changes to a clone do not affect the original variable. Most types in the standard library are marked Clone. The example below demonstrates using clone() on a string. s1 is first set to “many”, and then copied to s2. Then the first character of s1 is removed, without affecting s2. “any many” is printed to the console. If we control the definition of a type, we can implement Clone on it ourselves with #[derive(Clone)]. Some types have no ownership semantics at all and are trivial to duplicate. An example is i32 and the other number types. We don’t have to call .clone() to clone them, because they are marked Copy in addition to Clone. Implicit cloning is more convenient in this case. We can mark our own types Copy if all their members also are marked Copy. In the example below, we implement a Point type. Because it only stores two integers, we opt-out of ownership semantics with Copy. Then we can let p2 = p1 without p1 being moved. Alternatively, if we don’t control the struct’s definition, or mutable shared ownership is truly required, we can use Rc and RefCell: With this approach, x and y share ownership of the data via the Rc (reference count type). RefCell essentially performs runtime borrow checking: ensuring that at most one writer or mult

**錯誤範例**
```rust
struct MyStruct { s: u32 }

fn main() {
    let mut x = MyStruct{ s: 5u32 };
    let y = x;
    x.s = 6;
    println!("{}", x.s);
}
```

**正解**
```rust
fn main() {
    let s1 = String::from("hello");

    let len = calculate_length(&s1);

    println!("The length of '{}' is {}.", s1, len);
}

fn calculate_length(s: &String) -> usize {
    s.len()
}
```

出處：https://doc.rust-lang.org/error_codes/E0382.html


---

## E0383 — This error occurs when an attempt is made to partially reinitialize a
structure that is currently uninitialized.

Note: this error code is no longer emitted by the compiler. This error occurs when an attempt is made to partially reinitialize a structure that is currently uninitialized. For example, this can happen when a drop has taken place: This error can be fixed by fully reinitializing the structure in question:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    a: u32,
}
impl Drop for Foo {
    fn drop(&mut self) { /* ... */ }
}

let mut x = Foo { a: 1 };
drop(x); // `x` is now uninitialized
x.a = 2; // error, partial reinitialization of uninitialized structure `t`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    a: u32,
}
impl Drop for Foo {
    fn drop(&mut self) { /* ... */ }
}

let mut x = Foo { a: 1 };
drop(x);
x = Foo { a: 2 };
}
```

出處：https://doc.rust-lang.org/error_codes/E0383.html


---

## E0384 — An immutable variable was reassigned.

An immutable variable was reassigned. By default, variables in Rust are immutable. To fix this error, add the keyword mut after the keyword let when declaring the variable. For example: Alternatively, you might consider initializing a new variable: either with a new bound name or (by shadowing) with the bound name of your existing variable. For example:

**錯誤範例**
```rust
fn main() {
    let x = 3;
    x = 5; // error, reassignment of immutable variable
}
```

**正解**
```rust
fn main() {
    let mut x = 3;
    x = 5;
}
```

出處：https://doc.rust-lang.org/error_codes/E0384.html


---

## E0386 — This error occurs when an attempt is made to mutate the target of a mutable
reference stored inside an immutable container.

Note: this error code is no longer emitted by the compiler. This error occurs when an attempt is made to mutate the target of a mutable reference stored inside an immutable container. For example, this can happen when storing a &mut inside an immutable Box: This error can be fixed by making the container mutable: It can also be fixed by using a type with interior mutability, such as Cell or RefCell:

**正解**
```rust
#![allow(unused)]
fn main() {
let mut x: i64 = 1;
let y: Box<_> = Box::new(&mut x);
**y = 2; // error, cannot assign to data in an immutable container
}
```

出處：https://doc.rust-lang.org/error_codes/E0386.html


---

## E0387 — This error occurs when an attempt is made to mutate or mutably reference data
that a closure has captured immutably.

Note: this error code is no longer emitted by the compiler. This error occurs when an attempt is made to mutate or mutably reference data that a closure has captured immutably. The problem here is that foo is defined as accepting a parameter of type Fn. Closures passed into foo will thus be inferred to be of type Fn, meaning that they capture their context immutably. If the definition of foo is under your control, the simplest solution is to capture the data mutably. This can be done by defining foo to take FnMut rather than Fn: Alternatively, we can consider using the Cell and RefCell types to achieve interior mutability through a shared reference. Our example’s mutable function could be redefined as below: You can read more in the API documentation for Cell.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// Accepts a function or a closure that captures its environment immutably.
// Closures passed to foo will not be able to mutate their closed-over state.
fn foo<F: Fn()>(f: F) { }

// Attempts to mutate closed-over data. Error message reads:
// `cannot assign to data in a captured outer variable...`
fn mutable() {
    let mut x = 0u32;
    foo(|| x = 2);
}

// Attempts to take a mutable reference to closed-over data. Error message
// reads: `cannot borrow data mutably in a captured outer variable...`
fn mut_addr() {
    let mut x = 0u32;
    foo(|| { let y = &mut x; });
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo<F: FnMut()>(f: F) { }
}
```

出處：https://doc.rust-lang.org/error_codes/E0387.html


---

## E0388 — （已移除的錯誤碼）

Note: this error code is no longer emitted by the compiler.

出處：https://doc.rust-lang.org/error_codes/E0388.html


---

## E0389 — An attempt was made to mutate data using a non-mutable reference.

Note: this error code is no longer emitted by the compiler. An attempt was made to mutate data using a non-mutable reference. This commonly occurs when attempting to assign to a non-mutable reference of a mutable reference (&(&mut T)). Here, &mut fancy is mutable, but &(&mut fancy) is not. Creating an immutable reference to a value borrows it immutably. There can be multiple references of type &(&mut T) that point to the same value, so they must be immutable to prevent multiple mutable references to the same value. To fix this, either remove the outer reference: Or make the outer reference mutable:

**錯誤範例**
```rust
struct FancyNum {
    num: u8,
}

fn main() {
    let mut fancy = FancyNum{ num: 5 };
    let fancy_ref = &(&mut fancy);
    fancy_ref.num = 6; // error: cannot assign to data in a `&` reference
    println!("{}", fancy_ref.num);
}
```

**正解**
```rust
struct FancyNum {
    num: u8,
}

fn main() {
    let mut fancy = FancyNum{ num: 5 };

    let fancy_ref = &mut fancy;
    // `fancy_ref` is now &mut FancyNum, rather than &(&mut FancyNum)

    fancy_ref.num = 6; // No error!

    println!("{}", fancy_ref.num);
}
```

出處：https://doc.rust-lang.org/error_codes/E0389.html


---

## E0390 — A method or constant was implemented on a primitive type.

A method or constant was implemented on a primitive type. This isn’t allowed, but using a trait to implement a method or constant is a good solution. Example: Instead of defining an inherent implementation on a reference, you could also move the reference inside the implementation: becomes

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    x: i32
}

impl *mut Foo {}
// error: cannot define inherent `impl` for primitive types
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    x: i32
}

trait Bar {
    fn bar();
}

impl Bar for *mut Foo {
    fn bar() {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0390.html


---

## E0391 — A type dependency cycle has been encountered.

A type dependency cycle has been encountered. The previous example contains a circular dependency between two traits: FirstTrait depends on SecondTrait which itself depends on FirstTrait. See https://rustc-dev-guide.rust-lang.org/overview.html#queries and https://rustc-dev-guide.rust-lang.org/query.html for more information.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait FirstTrait : SecondTrait {

}

trait SecondTrait : FirstTrait {

}
}
```

出處：https://doc.rust-lang.org/error_codes/E0391.html


---

## E0392 — A type or lifetime parameter has been declared but is not actually used.

A type or lifetime parameter has been declared but is not actually used. If the type parameter was included by mistake, this error can be fixed by simply removing the type parameter, as shown below: Alternatively, if the type parameter was intentionally inserted, it must be used. A simple fix is shown below: This error may also commonly be found when working with unsafe code. For example, when using raw pointers one may wish to specify the lifetime for which the pointed-at data is valid. An initial attempt (below) causes this error: We want to express the constraint that Foo should not outlive 'a, because the data pointed to by T is only valid for that lifetime. The problem is that there are no actual uses of 'a. It’s possible to work around this by adding a PhantomData type to the struct, using it to tell the compiler to act as if the struct contained a borrowed reference &'a T: PhantomData can also be used to express information about unused type parameters.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Foo<T> {
    Bar,
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Foo {
    Bar,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0392.html


---

## E0393 — A type parameter which references Self in its default value was not specified.

A type parameter which references Self in its default value was not specified. A trait object is defined over a single, fully-defined trait. With a regular default parameter, this parameter can just be instantiated in. However, if the default parameter is Self, the trait changes for each concrete type; i.e. i32 will be expected to implement A<i32>, bool will be expected to implement A<bool>, etc… These types will not share an implementation of a fully-defined trait; instead they share implementations of a trait with different parameters instantiated in for each implementation. This is irreconcilable with what we need to make a trait object work, and is thus disallowed. Making the trait concrete by explicitly specifying the value of the defaulted parameter will fix this issue. Fixed example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait A<T = Self> {}

fn together_we_will_rule_the_galaxy(son: &dyn A) {}
// error: the type parameter `T` must be explicitly specified
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait A<T = Self> {}

fn together_we_will_rule_the_galaxy(son: &dyn A<i32>) {} // Ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0393.html


---

## E0398 — In Rust 1.3, the default object lifetime bounds are expected to change, as
described in RFC 1156.

Note: this error code is no longer emitted by the compiler. In Rust 1.3, the default object lifetime bounds are expected to change, as described in RFC 1156. You are getting a warning because the compiler thinks it is possible that this change will cause a compilation error in your code. It is possible, though unlikely, that this is a false alarm. The heart of the change is that where &'a Box<SomeTrait> used to default to &'a Box<SomeTrait+'a>, it now defaults to &'a Box<SomeTrait+'static> (here, SomeTrait is the name of some trait type). Note that the only types which are affected are references to boxes, like &Box<SomeTrait> or &[Box<SomeTrait>]. More common types like &SomeTrait or Box<SomeTrait> are unaffected. To silence this warning, edit your code to use an explicit bound. Most of the time, this means that you will want to change the signature of a function that you are calling. For example, if the error is reported on a call like foo(x), and foo is defined as follows: You might change it to: This explicitly states that you expect the trait object SomeTrait to contain references (with a maximum lifetime of 'a).

**正解**
```rust
#![allow(unused)]
fn main() {
trait SomeTrait {}
fn foo(arg: &Box<SomeTrait>) { /* ... */ }
}
```

出處：https://doc.rust-lang.org/error_codes/E0398.html


---

## E0399 — Note: this error code is no longer emitted by the compiler
You implemented a trait, overriding one or more of its associated types but did
not reimplement its default methods.

Note: this error code is no longer emitted by the compiler You implemented a trait, overriding one or more of its associated types but did not reimplement its default methods. Example of erroneous code: To fix this, add an implementation for each default method from the trait:

**正解**
```rust
#![allow(unused)]
#![feature(associated_type_defaults)]

fn main() {
pub trait Foo {
    type Assoc = u8;
    fn bar(&self) {}
}

impl Foo for i32 {
    // error - the following trait items need to be reimplemented as
    //         `Assoc` was overridden: `bar`
    type Assoc = i32;
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0399.html


---

## E0401 — Inner items do not inherit the generic parameters from the items
they are embedded in.

Inner items do not inherit the generic parameters from the items they are embedded in. Nor will this: Or this: Items nested inside other items are basically just like top-level items, except that they can only be used from the item they are in. There are a couple of solutions for this. If the item is a function, you may use a closure: For a generic item, you can copy over the parameters: Be sure to copy over any bounds as well: This may require additional type hints in the function body. In case the item is a function inside an impl, defining a private helper function might be easier: For default impls in traits, the private helper solution won’t work, however closures or copying the parameters should still work.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo<T>(x: T) {
    fn bar(y: T) { // T is defined in the "outer" function
        // ..
    }
    bar(x);
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo<T>(x: T) {
    let bar = |y: T| { // explicit type annotation may not be necessary
        // ..
    };
    bar(x);
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0401.html


---

## E0403 — Some type parameters have the same name.

Some type parameters have the same name. Please verify that none of the type parameters are misspelled, and rename any clashing parameters. Example: Type parameters in an associated item also cannot shadow parameters from the containing item:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn f<T, T>(s: T, u: T) {} // error: the name `T` is already used for a generic
                          //        parameter in this item's generic parameters
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn f<T, Y>(s: T, u: Y) {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0403.html


---

## E0404 — A type that is not a trait was used in a trait position, such as a bound
or impl.

A type that is not a trait was used in a trait position, such as a bound or impl. Another erroneous code example: Please verify that the trait’s name was not misspelled or that the right identifier was used. Example: Alternatively, you could introduce a new trait with your desired restrictions as a super trait: Finally, if you are on nightly and want to use a trait alias instead of a type alias, you should use #![feature(trait_alias)]:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;
struct Bar;

impl Foo for Bar {} // error: `Foo` is not a trait
fn baz<T: Foo>(t: T) {} // error: `Foo` is not a trait
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    // some functions
}
struct Bar;

impl Foo for Bar { // ok!
    // functions implementation
}

fn baz<T: Foo>(t: T) {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0404.html


---

## E0405 — The code refers to a trait that is not in scope.

The code refers to a trait that is not in scope. Please verify that the name of the trait wasn’t misspelled and ensure that it was imported. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl SomeTrait for Foo {} // error: trait `SomeTrait` is not in scope
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[cfg(for_demonstration_only)]
// solution 1:
use some_file::SomeTrait;

// solution 2:
trait SomeTrait {
    // some functions
}

struct Foo;

impl SomeTrait for Foo { // ok!
    // implements functions
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0405.html


---

## E0407 — A definition of a method not in the implemented trait was given in a trait
implementation.

A definition of a method not in the implemented trait was given in a trait implementation. Please verify you didn’t misspell the method name and you used the correct trait. First example: Second example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn a();
}

struct Bar;

impl Foo for Bar {
    fn a() {}
    fn b() {} // error: method `b` is not a member of trait `Foo`
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn a();
    fn b();
}

struct Bar;

impl Foo for Bar {
    fn a() {}
    fn b() {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0407.html


---

## E0408 — An “or” pattern was used where the variable bindings are not consistently bound
across patterns.

An “or” pattern was used where the variable bindings are not consistently bound across patterns. Here, y is bound to the contents of the Some and can be used within the block corresponding to the match arm. However, in case x is None, we have not specified what y is, and the block will use a nonexistent variable. To fix this error, either split into multiple match arms: or, bind the variable to a field of the same type in all sub-patterns of the or pattern: In this example, if x matches the pattern (0, _), the second field is set to y. If it matches (_, 0), the first field is set to y; so in all cases y is set to some value.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
match x {
    Some(y) | None => { /* use y */ } // error: variable `y` from pattern #1 is
                                      //        not bound in pattern #2
    _ => ()
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = Some(1);
match x {
    Some(y) => { /* use y */ }
    None => { /* ... */ }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0408.html


---

## E0409 — An “or” pattern was used where the variable bindings are not consistently bound
across patterns.

An “or” pattern was used where the variable bindings are not consistently bound across patterns. Here, y is bound by-value in one case and by-reference in the other. To fix this error, just use the same mode in both cases. Generally using ref or ref mut where not already used will fix this: Alternatively, split the pattern:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = (0, 2);
match x {
    (0, ref y) | (y, 0) => { /* use y */} // error: variable `y` is bound with
                                          //        different mode in pattern #2
                                          //        than in pattern #1
    _ => ()
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = (0, 2);
match x {
    (0, ref y) | (ref y, 0) => { /* use y */}
    _ => ()
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0409.html


---

## E0411 — The Self keyword was used outside an impl, trait, or type definition.

The Self keyword was used outside an impl, trait, or type definition. The Self keyword represents the current type, which explains why it can only be used inside an impl, trait, or type definition. It gives access to the associated items of a type: However, be careful when two types have a common associated type: This problem can be solved by specifying from which trait we want to use the Bar type:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
<Self>::foo; // error: use of `Self` outside of an impl, trait, or type
             // definition
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    type Bar;
}

trait Baz : Foo {
    fn bar() -> Self::Bar; // like this
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0411.html


---

## E0412 — Erroneous code examples:

To fix this error, please verify you didn’t misspell the type name, you did
declare it or imported it into the scope.

Note: this error code is no longer emitted by the compiler. Erroneous code examples: To fix this error, please verify you didn’t misspell the type name, you did declare it or imported it into the scope. Examples: Another case that causes this error is when a type is imported into a parent module. To fix this, you can follow the suggestion and use File directly or use super::File; which will import the types from the parent namespace. An example that causes this error is below:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
impl Something {} // error: type name `Something` is not in scope

// or:

trait Foo {
    fn bar(N); // error: type name `N` is not in scope
}

// or:

fn foo(x: T) {} // type name `T` is not in scope
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Something;

impl Something {} // ok!

// or:

trait Foo {
    type N;

    fn bar(_: Self::N); // ok!
}

// or:

fn foo<T>(x: T) {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0412.html


---

## E0415 — More than one function parameter have the same name.

More than one function parameter have the same name. Please verify you didn’t misspell parameters’ name. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo(f: i32, f: i32) {} // error: identifier `f` is bound more than
                          //        once in this parameter list
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo(f: i32, g: i32) {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0415.html


---

## E0416 — An identifier is bound more than once in a pattern.

An identifier is bound more than once in a pattern. Please verify you didn’t misspell identifiers’ name. Example: Or maybe did you mean to unify? Consider using a guard:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
match (1, 2) {
    (x, x) => {} // error: identifier `x` is bound more than once in the
                 //        same pattern
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
match (1, 2) {
    (x, y) => {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0416.html


---

## E0422 — An identifier that is neither defined nor a struct was used.

An identifier that is neither defined nor a struct was used. In this case, Foo is undefined, so it inherently isn’t anything, and definitely not a struct. In this case, foo is defined, but is not a struct, so Rust can’t use it as one.

**錯誤範例**
```rust
fn main () {
    let x = Foo { x: 1, y: 2 };
}
```

出處：https://doc.rust-lang.org/error_codes/E0422.html


---

## E0423 — An identifier was used like a function name or a value was expected and the
identifier exists but it belongs to a different namespace.

An identifier was used like a function name or a value was expected and the identifier exists but it belongs to a different namespace. Please verify you didn’t misspell the name of what you actually wanted to use here. Example: It is common to forget the trailing ! on macro invocations, which would also yield this error: Another case where this error is emitted is when a value is expected, but something else is found: Enum types used as values Enums are types and cannot be used directly as values.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo { a: bool };

let f = Foo();
// error: expected function, tuple struct or tuple variant, found `Foo`
// `Foo` is a struct name, but this expression uses it like a function name
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn Foo() -> u32 { 0 }

let f = Foo(); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0423.html


---

## E0424 — The self keyword was used inside of an associated function without a “self
receiver” parameter.

The self keyword was used inside of an associated function without a “self receiver” parameter. The self keyword can only be used inside methods, which are associated functions (functions defined inside of a trait or impl block) that have a self receiver as its first parameter, like self, &self, &mut self or self: &mut Pin<Self> (this last one is an example of an “arbitrary self type”). Check if the associated function’s parameter list should have contained a self receiver for it to be a method, and add it if so. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Foo {
    // `bar` is a method, because it has a receiver parameter.
    fn bar(&self) {}

    // `foo` is not a method, because it has no receiver parameter.
    fn foo() {
        self.bar(); // error: `self` value is a keyword only available in
                    //        methods with a `self` parameter
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Foo {
    fn bar(&self) {}

    fn foo(self) { // `foo` is now a method.
        self.bar(); // ok!
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0424.html


---

## E0425 — An unresolved name was used.

An unresolved name was used. Erroneous code examples: Please verify that the name wasn’t misspelled and ensure that the identifier being referred to is valid for the given situation. Example: Or: Or: If the item is not defined in the current module, it must be imported using a use statement, like so: If the item you are importing is not defined in some super-module of the current module, then it must also be declared as public (e.g., pub fn).

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
something_that_doesnt_exist::foo;
// error: unresolved name `something_that_doesnt_exist::foo`

// or:

trait Foo {
    fn bar() {
        Self; // error: unresolved name `Self`
    }
}

// or:

let x = unknown_variable;  // error: unresolved name `unknown_variable`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum something_that_does_exist {
    Foo,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0425.html


---

## E0426 — An undeclared label was used.

An undeclared label was used. Please verify you spelled or declared the label correctly. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
loop {
    break 'a; // error: use of undeclared label `'a`
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
'a: loop {
    break 'a; // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0426.html


---

## E0428 — A type or module has been defined more than once.

A type or module has been defined more than once. Please verify you didn’t misspell the type/module’s name or remove/rename the duplicated one. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Bar;
struct Bar; // error: duplicate definition of value `Bar`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Bar;
struct Bar2; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0428.html


---

## E0429 — The self keyword cannot appear alone as the last segment in a use
declaration.

Note: this error code is no longer emitted by the compiler. The self keyword cannot appear alone as the last segment in a use declaration. To use a namespace itself in addition to some of its members, self may appear as part of a brace-enclosed list of imports: If you only want to import the namespace, do so directly:

**正解**
```rust
use std::fmt::self; // error: `self` imports are only allowed within a { } list
```

出處：https://doc.rust-lang.org/error_codes/E0429.html


---

## E0430 — The self import appears more than once in the list.

Note: this error code is no longer emitted by the compiler. The self import appears more than once in the list. Please verify you didn’t misspell the import name or remove the duplicated self import. Example:

**正解**
```rust
use something::{self, self}; // error: `self` import can only appear once in
                             //        the list
```

出處：https://doc.rust-lang.org/error_codes/E0430.html


---

## E0431 — An invalid self import was made.

Note: this error code is no longer emitted by the compiler. An invalid self import was made. You cannot import the current module into itself, please remove this import or verify you didn’t misspell it.

**正解**
```rust
use {self}; // error: `self` import can only appear in an import list with a
            //        non-empty prefix
```

出處：https://doc.rust-lang.org/error_codes/E0431.html


---

## E0432 — An import was unresolved.

An import was unresolved. In Rust 2015, paths in use statements are relative to the crate root. To import items relative to the current and parent modules, use the self:: and super:: prefixes, respectively. In Rust 2018 or later, paths in use statements are relative to the current module unless they begin with the name of a crate or a literal crate::, in which case they start from the crate root. As in Rust 2015 code, the self:: and super:: prefixes refer to the current and parent modules respectively. Also verify that you didn’t misspell the import name and that the import exists in the module from where you tried to import it. Example: If you tried to use a module from an external crate and are using Rust 2015, you may have missed the extern crate declaration (which is usually placed in the crate root): Since Rust 2018 the extern crate declaration is not required and you can instead just use it:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use something::Foo; // error: unresolved import `something::Foo`.
}
```

**正解**
```rust
use self::something::Foo; // Ok.

mod something {
    pub struct Foo;
}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0432.html


---

## E0433 — An undeclared crate, module, or type was used.

An undeclared crate, module, or type was used. Please verify you didn’t misspell the type/module’s name or that you didn’t forget to import it: If you’ve expected to use a crate name: Make sure the crate has been added as a dependency in Cargo.toml. To use a module from your current crate, add the crate:: prefix to the path.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let map = HashMap::new();
// error: failed to resolve: use of undeclared type `HashMap`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::collections::HashMap; // HashMap has been imported.
let map: HashMap<u32, u32> = HashMap::new(); // So it can be used!
}
```

出處：https://doc.rust-lang.org/error_codes/E0433.html


---

## E0434 — A variable used inside an inner function comes from a dynamic environment.

A variable used inside an inner function comes from a dynamic environment. Inner functions do not have access to their containing environment. To fix this error, you can replace the function with a closure: Or replace the captured variable with a constant or a static item:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo() {
    let y = 5;
    fn bar() -> u32 {
        y // error: can't capture dynamic environment in a fn item; use the
          //        || { ... } closure form instead.
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo() {
    let y = 5;
    let bar = || {
        y
    };
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0434.html


---

## E0435 — A non-constant value was used in a constant expression.

A non-constant value was used in a constant expression. ‘constant’ means ‘a compile-time value’. More details can be found in the Variables and Mutability section of the book. To fix this error, please replace the value with a constant. Example: Or:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let foo = 42;
let a: [u8; foo]; // error: attempt to use a non-constant value in a constant
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let a: [u8; 42]; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0435.html


---

## E0436 — The functional record update syntax was used on something other than a struct.

The functional record update syntax was used on something other than a struct. The functional record update syntax is only allowed for structs (struct-like enum variants don’t qualify, for example). To fix the previous code, rewrite the expression without functional record update syntax:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum PublicationFrequency {
    Weekly,
    SemiMonthly { days: (u8, u8), annual_special: bool },
}

fn one_up_competitor(competitor_frequency: PublicationFrequency)
                     -> PublicationFrequency {
    match competitor_frequency {
        PublicationFrequency::Weekly => PublicationFrequency::SemiMonthly {
            days: (1, 15), annual_special: false
        },
        c @ PublicationFrequency::SemiMonthly{ .. } =>
            PublicationFrequency::SemiMonthly {
                annual_special: true, ..c // error: functional record update
                                          //        syntax requires a struct
        }
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum PublicationFrequency {
    Weekly,
    SemiMonthly { days: (u8, u8), annual_special: bool },
}

fn one_up_competitor(competitor_frequency: PublicationFrequency)
                     -> PublicationFrequency {
    match competitor_frequency {
        PublicationFrequency::Weekly => PublicationFrequency::SemiMonthly {
            days: (1, 15), annual_special: false
        },
        PublicationFrequency::SemiMonthly{ days, .. } =>
            PublicationFrequency::SemiMonthly {
                days, annual_special: true // ok!
        }
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0436.html


---

## E0437 — An associated type whose name does not match any of the associated types
in the trait was used when implementing the trait.

An associated type whose name does not match any of the associated types in the trait was used when implementing the trait. Trait implementations can only implement associated types that are members of the trait in question. The solution to this problem is to remove the extraneous associated type:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

impl Foo for i32 {
    type Bar = bool;
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

impl Foo for i32 {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0437.html


---

## E0438 — An associated constant whose name does not match any of the associated constants
in the trait was used when implementing the trait.

An associated constant whose name does not match any of the associated constants in the trait was used when implementing the trait. Trait implementations can only implement associated constants that are members of the trait in question. The solution to this problem is to remove the extraneous associated constant:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

impl Foo for i32 {
    const BAR: bool = true;
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {}

impl Foo for i32 {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0438.html


---

## E0439 — The length of the platform-intrinsic function simd_shuffle wasn’t specified.

Note: this error code is no longer emitted by the compiler. The length of the platform-intrinsic function simd_shuffle wasn’t specified. The simd_shuffle function needs the length of the array passed as last parameter in its name. Example:

**正解**
```rust
#![feature(platform_intrinsics)]

extern "platform-intrinsic" {
    fn simd_shuffle<A,B>(a: A, b: A, c: [u32; 8]) -> B;
    // error: invalid `simd_shuffle`, needs length: `simd_shuffle`
}
```

出處：https://doc.rust-lang.org/error_codes/E0439.html


---

## E0445 — A private trait was used on a public type parameter bound.

Note: this error code is no longer emitted by the compiler. A private trait was used on a public type parameter bound. Previously erroneous code examples: To solve this error, please ensure that the trait is also public. The trait can be made inaccessible if necessary by placing it into a private inner module, but it still has to be marked with pub. Example:

**正解**
```rust
trait Foo {
    fn dummy(&self) { }
}

pub trait Bar : Foo {} // error: private trait in public interface
pub struct Bar2<T: Foo>(pub T); // same error
pub fn foo<T: Foo> (t: T) {} // same error

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0445.html


---

## E0446 — A private type or trait was used in a public associated type signature.

A private type or trait was used in a public associated type signature. There are two ways to solve this error. The first is to make the public type signature only public to a module that also has access to the private type. This is done by using pub(crate) or pub(in crate::my_mod::etc) Example: The other way to solve this error is to make the private type public. Example:

**錯誤範例**
```rust
struct Bar;

pub trait PubTr {
    type Alias;
}

impl PubTr for u8 {
    type Alias = Bar; // error private type in public interface
}

fn main() {}
```

**正解**
```rust
struct Bar;

pub(crate) trait PubTr { // only public to crate root
    type Alias;
}

impl PubTr for u8 {
    type Alias = Bar;
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0446.html


---

## E0447 — The pub keyword was used inside a function.

Note: this error code is no longer emitted by the compiler. The pub keyword was used inside a function. Since we cannot access items defined inside a function, the visibility of its items does not impact outer code. So using the pub keyword in this context is invalid.

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo() {
    pub struct Bar; // error: visibility has no effect inside functions
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0447.html


---

## E0448 — The pub keyword was used inside a public enum.

Note: this error code is no longer emitted by the compiler. The pub keyword was used inside a public enum. Since the enum is already public, adding pub on one its elements is unnecessary. Example: This is the correct syntax:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
pub enum Foo {
    pub Bar, // error: unnecessary `pub` visibility
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
pub enum Foo {
    Bar, // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0448.html


---

## E0449 — A visibility qualifier was used where one is not permitted. Visibility
qualifiers are not permitted on enum variants, trait items, impl blocks, and
extern blocks, as they already share the visibility of the parent item.

A visibility qualifier was used where one is not permitted. Visibility qualifiers are not permitted on enum variants, trait items, impl blocks, and extern blocks, as they already share the visibility of the parent item. Erroneous code examples: To fix this error, simply remove the visibility qualifier. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Bar;

trait Foo {
    fn foo();
}

enum Baz {
    pub Qux, // error: visibility qualifiers are not permitted here
}

pub impl Bar {} // error: visibility qualifiers are not permitted here

pub impl Foo for Bar { // error: visibility qualifiers are not permitted here
    pub fn foo() {} // error: visibility qualifiers are not permitted here
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Bar;

trait Foo {
    fn foo();
}

enum Baz {
    // Enum variants share the visibility of the enum they are in, so
    // `pub` is not allowed here
    Qux,
}

// Directly implemented methods share the visibility of the type itself,
// so `pub` is not allowed here
impl Bar {}

// Trait methods share the visibility of the trait, so `pub` is not
// allowed in either case
impl Foo for Bar {
    fn foo() {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0449.html


---

## E0451 — A struct constructor with private fields was invoked.

A struct constructor with private fields was invoked. To fix this error, please ensure that all the fields of the struct are public, or implement a function for easy instantiation. Examples: Or:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod bar {
    pub struct Foo {
        pub a: isize,
        b: isize,
    }
}

let f = bar::Foo{ a: 0, b: 0 }; // error: field `b` of struct `bar::Foo`
                                //        is private
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
mod bar {
    pub struct Foo {
        pub a: isize,
        pub b: isize, // we set `b` field public
    }
}

let f = bar::Foo{ a: 0, b: 0 }; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0451.html


---

## E0452 — An invalid lint attribute has been given.

An invalid lint attribute has been given. Lint attributes only accept a list of identifiers (where each identifier is a lint name). Ensure the attribute is of this form:

**錯誤範例**
```rust
#![allow(unused)]
#![allow(foo = "")] // error: malformed lint attribute
fn main() {
}
```

**正解**
```rust
#![allow(unused)]
#![allow(foo)] // ok!
fn main() {
// or:
#![allow(foo, foo2)] // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0452.html


---

## E0453 — A lint check attribute was overruled by a forbid directive set as an
attribute on an enclosing scope, or on the command line with the -F option.

A lint check attribute was overruled by a forbid directive set as an attribute on an enclosing scope, or on the command line with the -F option. Example of erroneous code: The forbid lint setting, like deny, turns the corresponding compiler warning into a hard error. Unlike deny, forbid prevents itself from being overridden by inner attributes. If you’re sure you want to override the lint check, you can change forbid to deny (or use -D instead of -F if the forbid setting was given as a command-line option) to allow the inner lint check attribute: Otherwise, edit the code to pass the lint check, and remove the overruled attribute:

**錯誤範例**
```rust
#![forbid(non_snake_case)]

#[allow(non_snake_case)]
fn main() {
    // error: allow(non_snake_case) incompatible with previous forbid
    let MyNumber = 2;
}
```

**正解**
```rust
#![deny(non_snake_case)]

#[allow(non_snake_case)]
fn main() {
    let MyNumber = 2; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0453.html


---

## E0454 — A link name was given with an empty name.

A link name was given with an empty name. The rust compiler cannot link to an external library if you don’t give it its name. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[link(name = "")] extern "C" {}
// error: `#[link(name = "")]` given with empty name
}
```

**正解**
```rust
#[link(name = "some_lib")] extern "C" {} // ok!
```

出處：https://doc.rust-lang.org/error_codes/E0454.html


---

## E0455 — Some linking kinds are target-specific and not supported on all platforms.

Some linking kinds are target-specific and not supported on all platforms. Linking with kind=framework is only supported when targeting macOS, as frameworks are specific to that operating system. Similarly, kind=raw-dylib is only supported when targeting Windows-like platforms. To solve this error you can use conditional compilation: Learn more in the Conditional Compilation section of the Reference.

**錯誤範例**
```rust
#[link(name = "FooCoreServices", kind = "framework")] extern "C" {}
// OS used to compile is Linux for example
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[cfg_attr(target="macos", link(name = "FooCoreServices", kind = "framework"))]
extern "C" {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0455.html


---

## E0457 — Note: this error code is no longer emitted by the compiler
Plugin ..

Note: this error code is no longer emitted by the compiler Plugin .. only found in rlib format, but must be available in dylib format. rlib-plugin.rs main.rs The compiler exposes a plugin interface to allow altering the compile process (adding lints, etc). Plugins must be defined in their own crates (similar to proc-macro isolation) and then compiled and linked to another crate. Plugin crates must be compiled to the dynamically-linked dylib format, and not the statically-linked rlib format. Learn more about different output types in this section of the Rust reference. This error is easily fixed by recompiling the plugin crate in the dylib format.

**正解**
```rust
#![crate_type = "rlib"]
#![feature(rustc_private)]

extern crate rustc_middle;
extern crate rustc_driver;

use rustc_driver::plugin::Registry;

#[no_mangle]
fn __rustc_plugin_registrar(_: &mut Registry) {}
```

出處：https://doc.rust-lang.org/error_codes/E0457.html


---

## E0458 — An unknown “kind” was specified for a link attribute.

Note: this error code is no longer emitted by the compiler. An unknown “kind” was specified for a link attribute. Please specify a valid “kind” value, from one of the following: static dylib framework raw-dylib

**正解**
```rust
#[link(kind = "wonderful_unicorn")] extern "C" {}
// error: unknown kind: `wonderful_unicorn`
```

出處：https://doc.rust-lang.org/error_codes/E0458.html


---

## E0459 — A link was used without a name parameter.

A link was used without a name parameter. Please add the name parameter to allow the rust compiler to find the library you want. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[link(kind = "dylib")] extern "C" {}
// error: `#[link(...)]` specified without `name = "foo"`
}
```

**正解**
```rust
#[link(kind = "dylib", name = "some_lib")] extern "C" {} // ok!
```

出處：https://doc.rust-lang.org/error_codes/E0459.html


---

## E0460 — Found possibly newer version of crate .. which .. depends on.

Found possibly newer version of crate .. which .. depends on. Consider these erroneous files: a1.rs a2.rs b.rs main.rs The dependency graph of this program can be represented as follows: Crate main depends on crate a (version 1) and crate b which in turn depends on crate a (version 2); this discrepancy in versions cannot be reconciled. This difference in versions typically occurs when one crate is compiled and linked, then updated and linked to another crate. The crate “version” is a SVH (Strict Version Hash) of the crate in an implementation-specific way. Note that this error can only occur when directly compiling and linking with rustc; Cargo automatically resolves dependencies, without using the compiler’s own dependency management that causes this issue. This error can be fixed by: Using Cargo, the Rust package manager, automatically fixing this issue. Recompiling crate a so that both crate b and main have a uniform version to depend on.

**正解**
```rust
#![crate_name = "a"]

pub fn foo<T>() {}
```

出處：https://doc.rust-lang.org/error_codes/E0460.html


---

## E0461 — Couldn’t find crate .. with expected target triple ...

Couldn’t find crate .. with expected target triple ... Example of erroneous code: a.rs main.rs a.rs is then compiled with --target powerpc-unknown-linux-gnu and b.rs with --target x86_64-unknown-linux-gnu. a.rs is compiled into a binary format incompatible with b.rs; PowerPC and x86 are totally different architectures. This issue also extends to any difference in target triples, as std is operating-system specific. This error can be fixed by: Using Cargo, the Rust package manager, automatically fixing this issue. Recompiling either crate so that they target a consistent target triple.

**正解**
```rust
#![crate_type = "lib"]

fn foo() {}
```

出處：https://doc.rust-lang.org/error_codes/E0461.html


---

## E0462 — Found staticlib .. instead of rlib or dylib.

Found staticlib .. instead of rlib or dylib. Consider the following two files: a.rs main.rs Crate a is compiled as a staticlib. A staticlib is a system-dependant library only intended for linking with non-Rust applications (C programs). Note that staticlibs include all upstream dependencies (core, std, other user dependencies, etc) which makes them significantly larger than dylibs: prefer staticlib for linking with C programs. Learn more about different crate_types in this section of the Reference. This error can be fixed by: Using Cargo, the Rust package manager, automatically fixing this issue. Recompiling the crate as a rlib or dylib; formats suitable for Rust linking.

**正解**
```rust
#![crate_type = "staticlib"]

fn foo() {}
```

出處：https://doc.rust-lang.org/error_codes/E0462.html


---

## E0463 — A crate was declared but cannot be found.

A crate was declared but cannot be found. You need to link your code to the relevant crate in order to be able to use it (through Cargo or the -L option of rustc, for example). Common causes The crate is not present at all. If using Cargo, add it to [dependencies] in Cargo.toml. The crate is present, but under a different name. If using Cargo, look for package = under [dependencies] in Cargo.toml. Common causes for missing std or core You are cross-compiling for a target which doesn’t have std prepackaged. Consider one of the following: Adding a pre-compiled version of std with rustup target add Building std from source with cargo build -Z build-std Using #![no_std] at the crate root, so you won’t need std in the first place. You are developing the compiler itself and haven’t built libstd from source. You can usually build it with x.py build library/std. More information about x.py is available in the rustc-dev-guide.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
extern crate foo; // error: can't find crate
}
```

出處：https://doc.rust-lang.org/error_codes/E0463.html


---

## E0464 — The compiler found multiple library files with the requested crate name.

The compiler found multiple library files with the requested crate name. This error can occur in several different cases – for example, when using extern crate or passing --extern options without crate paths. It can also be caused by caching issues with the build directory, in which case cargo clean may help. In the above example, there are three different library files, all of which define the same crate name. Without providing a full path, there is no way for the compiler to know which crate it should use.

**錯誤範例**
```rust
// aux-build:crateresolve-1.rs
// aux-build:crateresolve-2.rs
// aux-build:crateresolve-3.rs

extern crate crateresolve;
//~^ ERROR multiple candidates for `rlib` dependency `crateresolve` found

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0464.html


---

## E0466 — Macro import declaration was malformed.

Note: this error code is no longer emitted by the compiler. Macro import declaration was malformed. Erroneous code examples: This is a syntax error at the level of attribute declarations. The proper syntax for macro imports is the following: If you would like to import all exported macros, write macro_use with no arguments.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[macro_use(a_macro(another_macro))] // error: invalid import declaration
extern crate core as some_crate;

#[macro_use(i_want = "some_macros")] // error: invalid import declaration
extern crate core as another_crate;
}
```

**正解**
```rust
// In some_crate:
#[macro_export]
macro_rules! get_tacos {
    ...
}

#[macro_export]
macro_rules! get_pimientos {
    ...
}

// In your crate:
#[macro_use(get_tacos, get_pimientos)] // It imports `get_tacos` and
extern crate some_crate;               // `get_pimientos` macros from some_crate
```

出處：https://doc.rust-lang.org/error_codes/E0466.html


---

## E0468 — A non-root module tried to import macros from another crate.

A non-root module tried to import macros from another crate. Example of erroneous code: Only extern crate imports at the crate root level are allowed to import macros. Either move the macro import to crate root or do without the foreign macros. This will work:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod foo {
    #[macro_use(debug_assert)]  // error: must be at crate root to import
    extern crate core;          //        macros from another crate
    fn run_macro() { debug_assert!(true); }
}
}
```

**正解**
```rust
#[macro_use(debug_assert)] // ok!
extern crate core;

mod foo {
    fn run_macro() { debug_assert!(true); }
}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0468.html


---

## E0469 — A macro listed for import was not found.

A macro listed for import was not found. Either the listed macro is not contained in the imported crate, or it is not exported from the given crate. This could be caused by a typo. Did you misspell the macro’s name? Double-check the names of the macros listed for import, and that the crate in question exports them. A working version would be:

**錯誤範例**
```rust
#[macro_use(drink, be_merry)] // error: imported macro not found
extern crate alloc;

fn main() {
    // ...
}
```

**正解**
```rust
// In some_crate crate:
#[macro_export]
macro_rules! eat {
    ...
}

#[macro_export]
macro_rules! drink {
    ...
}

// In your crate:
#[macro_use(eat, drink)]
extern crate some_crate; //ok!
```

出處：https://doc.rust-lang.org/error_codes/E0469.html


---

## E0472 — Inline assembly (asm!) is not supported on this target.

Inline assembly (asm!) is not supported on this target. Example of erroneous code: The Rust compiler does not support inline assembly, with the asm! macro (previously llvm_asm!), for all targets. All Tier 1 targets do support this macro but support among Tier 2 and 3 targets is not guaranteed (even when they have std support). Note that this error is related to error[E0658]: inline assembly is not stable yet on this architecture, but distinct in that with E0472 support is not planned or in progress. There is no way to easily fix this issue, however: Consider if you really need inline assembly, is there some other way to achieve your goal (intrinsics, etc)? Consider writing your assembly externally, linking with it and calling it from Rust. Consider contributing to https://github.com/rust-lang/rust and help integrate support for your target!

**正解**
```rust
// compile-flags: --target sparc64-unknown-linux-gnu
#![no_std]

use core::arch::asm;

fn main() {
    unsafe {
        asm!(""); // error: inline assembly is not supported on this target
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0472.html


---

## E0476 — The coerced type does not outlive the value being coerced to.

The coerced type does not outlive the value being coerced to. Example of erroneous code: During a coercion, the “source pointer” (the coerced type) did not outlive the “object type” (value being coerced to). In the above example, 'b is not a subtype of 'a. This error can currently only be encountered with the unstable CoerceUnsized trait which allows custom coercions of unsized types behind a smart pointer to be implemented.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(coerce_unsized)]
#![feature(unsize)]

fn main() {
use std::marker::Unsize;
use std::ops::CoerceUnsized;

// error: lifetime of the source pointer does not outlive lifetime bound of the
//        object type
impl<'a, 'b, T, S> CoerceUnsized<&'a T> for &'b S where S: Unsize<T> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0476.html


---

## E0477 — The type does not fulfill the required lifetime.

Note: this error code is no longer emitted by the compiler. The type does not fulfill the required lifetime. In this example, the closure does not satisfy the 'static lifetime constraint. To fix this error, you need to double check the lifetime of the type. Here, we can fix this problem by giving s a static lifetime:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::sync::Mutex;

struct MyString<'a> {
    data: &'a str,
}

fn i_want_static_closure<F>(a: F)
    where F: Fn() + 'static {}

fn print_string<'a>(s: Mutex<MyString<'a>>) {

    i_want_static_closure(move || {     // error: this closure has lifetime 'a
                                        //        rather than 'static
        println!("{}", s.lock().unwrap().data);
    });
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::sync::Mutex;

struct MyString<'a> {
    data: &'a str,
}

fn i_want_static_closure<F>(a: F)
    where F: Fn() + 'static {}

fn print_string(s: Mutex<MyString<'static>>) {

    i_want_static_closure(move || {     // ok!
        println!("{}", s.lock().unwrap().data);
    });
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0477.html


---

## E0478 — A lifetime bound was not satisfied.

A lifetime bound was not satisfied. In this example, the 'SnowWhite lifetime is supposed to outlive the 'kiss lifetime but the declaration of the Prince struct doesn’t enforce it. To fix this issue, you need to specify it:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// Check that the explicit lifetime bound (`'SnowWhite`, in this example) must
// outlive all the superbounds from the trait (`'kiss`, in this example).

trait Wedding<'t>: 't { }

struct Prince<'kiss, 'SnowWhite> {
    child: Box<Wedding<'kiss> + 'SnowWhite>,
    // error: lifetime bound not satisfied
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Wedding<'t>: 't { }

struct Prince<'kiss, 'SnowWhite: 'kiss> { // You say here that 'SnowWhite
                                          // must live longer than 'kiss.
    child: Box<Wedding<'kiss> + 'SnowWhite>, // And now it's all good!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0478.html


---

## E0482 — A lifetime of a returned value does not outlive the function call.

Note: this error code is no longer emitted by the compiler. A lifetime of a returned value does not outlive the function call. To fix this error, make the lifetime of the returned value explicit: The impl Trait feature in this example uses an implicit 'static lifetime restriction in the returned type. However the type implementing the Iterator passed to the function lives just as long as 'a, which is not long enough. The solution involves adding lifetime bound to both function argument and the return value to make sure that the values inside the iterator are not dropped when the function goes out of the scope. An alternative solution would be to guarantee that the Item references in the iterator are alive for the whole lifetime of the program. A similar lifetime problem might arise when returning closures: Analogically, a solution here is to use explicit return lifetime and move the ownership of the variable to the closure. To better understand the lifetime treatment in the impl Trait, please see the RFC 1951.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn prefix<'a>(
    words: impl Iterator<Item = &'a str>
) -> impl Iterator<Item = String> { // error!
    words.map(|v| format!("foo-{}", v))
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn prefix<'a>(
    words: impl Iterator<Item = &'a str> + 'a
) -> impl Iterator<Item = String> + 'a { // ok!
    words.map(|v| format!("foo-{}", v))
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0482.html


---

## E0491 — A reference has a longer lifetime than the data it references.

A reference has a longer lifetime than the data it references. Here, the problem is that the compiler cannot be sure that the 'b lifetime will live longer than 'a, which should be mandatory in order to be sure that Trait::Out will always have a reference pointing to an existing type. So in this case, we just need to tell the compiler than 'b must outlive 'a:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a> {
    x: fn(&'a i32),
}

trait Trait<'a, 'b> {
    type Out;
}

impl<'a, 'b> Trait<'a, 'b> for usize {
    type Out = &'a Foo<'b>; // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a> {
    x: fn(&'a i32),
}

trait Trait<'a, 'b> {
    type Out;
}

impl<'a, 'b: 'a> Trait<'a, 'b> for usize { // we added the lifetime enforcement
    type Out = &'a Foo<'b>; // it now works!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0491.html


---

## E0492 — A borrow of a constant containing interior mutability was attempted.

A borrow of a constant containing interior mutability was attempted. A const represents a constant value that should never change. If one takes a & reference to the constant, then one is taking a pointer to some memory location containing the value. Normally this is perfectly fine: most values can’t be changed via a shared & pointer, but interior mutability would allow it. That is, a constant value could be mutated. On the other hand, a static is explicitly a single memory location, which can be mutated at will. So, in order to solve this error, use statics which are Sync: You can also have this error while using a cell type: This is because cell types do operations that are not thread-safe. Due to this, they don’t implement Sync and thus can’t be placed in statics. However, if you still wish to use these types, you can achieve this by an unsafe wrapper: Remember this solution is unsafe! You will have to ensure that accesses to the cell are synchronized.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::sync::atomic::AtomicUsize;

const A: AtomicUsize = AtomicUsize::new(0);
const B: &'static AtomicUsize = &A;
// error: cannot borrow a constant which may contain interior mutability,
//        create a static instead
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::sync::atomic::AtomicUsize;

static A: AtomicUsize = AtomicUsize::new(0);
static B: &'static AtomicUsize = &A; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0492.html


---

## E0493 — A value with a custom Drop implementation may be dropped during const-eval.

A value with a custom Drop implementation may be dropped during const-eval. The problem here is that if the given type or one of its fields implements the Drop trait, this Drop implementation cannot be called within a const context since it may run arbitrary, non-const-checked code. To prevent this issue, ensure all values with a custom Drop implementation escape the initializer.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum DropType {
    A,
}

impl Drop for DropType {
    fn drop(&mut self) {}
}

struct Foo {
    field1: DropType,
}

static FOO: Foo = Foo { field1: (DropType::A, DropType::A).1 }; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum DropType {
    A,
}

impl Drop for DropType {
    fn drop(&mut self) {}
}

struct Foo {
    field1: DropType,
}

static FOO: Foo = Foo { field1: DropType::A }; // We initialize all fields
                                               // by hand.
}
```

出處：https://doc.rust-lang.org/error_codes/E0493.html


---

## E0495 — A lifetime cannot be determined in the given situation.

Note: this error code is no longer emitted by the compiler. A lifetime cannot be determined in the given situation. In this code, you have two ways to solve this issue: Enforce that 'a lives at least as long as 'b. Use the same lifetime requirement for both input and output values. So for the first solution, you can do it by replacing 'a with 'a: 'b: In the second you can do it by simply removing 'b so they both use 'a:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn transmute_lifetime<'a, 'b, T>(t: &'a (T,)) -> &'b T {
    match (&t,) { // error!
        ((u,),) => u,
    }
}

let y = Box::new((42,));
let x = transmute_lifetime(&y);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn transmute_lifetime<'a: 'b, 'b, T>(t: &'a (T,)) -> &'b T {
    match (&t,) { // ok!
        ((u,),) => u,
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0495.html


---

## E0496 — A lifetime name is shadowing another lifetime name.

A lifetime name is shadowing another lifetime name. Please change the name of one of the lifetimes to remove this error. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a> {
    a: &'a i32,
}

impl<'a> Foo<'a> {
    fn f<'a>(x: &'a i32) { // error: lifetime name `'a` shadows a lifetime
                           //        name that is already in scope
    }
}
}
```

**正解**
```rust
struct Foo<'a> {
    a: &'a i32,
}

impl<'a> Foo<'a> {
    fn f<'b>(x: &'b i32) { // ok!
    }
}

fn main() {
}
```

出處：https://doc.rust-lang.org/error_codes/E0496.html


---

## E0497 — A stability attribute was used outside of the standard library.

Note: this error code is no longer emitted by the compiler. A stability attribute was used outside of the standard library. It is not possible to use stability attributes outside of the standard library. Also, for now, it is not possible to write deprecation messages either.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[stable] // error: stability attributes may not be used outside of the
          //        standard library
fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0497.html


---

## E0498 — The plugin attribute was malformed.

Note: this error code is no longer emitted by the compiler. The plugin attribute was malformed. The #[plugin] attribute should take a single argument: the name of the plugin. For example, for the plugin foo: See the plugin feature section of the Unstable book for more details.

**正解**
```rust
#![feature(plugin)]
#![plugin(foo(args))] // error: invalid argument
#![plugin(bar="test")] // error: invalid argument
```

出處：https://doc.rust-lang.org/error_codes/E0498.html


---

## E0499 — A variable was borrowed as mutable more than once.

A variable was borrowed as mutable more than once. Please note that in Rust, you can either have many immutable references, or one mutable reference. For more details you may want to read the References & Borrowing section of the Book. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let mut i = 0;
let mut x = &mut i;
let mut a = &mut i;
x;
// error: cannot borrow `i` as mutable more than once at a time
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut i = 0;
let mut x = &mut i; // ok!

// or:
let mut i = 0;
let a = &i; // ok!
let b = &i; // still ok!
let c = &i; // still ok!
b;
a;
}
```

出處：https://doc.rust-lang.org/error_codes/E0499.html


---

## E0500 — A borrowed variable was used by a closure.

A borrowed variable was used by a closure. In here, jon_snow is already borrowed by the nights_watch reference, so it cannot be borrowed by the starks closure at the same time. To fix this issue, you can create the closure after the borrow has ended: Or, if the type implements the Clone trait, you can clone it between closures:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn you_know_nothing(jon_snow: &mut i32) {
    let nights_watch = &jon_snow;
    let starks = || {
        *jon_snow = 3; // error: closure requires unique access to `jon_snow`
                       //        but it is already borrowed
    };
    println!("{}", nights_watch);
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn you_know_nothing(jon_snow: &mut i32) {
    let nights_watch = &jon_snow;
    println!("{}", nights_watch);
    let starks = || {
        *jon_snow = 3;
    };
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0500.html


---

## E0501 — A mutable variable is used but it is already captured by a closure.

A mutable variable is used but it is already captured by a closure. This error indicates that a mutable variable is used while it is still captured by a closure. Because the closure has borrowed the variable, it is not available until the closure goes out of scope. Note that a capture will either move or borrow a variable, but in this situation, the closure is borrowing the variable. Take a look at the chapter on Capturing in Rust By Example for more information. To fix this error, you can finish using the closure before using the captured variable: Or you can pass the variable as a parameter to the closure: It may be possible to define the closure later:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn inside_closure(x: &mut i32) {
    // Actions which require unique access
}

fn outside_closure(x: &mut i32) {
    // Actions which require unique access
}

fn foo(a: &mut i32) {
    let mut bar = || {
        inside_closure(a)
    };
    outside_closure(a); // error: cannot borrow `*a` as mutable because previous
                        //        closure requires unique access.
    bar();
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn inside_closure(x: &mut i32) {}
fn outside_closure(x: &mut i32) {}

fn foo(a: &mut i32) {
    let mut bar = || {
        inside_closure(a)
    };
    bar();
    // borrow on `a` ends.
    outside_closure(a); // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0501.html


---

## E0502 — A variable already borrowed with a certain mutability (either mutable or
immutable) was borrowed again with a different mutability.

A variable already borrowed with a certain mutability (either mutable or immutable) was borrowed again with a different mutability. To fix this error, ensure that you don’t have any other references to the variable before trying to access it with a different mutability: For more information on Rust’s ownership system, take a look at the References & Borrowing section of the Book.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn bar(x: &mut i32) {}
fn foo(a: &mut i32) {
    let y = &a; // a is borrowed as immutable.
    bar(a); // error: cannot borrow `*a` as mutable because `a` is also borrowed
            //        as immutable
    println!("{}", y);
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn bar(x: &mut i32) {}
fn foo(a: &mut i32) {
    bar(a);
    let y = &a; // ok!
    println!("{}", y);
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0502.html


---

## E0503 — A value was used after it was mutably borrowed.

A value was used after it was mutably borrowed. In this example, value is mutably borrowed by borrow and cannot be used to calculate sum. This is not possible because this would violate Rust’s mutability rules. You can fix this error by finishing using the borrow before the next use of the value: Or by cloning value before borrowing it: For more information on Rust’s ownership system, take a look at the References & Borrowing section of the Book.

**錯誤範例**
```rust
fn main() {
    let mut value = 3;
    // Create a mutable borrow of `value`.
    let borrow = &mut value;
    let _sum = value + 1; // error: cannot use `value` because
                          //        it was mutably borrowed
    println!("{}", borrow);
}
```

**正解**
```rust
fn main() {
    let mut value = 3;
    let borrow = &mut value;
    println!("{}", borrow);
    // The block has ended and with it the borrow.
    // You can now use `value` again.
    let _sum = value + 1;
}
```

出處：https://doc.rust-lang.org/error_codes/E0503.html


---

## E0504 — This error occurs when an attempt is made to move a borrowed variable into a
closure.

Note: this error code is no longer emitted by the compiler. This error occurs when an attempt is made to move a borrowed variable into a closure. Here, fancy_num is borrowed by fancy_ref and so cannot be moved into the closure x. There is no way to move a value into a closure while it is borrowed, as that would invalidate the borrow. If the closure can’t outlive the value being moved, try using a reference rather than moving: If the value has to be borrowed and then moved, try limiting the lifetime of the borrow using a scoped block: If the lifetime of a reference isn’t enough, such as in the case of threading, consider using an Arc to create a reference-counted value:

**錯誤範例**
```rust
struct FancyNum {
    num: u8,
}

fn main() {
    let fancy_num = FancyNum { num: 5 };
    let fancy_ref = &fancy_num;

    let x = move || {
        println!("child function: {}", fancy_num.num);
        // error: cannot move `fancy_num` into closure because it is borrowed
    };

    x();
    println!("main function: {}", fancy_ref.num);
}
```

**正解**
```rust
struct FancyNum {
    num: u8,
}

fn main() {
    let fancy_num = FancyNum { num: 5 };
    let fancy_ref = &fancy_num;

    let x = move || {
        // fancy_ref is usable here because it doesn't move `fancy_num`
        println!("child function: {}", fancy_ref.num);
    };

    x();

    println!("main function: {}", fancy_num.num);
}
```

出處：https://doc.rust-lang.org/error_codes/E0504.html


---

## E0505 — A value was moved out while it was still borrowed.

A value was moved out while it was still borrowed. Here, the function eat takes ownership of x. However, x cannot be moved because the borrow to _ref_to_val needs to last till the function borrow. To fix that you can do a few different things: Try to avoid moving the variable. Release borrow before move. Implement the Copy trait on the type. Examples: Or: Or: For more information on Rust’s ownership system, take a look at the References & Borrowing section of the Book.

**錯誤範例**
```rust
struct Value {}

fn borrow(val: &Value) {}

fn eat(val: Value) {}

fn main() {
    let x = Value{};
    let _ref_to_val: &Value = &x;
    eat(x);
    borrow(_ref_to_val);
}
```

**正解**
```rust
struct Value {}

fn borrow(val: &Value) {}

fn eat(val: &Value) {}

fn main() {
    let x = Value{};

    let ref_to_val: &Value = &x;
    eat(&x); // pass by reference, if it's possible
    borrow(ref_to_val);
}
```

出處：https://doc.rust-lang.org/error_codes/E0505.html


---

## E0506 — An attempt was made to assign to a borrowed value.

An attempt was made to assign to a borrowed value. Because fancy_ref still holds a reference to fancy_num, fancy_num can’t be assigned to a new value as it would invalidate the reference. Alternatively, we can move out of fancy_num into a second fancy_num: If the value has to be borrowed, try limiting the lifetime of the borrow using a scoped block: Or by moving the reference into a function:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct FancyNum {
    num: u8,
}

let mut fancy_num = FancyNum { num: 5 };
let fancy_ref = &fancy_num;
fancy_num = FancyNum { num: 6 };
// error: cannot assign to `fancy_num` because it is borrowed

println!("Num: {}, Ref: {}", fancy_num.num, fancy_ref.num);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct FancyNum {
    num: u8,
}

let mut fancy_num = FancyNum { num: 5 };
let moved_num = fancy_num;
fancy_num = FancyNum { num: 6 };

println!("Num: {}, Moved num: {}", fancy_num.num, moved_num.num);
}
```

出處：https://doc.rust-lang.org/error_codes/E0506.html


---

## E0507 — A borrowed value was moved out.

A borrowed value was moved out. Here, the nothing_is_true method takes the ownership of self. However, self cannot be moved because .borrow() only provides an &TheDarkKnight, which is a borrow of the content owned by the RefCell. To fix this error, you have three choices: Try to avoid moving the variable. Somehow reclaim the ownership. Implement the Copy trait on the type. This can also happen when using a type implementing Fn or FnMut, as neither allows moving out of them (they usually represent closures which can be called more than once). Much of the text following applies equally well to non-FnOnce closure bodies. Examples: Or: Or: Moving a member out of a mutably borrowed struct will also cause E0507 error: It is fine only if you put something back. mem::replace can be used for that: For more information on Rust’s ownership system, take a look at the References & Borrowing section of the Book.

**錯誤範例**
```rust
use std::cell::RefCell;

struct TheDarkKnight;

impl TheDarkKnight {
    fn nothing_is_true(self) {}
}

fn main() {
    let x = RefCell::new(TheDarkKnight);

    x.borrow().nothing_is_true(); // error: cannot move out of borrowed content
}
```

**正解**
```rust
use std::cell::RefCell;

struct TheDarkKnight;

impl TheDarkKnight {
    fn nothing_is_true(&self) {} // First case, we don't take ownership
}

fn main() {
    let x = RefCell::new(TheDarkKnight);

    x.borrow().nothing_is_true(); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0507.html


---

## E0508 — A value was moved out of a non-copy fixed-size array.

A value was moved out of a non-copy fixed-size array. The first element was moved out of the array, but this is not possible because NonCopy does not implement the Copy trait. Consider borrowing the element instead of moving it: Alternatively, if your type implements Clone and you need to own the value, consider borrowing and then cloning: If you really want to move the value out, you can use a destructuring array pattern to move it:

**錯誤範例**
```rust
struct NonCopy;

fn main() {
    let array = [NonCopy; 1];
    let _value = array[0]; // error: cannot move out of type `[NonCopy; 1]`,
                           //        a non-copy fixed-size array
}
```

**正解**
```rust
struct NonCopy;

fn main() {
    let array = [NonCopy; 1];
    let _value = &array[0]; // Borrowing is allowed, unlike moving.
}
```

出處：https://doc.rust-lang.org/error_codes/E0508.html


---

## E0509 — This error occurs when an attempt is made to move out of a value whose type
implements the Drop trait.

This error occurs when an attempt is made to move out of a value whose type implements the Drop trait. Here, we tried to move a field out of a struct of type DropStruct which implements the Drop trait. However, a struct cannot be dropped if one or more of its fields have been moved. Structs implementing the Drop trait have an implicit destructor that gets called when they go out of scope. This destructor may use the fields of the struct, so moving out of the struct could make it impossible to run the destructor. Therefore, we must think of all values whose type implements the Drop trait as single units whose fields cannot be moved. This error can be fixed by creating a reference to the fields of a struct, enum, or tuple using the ref keyword: Note that this technique can also be used in the arms of a match expression:

**錯誤範例**
```rust
struct FancyNum {
    num: usize
}

struct DropStruct {
    fancy: FancyNum
}

impl Drop for DropStruct {
    fn drop(&mut self) {
        // Destruct DropStruct, possibly using FancyNum
    }
}

fn main() {
    let drop_struct = DropStruct{fancy: FancyNum{num: 5}};
    let fancy_field = drop_struct.fancy; // Error E0509
    println!("Fancy: {}", fancy_field.num);
    // implicit call to `drop_struct.drop()` as drop_struct goes out of scope
}
```

**正解**
```rust
struct FancyNum {
    num: usize
}

struct DropStruct {
    fancy: FancyNum
}

impl Drop for DropStruct {
    fn drop(&mut self) {
        // Destruct DropStruct, possibly using FancyNum
    }
}

fn main() {
    let drop_struct = DropStruct{fancy: FancyNum{num: 5}};
    let ref fancy_field = drop_struct.fancy; // No more errors!
    println!("Fancy: {}", fancy_field.num);
    // implicit call to `drop_struct.drop()` as drop_struct goes out of scope
}
```

出處：https://doc.rust-lang.org/error_codes/E0509.html


---

## E0510 — The matched value was assigned in a match guard.

The matched value was assigned in a match guard. When matching on a variable it cannot be mutated in the match guards, as this could cause the match to be non-exhaustive. Here executing x = None would modify the value being matched and require us to go “back in time” to the None arm. To fix it, change the value in the match arm:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let mut x = Some(0);
match x {
    None => {}
    Some(_) if { x = None; false } => {} // error!
    Some(_) => {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut x = Some(0);
match x {
    None => {}
    Some(_) => {
        x = None; // ok!
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0510.html


---

## E0511 — Invalid monomorphization of an intrinsic function was used.

Invalid monomorphization of an intrinsic function was used. The generic type has to be a SIMD type. Example:

**錯誤範例**
```rust
#![feature(intrinsics)]

#[rustc_intrinsic]
unsafe fn simd_add<T>(a: T, b: T) -> T;

fn main() {
    unsafe { simd_add(0, 1); }
    // error: invalid monomorphization of `simd_add` intrinsic
}
```

**正解**
```rust
#![allow(unused)]
#![feature(repr_simd)]
#![feature(intrinsics)]

fn main() {
#[repr(simd)]
#[derive(Copy, Clone)]
struct i32x2([i32; 2]);

#[rustc_intrinsic]
unsafe fn simd_add<T>(a: T, b: T) -> T;

unsafe { simd_add(i32x2([0, 0]), i32x2([1, 2])); } // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0511.html


---

## E0512 — Transmute with two differently sized types was attempted.

Transmute with two differently sized types was attempted. Please use types with same size or use the expected type directly. Example:

**錯誤範例**
```rust
fn takes_u8(_: u8) {}

fn main() {
    unsafe { takes_u8(::std::mem::transmute(0u16)); }
    // error: cannot transmute between types of different sizes,
    //        or dependently-sized types
}
```

**正解**
```rust
fn takes_u8(_: u8) {}

fn main() {
    unsafe { takes_u8(::std::mem::transmute(0i8)); } // ok!
    // or:
    unsafe { takes_u8(0u8); } // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0512.html


---

## E0514 — Dependency compiled with different version of rustc.

Dependency compiled with different version of rustc. Example of erroneous code: a.rs b.rs This error is caused when the version of rustc used to compile a crate, as stored in the binary’s metadata, differs from the version of one of its dependencies. Many parts of Rust binaries are considered unstable. For instance, the Rust ABI is not stable between compiler versions. This means that the compiler cannot be sure about how to call a function between compiler versions, and therefore this error occurs. This error can be fixed by: Using Cargo, the Rust package manager and Rustup, the Rust toolchain installer, automatically fixing this issue. Recompiling the crates with a uniform rustc version.

**正解**
```rust
// compiled with stable `rustc`

#[crate_type = "lib"]
```

出處：https://doc.rust-lang.org/error_codes/E0514.html


---

## E0515 — A reference to a local variable was returned.

A reference to a local variable was returned. Local variables, function parameters and temporaries are all dropped before the end of the function body. A returned reference (or struct containing a reference) to such a dropped value would immediately be invalid. Therefore it is not allowed to return such a reference. Consider returning a value that takes ownership of local data instead of referencing it:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn get_dangling_reference() -> &'static i32 {
    let x = 0;
    &x
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::vec::IntoIter;

fn get_integer() -> i32 {
    let x = 0;
    x
}

fn get_owned_iterator() -> IntoIter<i32> {
    let v = vec![1, 2, 3];
    v.into_iter()
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0515.html


---

## E0516 — The typeof keyword is currently reserved but unimplemented.

Note: this error code is no longer emitted by the compiler. The typeof keyword is currently reserved but unimplemented. Try using type inference instead. Example:

**錯誤範例**
```rust
fn main() {
    let x: typeof(92) = 92;
}
```

**正解**
```rust
fn main() {
    let x = 92;
}
```

出處：https://doc.rust-lang.org/error_codes/E0516.html


---

## E0517 — This error code was replaced with the
attribute cannot be used on...

Note: this error code is no longer emitted by the compiler. This error code was replaced with the attribute cannot be used on... diagnostic that does not have an error code. A #[repr(..)] attribute was placed on an unsupported item. Examples of erroneous code: The #[repr(C)] attribute can only be placed on structs and enums. The #[repr(packed)] and #[repr(simd)] attributes only work on structs. The #[repr(u8)], #[repr(i16)], etc attributes only work on enums. These attributes do not work on typedefs, since typedefs are just aliases. Representations like #[repr(u8)], #[repr(i64)] are for selecting the discriminant size for enums. For enums with no data fields on any of the variants, e.g. enum Color {Red, Blue, Green}, this effectively sets the size of the enum to the size of the provided type. Such an enum can be cast to a value of the same type as well. In short, #[repr(u8)] makes a field-less enum behave like an integer with a constrained set of allowed values. For a description of how #[repr(C)] and representations like #[repr(u8)] affect the layout of enums with data fields, see RFC 2195. Only field-less enums can be cast to numerical primitives. Representations like #[repr(u8)] will not apply to structs. #[repr(packed)] reduces padding to make the struct size smaller. The representation of enums isn’t strictly defined in Rust, and this attribute won’t work on enums. #[repr(simd)] will give a struct consisting of a homogeneous series of machine types (i.e., u8, i32, etc) a representation that permits vectorization via SIMD. This doesn’t make much sense for enums since they don’t consist of a single list of data.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(C)]
type Foo = u8;

#[repr(packed)]
enum Foo {Bar, Baz}

#[repr(u8)]
struct Foo {bar: bool, baz: bool}

#[repr(C)]
impl Foo {
    // ...
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0517.html


---

## E0518 — An #[inline(..)] attribute was incorrectly placed on something other than a
function or method.

Note: this error code is no longer emitted by the compiler. An #[inline(..)] attribute was incorrectly placed on something other than a function or method. Example of erroneous code: #[inline] hints the compiler whether or not to attempt to inline a method or function. By default, the compiler does a pretty good job of figuring this out itself, but if you feel the need for annotations, #[inline(always)] and #[inline(never)] can override or force the compiler’s decision. If you wish to apply this attribute to all methods in an impl, manually annotate each method; it is not possible to annotate the entire impl with an #[inline] attribute.

**正解**
```rust
#[inline(always)]
struct Foo;

#[inline(never)]
impl Foo {
    // ...
}
```

出處：https://doc.rust-lang.org/error_codes/E0518.html


---

## E0519 — The current crate is indistinguishable from one of its dependencies, in terms
of metadata.

The current crate is indistinguishable from one of its dependencies, in terms of metadata. Example of erroneous code: a.rs b.rs The above example compiles two crates with exactly the same name and crate_type (plus any other metadata). This causes an error because it becomes impossible for the compiler to distinguish between symbols (pub item names). This error can be fixed by: Using Cargo, the Rust package manager, automatically fixing this issue. Recompiling the crate with different metadata (different name/ crate_type).

**正解**
```rust
#![crate_name = "a"]
#![crate_type = "lib"]

pub fn foo() {}
```

出處：https://doc.rust-lang.org/error_codes/E0519.html


---

## E0520 — A non-default implementation was already made on this type so it cannot be
specialized further.

A non-default implementation was already made on this type so it cannot be specialized further. Specialization only allows you to override default functions in implementations. To fix this error, you need to mark all the parent implementations as default. Example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(specialization)]

fn main() {
trait SpaceLlama {
    fn fly(&self);
}

// applies to all T
impl<T> SpaceLlama for T {
    default fn fly(&self) {}
}

// non-default impl
// applies to all `Clone` T and overrides the previous impl
impl<T: Clone> SpaceLlama for T {
    fn fly(&self) {}
}

// since `i32` is clone, this conflicts with the previous implementation
impl SpaceLlama for i32 {
    default fn fly(&self) {}
    // error: item `fly` is provided by an `impl` that specializes
    //        another, but the item in the parent `impl` is not marked
    //        `default` and so it cannot be specialized.
}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(specialization)]

fn main() {
trait SpaceLlama {
    fn fly(&self);
}

// applies to all T
impl<T> SpaceLlama for T {
    default fn fly(&self) {} // This is a parent implementation.
}

// applies to all `Clone` T; overrides the previous impl
impl<T: Clone> SpaceLlama for T {
    default fn fly(&self) {} // This is a parent implementation but was
                             // previously not a default one, causing the error
}

// applies to i32, overrides the previous two impls
impl SpaceLlama for i32 {
    fn fly(&self) {} // And now that's ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0520.html


---

## E0521 — Borrowed data escapes outside of closure.

Borrowed data escapes outside of closure. A type annotation of a closure parameter implies a new lifetime declaration. Consider to drop it, the compiler is reliably able to infer them. See the Closure type inference and annotation and Lifetime elision sections of the Book for more details.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let mut list: Vec<&str> = Vec::new();

let _add = |el: &str| {
    list.push(el); // error: `el` escapes the closure body here
};
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut list: Vec<&str> = Vec::new();

let _add = |el| {
    list.push(el);
};
}
```

出處：https://doc.rust-lang.org/error_codes/E0521.html


---

## E0522 — The lang attribute was used in an invalid context.

The lang attribute was used in an invalid context. The lang attribute is intended for marking special items that are built-in to Rust itself. This includes special traits (like Copy and Sized) that affect how the compiler behaves, as well as special functions that may be automatically invoked (such as the handler for out-of-bounds accesses when indexing a slice).

**錯誤範例**
```rust
#![allow(unused)]
#![feature(lang_items)]

fn main() {
#[lang = "cookie"]
fn cookie() -> ! { // error: definition of an unknown lang item: `cookie`
    loop {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0522.html


---

## E0523 — The compiler found multiple library files with the requested crate name.

Note: this error code is no longer emitted by the compiler. The compiler found multiple library files with the requested crate name. This error can occur in several different cases – for example, when using extern crate or passing --extern options without crate paths. It can also be caused by caching issues with the build directory, in which case cargo clean may help. In the above example, there are three different library files, all of which define the same crate name. Without providing a full path, there is no way for the compiler to know which crate it should use. Note that E0523 has been merged into E0464.

**錯誤範例**
```rust
// aux-build:crateresolve-1.rs
// aux-build:crateresolve-2.rs
// aux-build:crateresolve-3.rs

extern crate crateresolve;
//~^ ERROR multiple candidates for `rlib` dependency `crateresolve` found

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0523.html


---

## E0524 — A variable which requires unique access is being used in more than one closure
at the same time.

A variable which requires unique access is being used in more than one closure at the same time. To solve this issue, multiple solutions are available. First, is it required for this variable to be used in more than one closure at a time? If it is the case, use reference counted types such as Rc (or Arc if it runs concurrently): If not, just run closures one at a time:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn set(x: &mut isize) {
    *x += 4;
}

fn dragoooon(x: &mut isize) {
    let mut c1 = || set(x);
    let mut c2 = || set(x); // error!

    c2();
    c1();
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::rc::Rc;
use std::cell::RefCell;

fn set(x: &mut isize) {
    *x += 4;
}

fn dragoooon(x: &mut isize) {
    let x = Rc::new(RefCell::new(x));
    let y = Rc::clone(&x);
    let mut c1 = || { let mut x2 = x.borrow_mut(); set(&mut x2); };
    let mut c2 = || { let mut x2 = y.borrow_mut(); set(&mut x2); }; // ok!

    c2();
    c1();
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0524.html


---

## E0525 — A closure was used but didn’t implement the expected trait.

A closure was used but didn’t implement the expected trait. In the example above, closure is an FnOnce closure whereas the bar function expected an Fn closure. In this case, it’s simple to fix the issue, you just have to implement Copy and Clone traits on struct X and it’ll be ok: To better understand how these work in Rust, read the Closures chapter of the Book.

**錯誤範例**
```rust
struct X;

fn foo<T>(_: T) {}
fn bar<T: Fn(u32)>(_: T) {}

fn main() {
    let x = X;
    let closure = |_| foo(x); // error: expected a closure that implements
                              //        the `Fn` trait, but this closure only
                              //        implements `FnOnce`
    bar(closure);
}
```

**正解**
```rust
#[derive(Clone, Copy)] // We implement `Clone` and `Copy` traits.
struct X;

fn foo<T>(_: T) {}
fn bar<T: Fn(u32)>(_: T) {}

fn main() {
    let x = X;
    let closure = |_| foo(x);
    bar(closure); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0525.html


---

## E0527 — The number of elements in an array or slice pattern differed from the number of
elements in the array being matched.

The number of elements in an array or slice pattern differed from the number of elements in the array being matched. Example of erroneous code: Ensure that the pattern is consistent with the size of the matched array. Additional elements can be matched with ..:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let r = &[1, 2, 3, 4];
match r {
    &[a, b] => { // error: pattern requires 2 elements but array
                 //        has 4
        println!("a={}, b={}", a, b);
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let r = &[1, 2, 3, 4];
match r {
    &[a, b, ..] => { // ok!
        println!("a={}, b={}", a, b);
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0527.html


---

## E0528 — An array or slice pattern required more elements than were present in the
matched array.

An array or slice pattern required more elements than were present in the matched array. Example of erroneous code: Ensure that the matched array has at least as many elements as the pattern requires. You can match an arbitrary number of remaining elements with ..:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let r = &[1, 2];
match r {
    &[a, b, c, rest @ ..] => { // error: pattern requires at least 3
                               //        elements but array has 2
        println!("a={}, b={}, c={} rest={:?}", a, b, c, rest);
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let r = &[1, 2, 3, 4, 5];
match r {
    &[a, b, c, rest @ ..] => { // ok!
        // prints `a=1, b=2, c=3 rest=[4, 5]`
        println!("a={}, b={}, c={} rest={:?}", a, b, c, rest);
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0528.html


---

## E0529 — An array or slice pattern was matched against some other type.

An array or slice pattern was matched against some other type. Example of erroneous code: Ensure that the pattern and the expression being matched on are of consistent types:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let r: f32 = 1.0;
match r {
    [a, b] => { // error: expected an array or slice, found `f32`
        println!("a={}, b={}", a, b);
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let r = [1.0, 2.0];
match r {
    [a, b] => { // ok!
        println!("a={}, b={}", a, b);
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0529.html


---

## E0530 — A binding shadowed something it shouldn’t.

A binding shadowed something it shouldn’t. A match arm or a variable has a name that is already used by something else, e.g. struct name enum variant static associated constant This error may also happen when an enum variant with fields is used in a pattern, but without its fields. Match bindings cannot shadow statics: Fixed examples: or

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Enum {
    WithField(i32)
}

use Enum::*;
match WithField(1) {
    WithField => {} // error: missing (_)
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
static TEST: i32 = 0;

let r = 123;
match r {
    some_value => {} // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0530.html


---

## E0531 — An unknown tuple struct/variant has been used.

An unknown tuple struct/variant has been used. In most cases, it’s either a forgotten import or a typo. However, let’s look at how you can have such a type: Either way, it should work fine with our previous code:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let Type(x) = Type(12); // error!
match Bar(12) {
    Bar(x) => {} // error!
    _ => {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Type(u32); // this is a tuple struct

enum Foo {
    Bar(u32), // this is a tuple variant
}

use Foo::*; // To use Foo's variant directly, we need to import them in
            // the scope.
}
```

出處：https://doc.rust-lang.org/error_codes/E0531.html


---

## E0532 — Pattern arm did not match expected kind.

Pattern arm did not match expected kind. To fix this error, ensure the match arm kind is the same as the expression matched. Fixed example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum State {
    Succeeded,
    Failed(String),
}

fn print_on_failure(state: &State) {
    match *state {
        // error: expected unit struct, unit variant or constant, found tuple
        //        variant `State::Failed`
        State::Failed => println!("Failed"),
        _ => ()
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum State {
    Succeeded,
    Failed(String),
}

fn print_on_failure(state: &State) {
    match *state {
        State::Failed(ref msg) => println!("Failed with {}", msg),
        _ => ()
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0532.html


---

## E0533 — An item which isn’t a unit struct, a variant, nor a constant has been used as a
match pattern.

An item which isn’t a unit struct, a variant, nor a constant has been used as a match pattern. If you want to match against a value returned by a method, you need to bind the value first:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Tortoise;

impl Tortoise {
    fn turtle(&self) -> u32 { 0 }
}

match 0u32 {
    Tortoise::turtle => {} // Error!
    _ => {}
}
if let Tortoise::turtle = 0u32 {} // Same error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Tortoise;

impl Tortoise {
    fn turtle(&self) -> u32 { 0 }
}

match 0u32 {
    x if x == Tortoise.turtle() => {} // Bound into `x` then we compare it!
    _ => {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0533.html


---

## E0534 — Note: this error code is no longer emitted by the compiler
This is because it was too specific to the inline attribute.

Note: this error code is no longer emitted by the compiler This is because it was too specific to the inline attribute. Similar diagnostics occur for other attributes too. The example here will now emit E0805 The inline attribute was malformed. The parenthesized inline attribute requires the parameter to be specified: or: Alternatively, a paren-less version of the attribute may be used to hint the compiler about inlining opportunity: For more information see the inline attribute section of the Reference.

**錯誤範例**
```rust
#[inline()] // error: expected one argument
pub fn something() {}

fn main() {}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[inline(always)]
fn something() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0534.html


---

## E0535 — Note: this error code is no longer emitted by the compiler
This is because it was too specific to the inline attribute.

Note: this error code is no longer emitted by the compiler This is because it was too specific to the inline attribute. Similar diagnostics occur for other attributes too. The example here will now emit E0539 The inline attribute only supports two arguments: always never All other arguments given to the inline attribute will return this error. Example: For more information see the inline Attribute section of the Reference.

**錯誤範例**
```rust
#[inline(unknown)] // error: invalid argument
pub fn something() {}

fn main() {}
```

**正解**
```rust
#[inline(never)] // ok!
pub fn something() {}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0535.html


---

## E0536 — The not cfg-predicate was malformed.

Note: this error code is no longer emitted by the compiler. The not cfg-predicate was malformed. Erroneous code example (using cargo doc): The not predicate expects one cfg-pattern. Example: For more information about the cfg macro, read the section on Conditional Compilation in the Reference.

**正解**
```rust
#![feature(doc_cfg)]
#[doc(cfg(not()))]
pub fn main() {

}
```

出處：https://doc.rust-lang.org/error_codes/E0536.html


---

## E0537 — Note: this error code is no longer emitted by the compiler
An unknown predicate was used inside the cfg attribute.

Note: this error code is no longer emitted by the compiler An unknown predicate was used inside the cfg attribute. The cfg attribute supports only three kinds of predicates: any all not Example: For more information about the cfg attribute, read the section on Conditional Compilation in the Reference.

**錯誤範例**
```rust
#[cfg(unknown())] // error: invalid predicate `unknown`
pub fn something() {}

pub fn main() {}
```

**正解**
```rust
#[cfg(not(target_os = "linux"))] // ok!
pub fn something() {}

pub fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0537.html


---

## E0538 — Attribute contains same meta item more than once.

Attribute contains same meta item more than once. Meta items are the key-value pairs inside of an attribute. Each key may only be used once in each attribute. To fix the problem, remove all but one of the meta items with the same key. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[deprecated(
    since="1.0.0",
    note="First deprecation note.",
    note="Second deprecation note." // error: multiple same meta item
)]
fn deprecated_function() {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[deprecated(
    since="1.0.0",
    note="First deprecation note."
)]
fn deprecated_function() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0538.html


---

## E0539 — An invalid meta-item was used inside an attribute.

An invalid meta-item was used inside an attribute. To fix the above example, you can write the following: Several causes of this are, an attribute may have expected you to give a list but you gave a name = value pair: Or a name = value pair, but you gave a list: Or it expected some specific word but you gave an unexpected one:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[deprecated(note)] // error!
#[unstable(feature = "deprecated_fn", issue = "123")]
fn deprecated() {}

#[unstable(feature = "unstable_struct", issue)] // error!
struct Unstable;

#[rustc_const_unstable(feature)] // error!
const fn unstable_fn() {}

#[stable(feature = "stable_struct", since)] // error!
struct Stable;

#[rustc_const_stable(feature)] // error!
const fn stable_fn() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[deprecated(since = "1.39.0", note = "reason")] // ok!
#[unstable(feature = "deprecated_fn", issue = "123")]
fn deprecated() {}

#[unstable(feature = "unstable_struct", issue = "123")] // ok!
struct Unstable;

#[rustc_const_unstable(feature = "unstable_fn", issue = "124")] // ok!
const fn unstable_fn() {}

#[stable(feature = "stable_struct", since = "1.39.0")] // ok!
struct Stable;

#[stable(feature = "stable_fn", since = "1.39.0")]
#[rustc_const_stable(feature = "stable_fn", since = "1.39.0")] // ok!
const fn stable_fn() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0539.html


---

## E0541 — An unknown meta item was used.

Note: this error code is no longer emitted by the compiler. An unknown meta item was used. Meta items are the key-value pairs inside of an attribute. The keys provided must be one of the valid keys for the specified attribute. To fix the problem, either remove the unknown meta item, or rename it if you provided the wrong name. In the erroneous code example above, the wrong name was provided, so changing to a correct one it will fix the error. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[deprecated(
    since="1.0.0",
    // error: unknown meta item
    reason="Example invalid meta item. Should be 'note'")
]
fn deprecated_function() {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[deprecated(
    since="1.0.0",
    note="This is a valid meta item for the deprecated attribute."
)]
fn deprecated_function() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0541.html


---

## E0542 — The since value is missing in a stability attribute.

The since value is missing in a stability attribute. To fix this issue, you need to provide the since field. Example: See the How Rust is Made and “Nightly Rust” appendix of the Book and the Stability attributes section of the Rustc Dev Guide for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[stable(feature = "_stable_fn")] // invalid
fn _stable_fn() {}

#[rustc_const_stable(feature = "_stable_const_fn")] // invalid
const fn _stable_const_fn() {}

#[stable(feature = "_deprecated_fn", since = "0.1.0")]
#[deprecated(
    note = "explanation for deprecation"
)] // invalid
fn _deprecated_fn() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[stable(feature = "_stable_fn", since = "1.0.0")] // ok!
fn _stable_fn() {}

#[stable(feature = "_stable_const_fn", since = "1.0.0")]
#[rustc_const_stable(feature = "_stable_const_fn", since = "1.0.0")] // ok!
const fn _stable_const_fn() {}

#[stable(feature = "_deprecated_fn", since = "0.1.0")]
#[deprecated(
    since = "1.0.0",
    note = "explanation for deprecation"
)] // ok!
fn _deprecated_fn() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0542.html


---

## E0543 — The note value is missing in a stability attribute.

The note value is missing in a stability attribute. To fix this issue, you need to provide the note field. Example: See the How Rust is Made and “Nightly Rust” appendix of the Book and the Stability attributes section of the Rustc Dev Guide for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[stable(since = "0.1.0", feature = "_deprecated_fn")]
#[deprecated(
    since = "1.0.0"
)] // invalid
fn _deprecated_fn() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[stable(since = "0.1.0", feature = "_deprecated_fn")]
#[deprecated(
    since = "1.0.0",
    note = "explanation for deprecation"
)] // ok!
fn _deprecated_fn() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0543.html


---

## E0544 — Multiple stability attributes were declared on the same item.

Multiple stability attributes were declared on the same item. To fix this issue, ensure that each item has at most one stability attribute. See the How Rust is Made and “Nightly Rust” appendix of the Book and the Stability attributes section of the Rustc Dev Guide for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "rust1")]

fn main() {
#[stable(feature = "rust1", since = "1.0.0")]
#[stable(feature = "test", since = "2.0.0")] // invalid
fn foo() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "rust1")]

fn main() {
#[stable(feature = "test", since = "2.0.0")] // ok!
fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0544.html


---

## E0545 — The issue value is incorrect in a stability attribute.

The issue value is incorrect in a stability attribute. To fix this issue, you need to provide a correct value in the issue field. Example: See the How Rust is Made and “Nightly Rust” appendix of the Book and the Stability attributes section of the Rustc Dev Guide for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[unstable(feature = "_unstable_fn", issue = "0")] // invalid
fn _unstable_fn() {}

#[rustc_const_unstable(feature = "_unstable_const_fn", issue = "0")] // invalid
const fn _unstable_const_fn() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[unstable(feature = "_unstable_fn", issue = "none")] // ok!
fn _unstable_fn() {}

#[rustc_const_unstable(feature = "_unstable_const_fn", issue = "1")] // ok!
const fn _unstable_const_fn() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0545.html


---

## E0546 — The feature value is missing in a stability attribute.

The feature value is missing in a stability attribute. To fix this issue, you need to provide the feature field. Example: See the How Rust is Made and “Nightly Rust” appendix of the Book and the Stability attributes section of the Rustc Dev Guide for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[unstable(issue = "none")] // invalid
fn unstable_fn() {}

#[stable(since = "1.0.0")] // invalid
fn stable_fn() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[unstable(feature = "unstable_fn", issue = "none")] // ok!
fn unstable_fn() {}

#[stable(feature = "stable_fn", since = "1.0.0")] // ok!
fn stable_fn() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0546.html


---

## E0547 — The issue value is missing in a stability attribute.

The issue value is missing in a stability attribute. To fix this issue, you need to provide the issue field. Example: See the How Rust is Made and “Nightly Rust” appendix of the Book and the Stability attributes section of the Rustc Dev Guide for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[unstable(feature = "_unstable_fn")] // invalid
fn _unstable_fn() {}

#[rustc_const_unstable(feature = "_unstable_const_fn")] // invalid
const fn _unstable_const_fn() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[unstable(feature = "_unstable_fn", issue = "none")] // ok!
fn _unstable_fn() {}

#[rustc_const_unstable(
    feature = "_unstable_const_fn",
    issue = "none"
)] // ok!
const fn _unstable_const_fn() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0547.html


---

## E0549 — A deprecated attribute wasn’t paired with a stable/unstable attribute with
#![feature(staged_api)] enabled.

A deprecated attribute wasn’t paired with a stable/unstable attribute with #![feature(staged_api)] enabled. To fix this issue, you need to add also an attribute stable or unstable. Example: See the How Rust is Made and “Nightly Rust” appendix of the Book and the Stability attributes section of the Rustc Dev Guide for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[deprecated(
    since = "1.0.1",
    note = "explanation for deprecation"
)] // invalid
fn _deprecated_fn() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(staged_api)]
#![allow(internal_features)]
#![stable(since = "1.0.0", feature = "test")]

fn main() {
#[stable(since = "1.0.0", feature = "test")]
#[deprecated(
    since = "1.0.1",
    note = "explanation for deprecation"
)] // ok!
fn _deprecated_fn() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0549.html


---

## E0550 — Note: this error code is no longer emitted by the compiler
More than one deprecated attribute has been put on an item.

Note: this error code is no longer emitted by the compiler More than one deprecated attribute has been put on an item. The deprecated attribute can only be present once on an item.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[deprecated(note = "because why not?")]
#[deprecated(note = "right?")] // error!
fn the_banished() {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[deprecated(note = "because why not, right?")]
fn the_banished() {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0550.html


---

## E0551 — Note: this error code is no longer emitted by the compiler
An invalid meta-item was used inside an attribute.

Note: this error code is no longer emitted by the compiler An invalid meta-item was used inside an attribute. Meta items are the key-value pairs inside of an attribute. To fix this issue, you need to give a value to the note key. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[deprecated(note)] // error!
fn i_am_deprecated() {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[deprecated(note = "because")] // ok!
fn i_am_deprecated() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0551.html


---

## E0552 — This error code was replaced by E0539.

Note: this error code is no longer emitted by the compiler. This error code was replaced by E0539. A unrecognized representation attribute was used. You can use a repr attribute to tell the compiler how you want a struct or enum to be laid out in memory. Make sure you’re using one of the supported options: For more information about specifying representations, see the “Alternative Representations” section of the Rustonomicon.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(D)] // error: unrecognized representation hint
struct MyStruct {
    my_field: usize
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(C)] // ok!
struct MyStruct {
    my_field: usize
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0552.html


---

## E0554 — Feature attributes are only allowed on the nightly release channel. Stable or
beta compilers will not comply.

Feature attributes are only allowed on the nightly release channel. Stable or beta compilers will not comply. If you need the feature, make sure to use a nightly release of the compiler (but be warned that the feature may be removed or altered in the future).

**正解**
```rust
#![feature(lang_items)] // error: `#![feature]` may not be used on the
                        //        stable release channel
```

出處：https://doc.rust-lang.org/error_codes/E0554.html


---

## E0556 — The feature attribute was badly formed.

Note: this error code is no longer emitted by the compiler. The feature attribute was badly formed. The feature attribute only accept a “feature flag” and can only be used on nightly. Example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(foo_bar_baz, foo(bar), foo = "baz", foo)] // error!
#![feature] // error!
#![feature = "foo"] // error!
fn main() {
}
```

**正解**
```rust
#![feature(flag)]
```

出處：https://doc.rust-lang.org/error_codes/E0556.html


---

## E0557 — A feature attribute named a feature that has been removed.

A feature attribute named a feature that has been removed. Delete the offending feature attribute.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(managed_boxes)] // error: feature has been removed
fn main() {
}
```

出處：https://doc.rust-lang.org/error_codes/E0557.html


---

## E0559 — An unknown field was specified into an enum’s structure variant.

An unknown field was specified into an enum’s structure variant. Verify you didn’t misspell the field’s name or that the field exists. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Field {
    Fool { x: u32 },
}

let s = Field::Fool { joke: 0 };
// error: struct variant `Field::Fool` has no field named `joke`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Field {
    Fool { joke: u32 },
}

let s = Field::Fool { joke: 0 }; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0559.html


---

## E0560 — An unknown field was specified into a structure.

An unknown field was specified into a structure. Verify you didn’t misspell the field’s name or that the field exists. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Simba {
    mother: u32,
}

let s = Simba { mother: 1, father: 0 };
// error: structure `Simba` has no field named `father`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Simba {
    mother: u32,
    father: u32,
}

let s = Simba { mother: 1, father: 0 }; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0560.html


---

## E0561 — A non-ident or non-wildcard pattern has been used as a parameter of a function
pointer type.

A non-ident or non-wildcard pattern has been used as a parameter of a function pointer type. When using an alias over a function type, you cannot e.g. denote a parameter as being mutable. To fix the issue, remove patterns (_ is allowed though). Example: You can also omit the parameter name:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
type A1 = fn(mut param: u8); // error!
type A2 = fn(&param: u32); // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
type A1 = fn(param: u8); // ok!
type A2 = fn(_: u32); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0561.html


---

## E0562 — impl Trait is only allowed as a function return and argument type.

impl Trait is only allowed as a function return and argument type. Make sure impl Trait appears in a function signature. See the reference for more details on impl Trait.

**錯誤範例**
```rust
fn main() {
    let count_to_ten: impl Iterator<Item=usize> = 0..10;
    // error: `impl Trait` not allowed outside of function and inherent method
    //        return types
    for i in count_to_ten {
        println!("{}", i);
    }
}
```

**正解**
```rust
fn count_to_n(n: usize) -> impl Iterator<Item=usize> {
    0..n
}

fn main() {
    for i in count_to_n(10) {  // ok!
        println!("{}", i);
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0562.html


---

## E0565 — A literal was used in a built-in attribute that doesn’t support literals.

A literal was used in a built-in attribute that doesn’t support literals. Not all attributes support literals in their input, and in some cases they expect an identifier instead. That would be the solution in the case of repr:

**錯誤範例**
```rust
#[repr("C")] // error: meta item in `repr` must be an identifier
struct Repr {}

fn main() {}
```

**正解**
```rust
#[repr(C)] // ok!
struct Repr {}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0565.html


---

## E0566 — Conflicting representation hints have been used on a same item.

Conflicting representation hints have been used on a same item. In most cases (if not all), using just one representation hint is more than enough. If you want to have a representation hint depending on the current architecture, use cfg_attr. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(u32, u64)]
enum Repr { A }
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[cfg_attr(linux, repr(u32))]
#[cfg_attr(not(linux), repr(u64))]
enum Repr { A }
}
```

出處：https://doc.rust-lang.org/error_codes/E0566.html


---

## E0567 — Generics have been used on an auto trait.

Generics have been used on an auto trait. Since an auto trait is implemented on all existing types, the compiler would not be able to infer the types of the trait’s generic parameters. To fix this issue, just remove the generics:

**錯誤範例**
```rust
#![feature(auto_traits)]

auto trait Generic<T> {} // error!
fn main() {}
```

**正解**
```rust
#![feature(auto_traits)]

auto trait Generic {} // ok!
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0567.html


---

## E0568 — A super trait has been added to an auto trait.

A super trait has been added to an auto trait. Since an auto trait is implemented on all existing types, adding a super trait would filter out a lot of those types. In the current example, almost none of all the existing types could implement Bound because very few of them have the Copy trait. To fix this issue, just remove the super trait:

**錯誤範例**
```rust
#![feature(auto_traits)]

auto trait Bound : Copy {} // error!

fn main() {}
```

**正解**
```rust
#![feature(auto_traits)]

auto trait Bound {} // ok!

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0568.html


---

## E0569 — If an impl has a generic parameter with the #[may_dangle] attribute, then
that impl must be declared as an unsafe impl.

If an impl has a generic parameter with the #[may_dangle] attribute, then that impl must be declared as an unsafe impl. In this example, we are asserting that the destructor for Foo will not access any data of type X, and require this assertion to be true for overall safety in our program. The compiler does not currently attempt to verify this assertion; therefore we must tag this impl as unsafe.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(dropck_eyepatch)]

fn main() {
struct Foo<X>(X);
impl<#[may_dangle] X> Drop for Foo<X> {
    fn drop(&mut self) { }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0569.html


---

## E0570 — The requested ABI is unsupported by the current target.

The requested ABI is unsupported by the current target. The Rust compiler maintains a list of unsupported ABIs for each target. If an ABI is present in such a list, this usually means that the target / ABI combination is currently unsupported by llvm. If necessary, you can circumvent this check using custom target specifications.

出處：https://doc.rust-lang.org/error_codes/E0570.html


---

## E0571 — A break statement with an argument appeared in a non-loop loop.

A break statement with an argument appeared in a non-loop loop. Example of erroneous code: The break statement can take an argument (which will be the value of the loop expression if the break statement is executed) in loop loops, but not for, while, or while let loops. Make sure break value; statements only occur in loop loops:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let mut i = 1;
fn satisfied(n: usize) -> bool { n % 23 == 0 }
let result = while true {
    if satisfied(i) {
        break 2 * i; // error: `break` with value from a `while` loop
    }
    i += 1;
};
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut i = 1;
fn satisfied(n: usize) -> bool { n % 23 == 0 }
let result = loop { // This is now a "loop" loop.
    if satisfied(i) {
        break 2 * i; // ok!
    }
    i += 1;
};
}
```

出處：https://doc.rust-lang.org/error_codes/E0571.html


---

## E0572 — A return statement was found outside of a function body.

A return statement was found outside of a function body. To fix this issue, just remove the return keyword or move the expression into a function. Example:

**錯誤範例**
```rust
const FOO: u32 = return 0; // error: return statement outside of function body

fn main() {}
```

**正解**
```rust
const FOO: u32 = 0;

fn some_fn() -> u32 {
    return FOO;
}

fn main() {
    some_fn();
}
```

出處：https://doc.rust-lang.org/error_codes/E0572.html


---

## E0573 — Something other than a type has been used when one was expected.

Something other than a type has been used when one was expected. Erroneous code examples: In all these errors, a type was expected. For example, in the first error, if we want to return the Born variant from the Dragon enum, we must set the function to return the enum and not its variant: In the second error, you can’t implement something on an item, only on types. We would need to create a new type if we wanted to do something similar: In the third case, we tried to only expect one variant of the Wizard enum, which is not possible. To make this work, we need to using pattern matching over the Wizard enum:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Dragon {
    Born,
}

fn oblivion() -> Dragon::Born { // error!
    Dragon::Born
}

const HOBBIT: u32 = 2;
impl HOBBIT {} // error!

enum Wizard {
    Gandalf,
    Saruman,
}

trait Isengard {
    fn wizard(_: Wizard::Saruman); // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Dragon {
    Born,
}

fn oblivion() -> Dragon { // ok!
    Dragon::Born
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0573.html


---

## E0574 — Something other than a struct, variant or union has been used when one was
expected.

Something other than a struct, variant or union has been used when one was expected. In all these errors, a type was expected. For example, in the first error, we tried to instantiate the mordor module, which is impossible. If you want to instantiate a type inside a module, you can do it as follow: In the second error, we tried to bind the Jak enum directly, which is not possible: you can only bind one of its variants. To do so:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod mordor {}

let sauron = mordor { x: () }; // error!

enum Jak {
    Daxter { i: isize },
}

let eco = Jak::Daxter { i: 1 };
match eco {
    Jak { i } => {} // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
mod mordor {
    pub struct TheRing {
        pub x: usize,
    }
}

let sauron = mordor::TheRing { x: 1 }; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0574.html


---

## E0575 — Something other than a type or an associated type was given.

Something other than a type or an associated type was given. In both cases, we’re declaring a variable (called _) and we’re giving it a type. However, <u8 as Rick>::Morty and <u8 as Age>::Mythology aren’t types, therefore the compiler throws an error. <u8 as Rick>::Morty is an enum variant, you cannot use a variant as a type, you have to use the enum directly: <u8 as Age>::Mythology is a trait method, which is definitely not a type. However, the Age trait provides an associated type Empire which can be used as a type:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Rick { Morty }

let _: <u8 as Rick>::Morty; // error!

trait Age {
    type Empire;
    fn Mythology() {}
}

impl Age for u8 {
    type Empire = u16;
}

let _: <u8 as Age>::Mythology; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum Rick { Morty }

let _: Rick; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0575.html


---

## E0576 — An associated item wasn’t found in the given type.

An associated item wasn’t found in the given type. In this example, we tried to use the nonexistent associated type You of the Hello trait. To fix this error, use an existing associated type:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Hello {
    type Who;

    fn hello() -> <Self as Hello>::You; // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Hello {
    type Who;

    fn hello() -> <Self as Hello>::Who; // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0576.html


---

## E0577 — Something other than a module was found in visibility scope.

Something other than a module was found in visibility scope. Sea is not a module, therefore it is invalid to use it in a visibility path. To fix this error we need to ensure sea is a module. Please note that the visibility scope can only be applied on ancestors!

**錯誤範例**
```rust
pub enum Sea {}

pub (in crate::Sea) struct Shark; // error!

fn main() {}
```

**正解**
```rust
pub mod sea {
    pub (in crate::sea) struct Shark; // ok!
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0577.html


---

## E0578 — A module cannot be found and therefore, the visibility cannot be determined.

Note: this error code is no longer emitted by the compiler. A module cannot be found and therefore, the visibility cannot be determined. Because of the call to the foo macro, the compiler guesses that the missing module could be inside it and fails because the macro definition cannot be found. To fix this error, please be sure that the module is in scope:

**正解**
```rust
foo!();

pub (in ::Sea) struct Shark; // error!

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0578.html


---

## E0579 — A lower range wasn’t less than the upper range.

A lower range wasn’t less than the upper range. When matching against an exclusive range, the compiler verifies that the range is non-empty. Exclusive range patterns include the start point but not the end point, so this is equivalent to requiring the start of the range to be less than the end of the range.

**錯誤範例**
```rust
fn main() {
    match 5u32 {
        // This range is ok, albeit pointless.
        1..2 => {}
        // This range is empty, and the compiler can tell.
        5..5 => {} // error!
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0579.html


---

## E0580 — The main function was incorrectly declared.

The main function was incorrectly declared. The main function prototype should never take arguments. Example: If you want to get command-line arguments, use std::env::args. To exit with a specified exit code, use std::process::exit.

**錯誤範例**
```rust
fn main(x: i32) { // error: main function has wrong type
    println!("{}", x);
}
```

**正解**
```rust
fn main() {
    // your code
}
```

出處：https://doc.rust-lang.org/error_codes/E0580.html


---

## E0581 — In a fn type, a lifetime appears only in the return type
and not in the arguments types.

In a fn type, a lifetime appears only in the return type and not in the arguments types. The problem here is that the lifetime isn’t constrained by any of the arguments, making it impossible to determine how long it’s supposed to live. To fix this issue, either use the lifetime in the arguments, or use the 'static lifetime. Example: Note: The examples above used to be (erroneously) accepted by the compiler, but this was since corrected. See issue #33685 for more details.

**錯誤範例**
```rust
fn main() {
    // Here, `'a` appears only in the return type:
    let x: for<'a> fn() -> &'a i32;
}
```

**正解**
```rust
fn main() {
    // Here, `'a` appears only in the return type:
    let x: for<'a> fn(&'a i32) -> &'a i32;
    let y: fn() -> &'static i32;
}
```

出處：https://doc.rust-lang.org/error_codes/E0581.html


---

## E0582 — A lifetime is only present in an associated-type binding, and not in the input
types to the trait.

A lifetime is only present in an associated-type binding, and not in the input types to the trait. To fix this issue, either use the lifetime in the inputs, or use 'static. Example: This error also includes the use of associated types with lifetime parameters. The latter scenario encounters this error because Foo::Assoc<'a> could be implemented by a type that does not use the 'a parameter, so there is no guarantee that X::Assoc<'a> actually uses 'a. To fix this we can pass a dummy parameter: Note: The examples above used to be (erroneously) accepted by the compiler, but this was since corrected. See issue #33685 for more details.

**錯誤範例**
```rust
fn bar<F>(t: F)
    // No type can satisfy this requirement, since `'a` does not
    // appear in any of the input types (here, `i32`):
    where F: for<'a> Fn(i32) -> Option<&'a i32>
{
}

fn main() { }
```

**正解**
```rust
fn bar<F, G>(t: F, u: G)
    where F: for<'a> Fn(&'a i32) -> Option<&'a i32>,
          G: Fn(i32) -> Option<&'static i32>,
{
}

fn main() { }
```

出處：https://doc.rust-lang.org/error_codes/E0582.html


---

## E0583 — A file wasn’t found for an out-of-line module.

A file wasn’t found for an out-of-line module. Please be sure that a file corresponding to the module exists. If you want to use a module named file_that_doesnt_exist, you need to have a file named file_that_doesnt_exist.rs or file_that_doesnt_exist/mod.rs in the same directory.

**錯誤範例**
```rust
mod file_that_doesnt_exist; // error: file not found for module

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0583.html


---

## E0584 — A doc comment that is not attached to anything has been encountered.

A doc comment that is not attached to anything has been encountered. A little reminder: a doc comment has to be placed before the item it’s supposed to document. So if you want to document the Island trait, you need to put a doc comment before it, not inside it. Same goes for the lost method: the doc comment needs to be before it:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Island {
    fn lost();

    /// I'm lost!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
/// I'm THE island!
trait Island {
    /// I'm lost!
    fn lost();
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0584.html


---

## E0585 — A documentation comment that doesn’t document anything was found.

A documentation comment that doesn’t document anything was found. Documentation comments need to be followed by items, including functions, types, modules, etc. Examples:

**錯誤範例**
```rust
fn main() {
    // The following doc comment will fail:
    /// This is a useless doc comment!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
/// I'm documenting the following struct:
struct Foo;

/// I'm documenting the following function:
fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0585.html


---

## E0586 — An inclusive range was used with no end.

An inclusive range was used with no end. An inclusive range needs an end in order to include it. If you just need a start and no end, use a non-inclusive range (with ..): Or put an end to your inclusive range:

**錯誤範例**
```rust
fn main() {
    let tmp = vec![0, 1, 2, 3, 4, 4, 3, 3, 2, 1];
    let x = &tmp[1..=]; // error: inclusive range was used with no end
}
```

**正解**
```rust
fn main() {
    let tmp = vec![0, 1, 2, 3, 4, 4, 3, 3, 2, 1];
    let x = &tmp[1..]; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0586.html


---

## E0587 — A type has both packed and align representation hints.

A type has both packed and align representation hints. You cannot use packed and align hints on a same type. If you want to pack a type to a given size, you should provide a size to packed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(packed, align(8))] // error!
struct Umbrella(i32);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(packed(8))] // ok!
struct Umbrella(i32);
}
```

出處：https://doc.rust-lang.org/error_codes/E0587.html


---

## E0588 — A type with packed representation hint has a field with align
representation hint.

A type with packed representation hint has a field with align representation hint. Just like you cannot have both align and packed representation hints on the same type, a packed type cannot contain another type with the align representation hint. However, you can do the opposite:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(align(16))]
struct Aligned(i32);

#[repr(packed)] // error!
struct Packed(Aligned);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(packed)]
struct Packed(i32);

#[repr(align(16))] // ok!
struct Aligned(Packed);
}
```

出處：https://doc.rust-lang.org/error_codes/E0588.html


---

## E0589 — The value of N that was specified for repr(align(N)) was not a power
of two, or was greater than 2^29.

The value of N that was specified for repr(align(N)) was not a power of two, or was greater than 2^29.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(align(15))] // error: invalid `repr(align)` attribute: not a power of two
enum Foo {
    Bar(u64),
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0589.html


---

## E0590 — break or continue keywords were used in a condition of a while loop
without a label.

break or continue keywords were used in a condition of a while loop without a label. Erroneous code code: break or continue must include a label when used in the condition of a while loop. To fix this, add a label specifying which loop is being broken out of:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
while break {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
'foo: while break 'foo {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0590.html


---

## E0591 — Per RFC 401, if you have a function declaration foo:

Per RFC 401, if you have a function declaration foo: the type of foo is not fn(S), as one might expect. Rather, it is a unique, zero-sized marker type written here as typeof(foo). However, typeof(foo) can be coerced to a function pointer fn(S), so you rarely notice this: The reason that this matter is that the type fn(S) is not specific to any particular function: it’s a function pointer. So calling x() results in a virtual call, whereas foo() is statically dispatched, because the type of foo tells us precisely what function is being called. As noted above, coercions mean that most code doesn’t have to be concerned with this distinction. However, you can tell the difference when using transmute to convert a fn item into a fn pointer. This is sometimes done as part of an FFI: Here, transmute is being used to convert the types of the fn arguments. This pattern is incorrect because the type of foo is a function item (typeof(foo)), which is zero-sized, and the target type (fn()) is a function pointer, which is not zero-sized. This pattern should be rewritten. There are a few possible ways to do this: change the original fn declaration to match the expected signature, and do the cast in the fn body (the preferred option) cast the fn item of a fn pointer before calling transmute, as shown here: The same applies to transmutes to *mut fn(), which were observed in practice. Note though that use of this type is generally incorrect. The intention is typically to describe a function pointer, but just fn() alone suffices for that. *mut fn() is a pointer to a fn pointer. (Since these values are typically just passed to C code, however, this rarely makes a difference in practice.)

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
extern "C" fn foo(userdata: Box<i32>) {
    /* ... */
}

fn callback(_: extern "C" fn(*mut i32)) {}
use std::mem::transmute;
unsafe {
    let f: extern "C" fn(*mut i32) = transmute(foo);
    callback(f);
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct S;

// For the purposes of this explanation, all of these
// different kinds of `fn` declarations are equivalent:

fn foo(x: S) { /* ... */ }
#[cfg(for_demonstration_only)]
extern "C" {
    fn foo(x: S);
}
#[cfg(for_demonstration_only)]
impl S {
    fn foo(self) { /* ... */ }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0591.html


---

## E0592 — This error occurs when you defined methods or associated functions with same
name.

This error occurs when you defined methods or associated functions with same name. A similar error is E0201. The difference is whether there is one declaration block or not. To avoid this error, you must give each fn a unique name.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Foo {
    fn bar() {} // previous definition here
}

impl Foo {
    fn bar() {} // duplicate definition here
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Foo {
    fn bar() {}
}

impl Foo {
    fn baz() {} // define with different name
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0592.html


---

## E0593 — You tried to supply an Fn-based type with an incorrect number of arguments
than what was expected.

You tried to supply an Fn-based type with an incorrect number of arguments than what was expected. You have to provide the same number of arguments as expected by the Fn-based type. So to fix the previous example, we need to remove the y argument:

**錯誤範例**
```rust
fn foo<F: Fn()>(x: F) { }

fn main() {
    // [E0593] closure takes 1 argument but 0 arguments are required
    foo(|y| { });
}
```

**正解**
```rust
fn foo<F: Fn()>(x: F) { }

fn main() {
    foo(|| { }); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0593.html


---

## E0594 — A non-mutable value was assigned a value.

A non-mutable value was assigned a value. To fix this error, declare ss as mutable by using the mut keyword:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct SolarSystem {
    earth: i32,
}

let ss = SolarSystem { earth: 3 };
ss.earth = 2; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct SolarSystem {
    earth: i32,
}

let mut ss = SolarSystem { earth: 3 }; // declaring `ss` as mutable
ss.earth = 2; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0594.html


---

## E0595 — Closures cannot mutate immutable captured variables.

Note: this error code is no longer emitted by the compiler. Closures cannot mutate immutable captured variables. Make the variable binding mutable:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = 3; // error: closure cannot assign to immutable local variable `x`
let mut c = || { x += 1 };
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut x = 3; // ok!
let mut c = || { x += 1 };
}
```

出處：https://doc.rust-lang.org/error_codes/E0595.html


---

## E0596 — This error occurs because you tried to mutably borrow a non-mutable variable.

This error occurs because you tried to mutably borrow a non-mutable variable. In here, x isn’t mutable, so when we try to mutably borrow it in y, it fails. To fix this error, you need to make x mutable:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = 1;
let y = &mut x; // error: cannot borrow mutably
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut x = 1;
let y = &mut x; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0596.html


---

## E0597 — This error occurs because a value was dropped while it was still borrowed.

This error occurs because a value was dropped while it was still borrowed. Here, y is dropped at the end of the inner scope, but it is borrowed by x until the println. To fix the previous example, just remove the scope so that y isn’t dropped until after the println

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a> {
    x: Option<&'a u32>,
}

let mut x = Foo { x: None };
{
    let y = 0;
    x.x = Some(&y); // error: `y` does not live long enough
}
println!("{:?}", x.x);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a> {
    x: Option<&'a u32>,
}

let mut x = Foo { x: None };

let y = 0;
x.x = Some(&y);

println!("{:?}", x.x);
}
```

出處：https://doc.rust-lang.org/error_codes/E0597.html


---

## E0599 — This error occurs when a method is used on a type which doesn’t implement it:

This error occurs when a method is used on a type which doesn’t implement it: In this case, you need to implement the chocolate method to fix the error:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Mouth;

let x = Mouth;
x.chocolate(); // error: no method named `chocolate` found for type `Mouth`
               //        in the current scope
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Mouth;

impl Mouth {
    fn chocolate(&self) { // We implement the `chocolate` method here.
        println!("Hmmm! I love chocolate!");
    }
}

let x = Mouth;
x.chocolate(); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0599.html


---

## E0600 — An unary operator was used on a type which doesn’t implement it.

An unary operator was used on a type which doesn’t implement it. In this case, Question would need to implement the std::ops::Not trait in order to be able to use ! on it. Let’s implement it:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum Question {
    Yes,
    No,
}

!Question::Yes; // error: cannot apply unary operator `!` to type `Question`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::ops::Not;

enum Question {
    Yes,
    No,
}

// We implement the `Not` trait on the enum.
impl Not for Question {
    type Output = bool;

    fn not(self) -> bool {
        match self {
            Question::Yes => false, // If the `Answer` is `Yes`, then it
                                    // returns false.
            Question::No => true, // And here we do the opposite.
        }
    }
}

assert_eq!(!Question::Yes, false);
assert_eq!(!Question::No, true);
}
```

出處：https://doc.rust-lang.org/error_codes/E0600.html


---

## E0601 — No main function was found in a binary crate.

No main function was found in a binary crate. To fix this error, add a main function: If you don’t know the basics of Rust, you can look at the Rust Book to get started.

**正解**
```rust
fn main() {
    // Your program will start here.
    println!("Hello world!");
}
```

出處：https://doc.rust-lang.org/error_codes/E0601.html


---

## E0602 — An unknown or invalid lint was used on the command line.

An unknown or invalid lint was used on the command line. Maybe you just misspelled the lint name or the lint doesn’t exist anymore. Either way, try to update/remove it in order to fix the error.

**正解**
```rust
rustc -D bogus rust_file.rs
```

出處：https://doc.rust-lang.org/error_codes/E0602.html


---

## E0603 — A private item was used outside its scope.

A private item was used outside its scope. In order to fix this error, you need to make the item public by using the pub keyword. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod foo {
    const PRIVATE: u32 = 0x_a_bad_1dea_u32; // This const is private, so we
                                            // can't use it outside of the
                                            // `foo` module.
}

println!("const value: {}", foo::PRIVATE); // error: constant `PRIVATE`
                                                  //        is private
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
mod foo {
    pub const PRIVATE: u32 = 0x_a_bad_1dea_u32; // We set it public by using the
                                                // `pub` keyword.
}

println!("const value: {}", foo::PRIVATE); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0603.html


---

## E0604 — A cast to char was attempted on a type other than u8.

A cast to char was attempted on a type other than u8. char is a Unicode Scalar Value, an integer value from 0 to 0xD7FF and 0xE000 to 0x10FFFF. (The gap is for surrogate pairs.) Only u8 always fits in those ranges so only u8 may be cast to char. To allow larger values, use char::from_u32, which checks the value is valid. For more information about casts, take a look at the Type cast section in The Reference Book.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
0u32 as char; // error: only `u8` can be cast as `char`, not `u32`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
assert_eq!(86u8 as char, 'V'); // ok!
assert_eq!(char::from_u32(0x3B1), Some('α')); // ok!
assert_eq!(char::from_u32(0xD800), None); // not a USV.
}
```

出處：https://doc.rust-lang.org/error_codes/E0604.html


---

## E0605 — An invalid cast was attempted.

An invalid cast was attempted. Erroneous code examples: Only primitive types can be cast into each other. Examples: For more information about casts, take a look at the Type cast section in The Reference Book.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = 0u8;
x as Vec<u8>; // error: non-primitive cast: `u8` as `std::vec::Vec<u8>`

// Another example

let v = core::ptr::null::<u8>(); // So here, `v` is a `*const u8`.
v as &u8; // error: non-primitive cast: `*const u8` as `&u8`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = 0u8;
x as u32; // ok!

let v = core::ptr::null::<u8>();
v as *const i8; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0605.html


---

## E0606 — An incompatible cast was attempted.

An incompatible cast was attempted. When casting, keep in mind that only primitive types can be cast into each other. Example: For more information about casts, take a look at the Type cast section in The Reference Book.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = &0u8; // Here, `x` is a `&u8`.
let y: u32 = x as u32; // error: casting `&u8` as `u32` is invalid
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = &0u8;
let y: u32 = *x as u32; // We dereference it first and then cast it.
}
```

出處：https://doc.rust-lang.org/error_codes/E0606.html


---

## E0607 — A cast between a thin and a wide pointer was attempted.

A cast between a thin and a wide pointer was attempted. First: what are thin and wide pointers? Thin pointers are “simple” pointers: they are purely a reference to a memory address. Wide pointers are pointers referencing Dynamically Sized Types (also called DSTs). DSTs don’t have a statically known size, therefore they can only exist behind some kind of pointer that contains additional information. For example, slices and trait objects are DSTs. In the case of slices, the additional information the wide pointer holds is their size. To fix this error, don’t try to cast directly between thin and wide pointers. For more information about type casts, take a look at the section of the The Rust Reference on type cast expressions.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let v = core::ptr::null::<u8>();
v as *const [u8];
}
```

出處：https://doc.rust-lang.org/error_codes/E0607.html


---

## E0608 — Attempted to index a value whose type doesn’t implement the
std::ops::Index trait.

Attempted to index a value whose type doesn’t implement the std::ops::Index trait. Only values with types that implement the std::ops::Index trait can be indexed with square brackets. Example: Tuples and structs are indexed with dot (.), not with brackets ([]), and tuple element names are their positions:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
0u8[2]; // error: cannot index into a value of type `u8`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let v: Vec<u8> = vec![0, 1, 2, 3];

// The `Vec` type implements the `Index` trait so you can do:
println!("{}", v[2]);
}
```

出處：https://doc.rust-lang.org/error_codes/E0608.html


---

## E0609 — Attempted to access a nonexistent field in a struct.

Attempted to access a nonexistent field in a struct. To fix this error, check that you didn’t misspell the field’s name or that the field actually exists. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct StructWithFields {
    x: u32,
}

let s = StructWithFields { x: 0 };
println!("{}", s.foo); // error: no field `foo` on type `StructWithFields`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct StructWithFields {
    x: u32,
}

let s = StructWithFields { x: 0 };
println!("{}", s.x); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0609.html


---

## E0610 — Attempted to access a field on a primitive type.

Attempted to access a field on a primitive type. Primitive types are the most basic types available in Rust and don’t have fields. To access data via named fields, struct types are used. Example: For more information about primitives and structs, take a look at the Book.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x: u32 = 0;
println!("{}", x.foo); // error: `{integer}` is a primitive type, therefore
                       //        doesn't have fields
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
// We declare struct called `Foo` containing two fields:
struct Foo {
    x: u32,
    y: i64,
}

// We create an instance of this struct:
let variable = Foo { x: 0, y: -12 };
// And we can now access its fields:
println!("x: {}, y: {}", variable.x, variable.y);
}
```

出處：https://doc.rust-lang.org/error_codes/E0610.html


---

## E0614 — Attempted to dereference a variable which cannot be dereferenced.

Attempted to dereference a variable which cannot be dereferenced. Only types implementing std::ops::Deref can be dereferenced (such as &T). Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let y = 0u32;
*y; // error: type `u32` cannot be dereferenced
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let y = 0u32;
let x = &y;
// So here, `x` is a `&u32`, so we can dereference it:
*x; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0614.html


---

## E0615 — Attempted to access a method like a field.

Attempted to access a method like a field. If you want to use a method, add () after it: However, if you wanted to access a field of a struct check that the field name is spelled correctly. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    x: u32,
}

impl Foo {
    fn method(&self) {}
}

let f = Foo { x: 0 };
f.method; // error: attempted to take value of method `method` on type `Foo`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo { x: u32 }
impl Foo { fn method(&self) {} }
let f = Foo { x: 0 };
f.method();
}
```

出處：https://doc.rust-lang.org/error_codes/E0615.html


---

## E0616 — Attempted to access a private field on a struct.

Attempted to access a private field on a struct. If you want to access this field, you have two options: Set the field public: Add a getter function:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod some_module {
    pub struct Foo {
        x: u32, // So `x` is private in here.
    }

    impl Foo {
        pub fn new() -> Foo { Foo { x: 0 } }
    }
}

let f = some_module::Foo::new();
println!("{}", f.x); // error: field `x` of struct `some_module::Foo` is private
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
mod some_module {
    pub struct Foo {
        pub x: u32, // `x` is now public.
    }

    impl Foo {
        pub fn new() -> Foo { Foo { x: 0 } }
    }
}

let f = some_module::Foo::new();
println!("{}", f.x); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0616.html


---

## E0617 — Attempted to pass an invalid type of variable into a variadic function.

Attempted to pass an invalid type of variable into a variadic function. Certain Rust types must be cast before passing them to a variadic function, because of arcane ABI rules dictated by the C standard. To fix the error, cast the value to the type specified by the error message (which you may need to import from std::os::raw). In this case, c_double has the same size as f64 so we can use it directly:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::os::raw::{c_char, c_int};
extern "C" {
    fn printf(format: *const c_char, ...) -> c_int;
}

unsafe {
    printf("%f\n\0".as_ptr() as _, 0f32);
    // error: cannot pass an `f32` to variadic function, cast to `c_double`
}
}
```

**正解**
```rust
# use std::os::raw::{c_char, c_int};
# extern "C" {
#     fn printf(format: *const c_char, ...) -> c_int;
# }

unsafe {
    printf("%f\n\0".as_ptr() as _, 0f64); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0617.html


---

## E0618 — Attempted to call something which isn’t a function nor a method.

Attempted to call something which isn’t a function nor a method. Erroneous code examples: Only functions and methods can be called using (). Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum X {
    Entry,
}

X::Entry(); // error: expected function, tuple struct or tuple variant,
            // found `X::Entry`

// Or even simpler:
let x = 0i32;
x(); // error: expected function, tuple struct or tuple variant, found `i32`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
// We declare a function:
fn i_am_a_function() {}

// And we call it:
i_am_a_function();
}
```

出處：https://doc.rust-lang.org/error_codes/E0618.html


---

## E0619 — The type-checker needed to know the type of an expression, but that type had not
yet been inferred.

Note: this error code is no longer emitted by the compiler. The type-checker needed to know the type of an expression, but that type had not yet been inferred. Type inference typically proceeds from the top of the function to the bottom, figuring out types as it goes. In some cases – notably method calls and overloadable operators like * – the type checker may not have enough information yet to make progress. This can be true even if the rest of the function provides enough context (because the type-checker hasn’t looked that far ahead yet). In this case, type annotations can be used to help it along. To fix this error, just specify the type of the variable. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let mut x = vec![];
match x.pop() {
    Some(v) => {
        // Here, the type of `v` is not (yet) known, so we
        // cannot resolve this method call:
        v.to_uppercase(); // error: the type of this value must be known in
                          //        this context
    }
    None => {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let mut x: Vec<String> = vec![]; // We precise the type of the vec elements.
match x.pop() {
    Some(v) => {
        v.to_uppercase(); // Since rustc now knows the type of the vec elements,
                          // we can use `v`'s methods.
    }
    None => {}
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0619.html


---

## E0620 — A cast to an unsized type was attempted.

A cast to an unsized type was attempted. In Rust, some types don’t have a known size at compile-time. For example, in a slice type like [u32], the number of elements is not known at compile-time and hence the overall size cannot be computed. As a result, such types can only be manipulated through a reference (e.g., &T or &mut T) or other pointer-type (e.g., Box or Rc). Try casting to a reference instead:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let x = &[1_usize, 2] as [usize]; // error: cast to unsized type: `&[usize; 2]`
                                  //        as `[usize]`
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let x = &[1_usize, 2] as &[usize]; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0620.html


---

## E0621 — This error code indicates a mismatch between the lifetimes appearing in the
function signature (i.e., the parameter types and the return type) and the
data-flow found in the function body.

This error code indicates a mismatch between the lifetimes appearing in the function signature (i.e., the parameter types and the return type) and the data-flow found in the function body. In the code above, the function is returning data borrowed from either x or y, but the 'a annotation indicates that it is returning data only from x. To fix the error, the signature and the body must be made to match. Typically, this is done by updating the function signature. So, in this case, we change the type of y to &'a i32, like so: Now the signature indicates that the function data borrowed from either x or y. Alternatively, you could change the body to not return data from y:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo<'a>(x: &'a i32, y: &i32) -> &'a i32 { // error: explicit lifetime
                                             //        required in the type of
                                             //        `y`
    if x > y { x } else { y }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo<'a>(x: &'a i32, y: &'a i32) -> &'a i32 {
    if x > y { x } else { y }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0621.html


---

## E0622 — An intrinsic was declared without being a function.

Note: this error code is no longer emitted by the compiler. An intrinsic was declared without being a function. An intrinsic is a function available for use in a given programming language whose implementation is handled specially by the compiler. In order to fix this error, just declare a function. Example:

**正解**
```rust
#![feature(intrinsics)]
#![allow(internal_features)]

extern "C" {
    #[rustc_intrinsic]
    pub static atomic_singlethreadfence_seqcst: unsafe fn();
    // error: intrinsic must be a function
}

fn main() { unsafe { atomic_singlethreadfence_seqcst(); } }
```

出處：https://doc.rust-lang.org/error_codes/E0622.html


---

## E0623 — A lifetime didn’t match what was expected.

A lifetime didn’t match what was expected. In this example, we tried to set a value with an incompatible lifetime to another one ('in_ is unrelated to 'out). We can solve this issue in two different ways: Either we make 'in_ live at least as long as 'out: Or we use only one lifetime:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a, 'b, T>(std::marker::PhantomData<(&'a (), &'b (), T)>)
where
    T: Convert<'a, 'b>;

trait Convert<'a, 'b>: Sized {
    fn cast(&'a self) -> &'b Self;
}
impl<'long: 'short, 'short, T> Convert<'long, 'short> for T {
    fn cast(&'long self) -> &'short T {
        self
    }
}
// error
fn badboi<'in_, 'out, T>(
    x: Foo<'in_, 'out, T>,
    sadness: &'in_ T
) -> &'out T {
    sadness.cast()
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo<'a, 'b, T>(std::marker::PhantomData<(&'a (), &'b (), T)>)
where
    T: Convert<'a, 'b>;

trait Convert<'a, 'b>: Sized {
    fn cast(&'a self) -> &'b Self;
}
impl<'long: 'short, 'short, T> Convert<'long, 'short> for T {
    fn cast(&'long self) -> &'short T {
        self
    }
}
fn badboi<'in_: 'out, 'out, T>(
    x: Foo<'in_, 'out, T>,
    sadness: &'in_ T
) -> &'out T {
    sadness.cast()
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0623.html


---

## E0624 — A private item was used outside of its scope.

A private item was used outside of its scope. Two possibilities are available to solve this issue: Only use the item in the scope it has been defined: Make the item public:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod inner {
    pub struct Foo;

    impl Foo {
        fn method(&self) {}
    }
}

let foo = inner::Foo;
foo.method(); // error: method `method` is private
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
mod inner {
    pub struct Foo;

    impl Foo {
        fn method(&self) {}
    }

    pub fn call_method(foo: &Foo) { // We create a public function.
        foo.method(); // Which calls the item.
    }
}

let foo = inner::Foo;
inner::call_method(&foo); // And since the function is public, we can call the
                          // method through it.
}
```

出處：https://doc.rust-lang.org/error_codes/E0624.html


---

## E0625 — A compile-time const variable is referring to a thread-local static variable.

A compile-time const variable is referring to a thread-local static variable. Static and const variables can refer to other const variables but a const variable cannot refer to a thread-local static variable. In this example, Y cannot refer to X. To fix this, the value can be extracted as a const and then used:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(thread_local)]

fn main() {
#[thread_local]
static X: usize = 12;

const Y: usize = 2 * X;
}
```

**正解**
```rust
#![allow(unused)]
#![feature(thread_local)]

fn main() {
const C: usize = 12;

#[thread_local]
static X: usize = C;

const Y: usize = 2 * C;
}
```

出處：https://doc.rust-lang.org/error_codes/E0625.html


---

## E0626 — This error occurs because a borrow in a movable coroutine persists across a
yield point.

This error occurs because a borrow in a movable coroutine persists across a yield point. Coroutines may be either unmarked, or marked with static. If it is unmarked, then the coroutine is considered “movable”. At present, it is not permitted to have a yield in a movable coroutine that occurs while a borrow is still in scope. To resolve this error, the coroutine may be marked static: If the coroutine must remain movable, for example to be used as Unpin without pinning it on the stack or in an allocation, we can alternatively resolve the previous example by removing the borrow and just storing the type by value: This is a very simple case, of course. In more complex cases, we may wish to have more than one reference to the value that was borrowed – in those cases, something like the Rc or Arc types may be useful. This error also frequently arises with iteration: Such cases can sometimes be resolved by iterating “by value” (or using into_iter()) to avoid borrowing: If taking ownership is not an option, using indices can work too:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#![feature(coroutines, coroutine_trait, stmt_expr_attributes)]
use std::ops::Coroutine;
use std::pin::Pin;
let mut b = #[coroutine] || {
    let a = &String::new(); // <-- This borrow...
    yield (); // ...is still in scope here, when the yield occurs.
    println!("{}", a);
};
Pin::new(&mut b).resume(());
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#![feature(coroutines, coroutine_trait, stmt_expr_attributes)]
use std::ops::Coroutine;
use std::pin::Pin;
let mut b = #[coroutine] static || { // <-- note the static keyword
    let a = &String::from("hello, world");
    yield ();
    println!("{}", a);
};
let mut b = std::pin::pin!(b);
b.as_mut().resume(());
}
```

出處：https://doc.rust-lang.org/error_codes/E0626.html


---

## E0627 — A yield expression was used outside of the coroutine literal.

A yield expression was used outside of the coroutine literal. The error occurs because keyword yield can only be used inside the coroutine literal. This can be fixed by constructing the coroutine correctly.

**錯誤範例**
```rust
#![feature(coroutines, coroutine_trait, stmt_expr_attributes)]

fn fake_coroutine() -> &'static str {
    yield 1;
    return "foo"
}

fn main() {
    let mut coroutine = fake_coroutine;
}
```

**正解**
```rust
#![feature(coroutines, coroutine_trait, stmt_expr_attributes)]

fn main() {
    let mut coroutine = #[coroutine] || {
        yield 1;
        return "foo"
    };
}
```

出處：https://doc.rust-lang.org/error_codes/E0627.html


---

## E0628 — More than one parameter was used for a coroutine.

More than one parameter was used for a coroutine. At present, it is not permitted to pass more than one explicit parameter for a coroutine.This can be fixed by using at most 1 parameter for the coroutine. For example, we might resolve the previous example by passing only one parameter.

**錯誤範例**
```rust
#![feature(coroutines, coroutine_trait, stmt_expr_attributes)]

fn main() {
    let coroutine = #[coroutine] |a: i32, b: i32| {
        // error: too many parameters for a coroutine
        // Allowed only 0 or 1 parameter
        yield a;
    };
}
```

**正解**
```rust
#![feature(coroutines, coroutine_trait, stmt_expr_attributes)]

fn main() {
    let coroutine = #[coroutine] |a: i32| {
        yield a;
    };
}
```

出處：https://doc.rust-lang.org/error_codes/E0628.html


---

## E0631 — This error indicates a type mismatch in closure arguments.

This error indicates a type mismatch in closure arguments. The error occurs because foo accepts a closure that takes an i32 argument, but in main, it is passed a closure with a &str argument. This can be resolved by changing the type annotation or removing it entirely if it can be inferred.

**錯誤範例**
```rust
fn foo<F: Fn(i32)>(f: F) {
}

fn main() {
    foo(|x: &str| {});
}
```

**正解**
```rust
fn foo<F: Fn(i32)>(f: F) {
}

fn main() {
    foo(|x: i32| {});
}
```

出處：https://doc.rust-lang.org/error_codes/E0631.html


---

## E0632 — An explicit generic argument was provided when calling a function that
uses impl Trait in argument position.

Note: this error code is no longer emitted by the compiler. An explicit generic argument was provided when calling a function that uses impl Trait in argument position. Either all generic arguments should be inferred at the call site, or the function definition should use an explicit generic type parameter instead of impl Trait. Example:

**正解**
```rust
fn foo<T: Copy>(a: T, b: impl Clone) {}

foo::<i32>(0i32, "abc".to_string());
```

出處：https://doc.rust-lang.org/error_codes/E0632.html


---

## E0633 — The unwind attribute was malformed.

Note: this error code is no longer emitted by the compiler. The unwind attribute was malformed. The #[unwind] attribute should be used as follows: #[unwind(aborts)] – specifies that if a non-Rust ABI function should abort the process if it attempts to unwind. This is the safer and preferred option. #[unwind(allowed)] – specifies that a non-Rust ABI function should be allowed to unwind. This can easily result in Undefined Behavior (UB), so be careful. NB. The default behavior here is “allowed”, but this is unspecified and likely to change in the future.

**錯誤範例**
```rust
#![feature(unwind_attributes)]

#[unwind()] // error: expected one argument
pub extern "C" fn something() {}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0633.html


---

## E0634 — A type has conflicting packed representation hints.

A type has conflicting packed representation hints. Erroneous code examples: You cannot use conflicting packed hints on a same type. If you want to pack a type to a given size, you should provide a size to packed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(packed, packed(2))] // error!
struct Company(i32);

#[repr(packed(2))] // error!
#[repr(packed)]
struct Company(i32);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(packed)] // ok!
struct Company(i32);
}
```

出處：https://doc.rust-lang.org/error_codes/E0634.html


---

## E0635 — The #![feature] attribute specified an unknown feature.

The #![feature] attribute specified an unknown feature.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(nonexistent_rust_feature)] // error: unknown feature
fn main() {
}
```

出處：https://doc.rust-lang.org/error_codes/E0635.html


---

## E0636 — The same feature is enabled multiple times with #![feature] attributes

Note: this error code is no longer emitted by the compiler. The same feature is enabled multiple times with #![feature] attributes

**錯誤範例**
```rust
#![allow(unused)]
#![allow(stable_features)]
#![feature(rust1)]
#![feature(rust1)] // error: the feature `rust1` has already been enabled
fn main() {
}
```

出處：https://doc.rust-lang.org/error_codes/E0636.html


---

## E0637 — '_ lifetime name or &T without an explicit lifetime name has been used
in an illegal place.

'_ lifetime name or &T without an explicit lifetime name has been used in an illegal place. First, '_ cannot be used as a lifetime identifier in some places because it is a reserved for the anonymous lifetime. Second, &T without an explicit lifetime name cannot also be used in some places. To fix them, use a lowercase letter such as 'a, or a series of lowercase letters such as 'foo. For more information about lifetime identifier, see the book. For more information on using the anonymous lifetime in Rust 2018, see the Rust 2018 blog post. Corrected example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn underscore_lifetime<'_>(str1: &'_ str, str2: &'_ str) -> &'_ str {
                     //^^ `'_` is a reserved lifetime name
    if str1.len() > str2.len() {
        str1
    } else {
        str2
    }
}

fn without_explicit_lifetime<T>()
where
    T: Iterator<Item = &u32>,
                     //^ `&` without an explicit lifetime name
{
}

fn without_hrtb<T>()
where
    T: Into<&u32>,
          //^ `&` without an explicit lifetime name
{
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn underscore_lifetime<'a>(str1: &'a str, str2: &'a str) -> &'a str {
    if str1.len() > str2.len() {
        str1
    } else {
        str2
    }
}

fn without_explicit_lifetime<'a, T>()
where
    T: Iterator<Item = &'a u32>,
{
}

fn without_hrtb<T>()
where
    T: for<'foo> Into<&'foo u32>,
{
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0637.html


---

## E0638 — This error indicates that the struct, enum or enum variant must be matched
non-exhaustively as it has been marked as non_exhaustive.

This error indicates that the struct, enum or enum variant must be matched non-exhaustively as it has been marked as non_exhaustive. When applied within a crate, downstream users of the crate will need to use the _ pattern when matching enums and use the .. pattern when matching structs. Downstream crates cannot match against non-exhaustive enum variants. For example, in the below example, since the enum is marked as non_exhaustive, it is required that downstream crates match non-exhaustively on it. An example of matching non-exhaustively on the above enum is provided below: Similarly, for structs, match with .. to avoid this error.

**正解**
```rust
#[non_exhaustive]
pub enum Error {
    Message(String),
    Other,
}

impl Display for Error {
    fn fmt(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        // This will not error, despite being marked as non_exhaustive, as this
        // enum is defined within the current crate, it can be matched
        // exhaustively.
        let display = match self {
            Message(s) => s,
            Other => "other or unknown error",
        };
        formatter.write_str(display)
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0638.html


---

## E0639 — This error indicates that the struct, enum or enum variant cannot be
instantiated from outside of the defining crate as it has been marked
as non_exhaustive and as such more fields/variants may be added in
future that could cause adverse side effects for this code.

This error indicates that the struct, enum or enum variant cannot be instantiated from outside of the defining crate as it has been marked as non_exhaustive and as such more fields/variants may be added in future that could cause adverse side effects for this code. It is recommended that you look for a new function or equivalent in the crate’s documentation.

**正解**
```rust
#[non_exhaustive]
pub struct NormalStruct {
    pub first_field: u16,
    pub second_field: u16,
}

let ns = NormalStruct { first_field: 640, second_field: 480 }; // error!
```

出處：https://doc.rust-lang.org/error_codes/E0639.html


---

## E0640 — This error code is internal to the compiler and will not be emitted with normal Rust code.

This error code is internal to the compiler and will not be emitted with normal Rust code. Note: this error code is no longer emitted by the compiler.

出處：https://doc.rust-lang.org/error_codes/E0640.html


---

## E0641 — Attempted to cast to/from a pointer with an unknown kind.

Attempted to cast to/from a pointer with an unknown kind. Type information must be provided if a pointer type being cast from/into another type which cannot be inferred:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let b = 0 as *const _; // error
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
// Creating a pointer from reference: type can be inferred
let a = &(String::from("Hello world!")) as *const _; // ok!

let b = 0 as *const i32; // ok!

let c: *const i32 = 0 as *const _; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0641.html


---

## E0642 — Trait methods currently cannot take patterns as arguments.

Trait methods currently cannot take patterns as arguments. You can instead use a single name for the argument:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo((x, y): (i32, i32)); // error: patterns aren't allowed
                                //        in trait methods
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo(x_and_y: (i32, i32)); // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0642.html


---

## E0643 — This error indicates that there is a mismatch between generic parameters and
impl Trait parameters in a trait declaration versus its impl.

This error indicates that there is a mismatch between generic parameters and impl Trait parameters in a trait declaration versus its impl.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    fn foo(&self, _: &impl Iterator);
}
impl Foo for () {
    fn foo<U: Iterator>(&self, _: &U) { } // error method `foo` has incompatible
                                          // signature for trait
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0643.html


---

## E0644 — A closure or generator was constructed that references its own type.

A closure or generator was constructed that references its own type. Rust does not permit a closure to directly reference its own type, either through an argument (as in the example above) or by capturing itself through its environment. This restriction helps keep closure inference tractable. The easiest fix is to rewrite your closure into a top-level function, or into a method. In some cases, you may also be able to have your closure call itself by capturing a &Fn() object or fn() pointer that refers to itself. That is permitting, since the closure would be invoking itself via a virtual call, and hence does not directly reference its own type.

**錯誤範例**
```rust
fn fix<F>(f: &F)
  where F: Fn(&F)
{
    f(&f);
}

fn main() {
    fix(&|y| {
        // Here, when `x` is called, the parameter `y` is equal to `x`.
    });
}
```

出處：https://doc.rust-lang.org/error_codes/E0644.html


---

## E0646 — It is not possible to define main with a where clause.

It is not possible to define main with a where clause.

**錯誤範例**
```rust
fn main() where i32: Copy { // error: main function is not allowed to have
                            // a where clause
}
```

出處：https://doc.rust-lang.org/error_codes/E0646.html


---

## E0647 — The start function was defined with a where clause.

Note: this error code is no longer emitted by the compiler. The start function was defined with a where clause.

出處：https://doc.rust-lang.org/error_codes/E0647.html


---

## E0648 — An export_name attribute contains null characters (\0).

An export_name attribute contains null characters (\0). To fix this error, remove the null characters:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[export_name="\0foo"] // error: `export_name` may not contain null characters
pub fn bar() {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[export_name="foo"] // ok!
pub fn bar() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0648.html


---

## E0657 — An impl Trait captured a higher-ranked lifetime, which is not supported.

An impl Trait captured a higher-ranked lifetime, which is not supported. Currently, impl Trait types are only allowed to capture lifetimes from their parent items, and not from any for<'a> binders in scope.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait BorrowInto<'a> {
    type Target;

    fn borrow_into(&'a self) -> Self::Target;
}

impl<'a> BorrowInto<'a> for () {
    type Target = &'a ();

    fn borrow_into(&'a self) -> Self::Target {
        self
    }
}

fn opaque() -> impl for<'a> BorrowInto<'a, Target = impl Sized + 'a> {
    ()
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0657.html


---

## E0658 — An unstable feature was used.

An unstable feature was used. If you’re using a stable or a beta version of rustc, you won’t be able to use any unstable features. In order to do so, please switch to a nightly version of rustc (by using rustup). If you’re using a nightly version of rustc, just add the corresponding feature to be able to use it:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::intrinsics; // error: use of unstable library feature `core_intrinsics`
}
```

**正解**
```rust
#![allow(unused)]
#![feature(core_intrinsics)]

fn main() {
use std::intrinsics; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0658.html


---

## E0659 — An item usage is ambiguous.

An item usage is ambiguous. This error generally appears when two items with the same name are imported into a module. Here, the foo functions are imported and reexported from the collider module and therefore, when we’re using collider::foo(), both functions collide. To solve this error, the best solution is generally to keep the path before the item when using it. Example:

**錯誤範例**
```rust
pub mod moon {
    pub fn foo() {}
}

pub mod earth {
    pub fn foo() {}
}

mod collider {
    pub use crate::moon::*;
    pub use crate::earth::*;
}

fn main() {
    crate::collider::foo(); // ERROR: `foo` is ambiguous
}
```

**正解**
```rust
pub mod moon {
    pub fn foo() {}
}

pub mod earth {
    pub fn foo() {}
}

mod collider {
    pub use crate::moon;
    pub use crate::earth;
}

fn main() {
    crate::collider::moon::foo(); // ok!
    crate::collider::earth::foo(); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0659.html


---

## E0660 — The argument to the llvm_asm macro is not well-formed.

Note: this error code is no longer emitted by the compiler. The argument to the llvm_asm macro is not well-formed.

**正解**
```rust
llvm_asm!("nop" "nop");
```

出處：https://doc.rust-lang.org/error_codes/E0660.html


---

## E0661 — An invalid syntax was passed to the second argument of an llvm_asm macro line.

Note: this error code is no longer emitted by the compiler. An invalid syntax was passed to the second argument of an llvm_asm macro line.

**正解**
```rust
let a;
llvm_asm!("nop" : "r"(a));
```

出處：https://doc.rust-lang.org/error_codes/E0661.html


---

## E0662 — An invalid input operand constraint was passed to the llvm_asm macro
(third line).

Note: this error code is no longer emitted by the compiler. An invalid input operand constraint was passed to the llvm_asm macro (third line).

**正解**
```rust
llvm_asm!("xor %eax, %eax"
          :
          : "=test"("a")
         );
```

出處：https://doc.rust-lang.org/error_codes/E0662.html


---

## E0663 — An invalid input operand constraint was passed to the llvm_asm macro
(third line).

Note: this error code is no longer emitted by the compiler. An invalid input operand constraint was passed to the llvm_asm macro (third line).

**正解**
```rust
llvm_asm!("xor %eax, %eax"
          :
          : "+test"("a")
         );
```

出處：https://doc.rust-lang.org/error_codes/E0663.html


---

## E0664 — A clobber was surrounded by braces in the llvm_asm macro.

Note: this error code is no longer emitted by the compiler. A clobber was surrounded by braces in the llvm_asm macro.

**正解**
```rust
llvm_asm!("mov $$0x200, %eax"
          :
          :
          : "{eax}"
         );
```

出處：https://doc.rust-lang.org/error_codes/E0664.html


---

## E0665 — The Default trait was derived on an enum without specifying the default
variant.

The Default trait was derived on an enum without specifying the default variant. The Default cannot be derived on an enum for the simple reason that the compiler doesn’t know which value to pick by default whereas it can for a struct as long as all its fields implement the Default trait as well. For the case where the desired default variant has no payload, you can annotate it with #[default] to derive it: In the case where the default variant does have a payload, you will have to implement Default on your enum manually:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[derive(Default)]
enum Food {
    Sweet,
    Salty,
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[derive(Default)]
enum Food {
    #[default]
    Sweet,
    Salty,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0665.html


---

## E0666 — impl Trait types cannot appear nested in the generic arguments of other
impl Trait types.

impl Trait types cannot appear nested in the generic arguments of other impl Trait types. Type parameters for impl Trait types must be explicitly defined as named generic parameters:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait MyGenericTrait<T> {}
trait MyInnerTrait {}

fn foo(
    bar: impl MyGenericTrait<impl MyInnerTrait>, // error!
) {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait MyGenericTrait<T> {}
trait MyInnerTrait {}

fn foo<T: MyInnerTrait>(
    bar: impl MyGenericTrait<T>, // ok!
) {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0666.html


---

## E0667 — impl Trait is not allowed in path parameters.

Note: this error code is no longer emitted by the compiler. impl Trait is not allowed in path parameters. You cannot use impl Trait in path parameters. If you want something equivalent, you can do this instead:

**正解**
```rust
fn some_fn(mut x: impl Iterator) -> <impl Iterator>::Item { // error!
    x.next().unwrap()
}
```

出處：https://doc.rust-lang.org/error_codes/E0667.html


---

## E0668 — Malformed inline assembly rejected by LLVM.

Note: this error code is no longer emitted by the compiler. Malformed inline assembly rejected by LLVM. LLVM checks the validity of the constraints and the assembly string passed to it. This error implies that LLVM seems something wrong with the inline assembly call. In particular, it can happen if you forgot the closing bracket of a register constraint (see issue #51430), like in the previous code example.

**正解**
```rust
#![feature(llvm_asm)]

fn main() {
    let rax: u64;
    unsafe {
        llvm_asm!("" :"={rax"(rax));
        println!("Accumulator is: {}", rax);
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0668.html


---

## E0669 — Cannot convert inline assembly operand to a single LLVM value.

Note: this error code is no longer emitted by the compiler. Cannot convert inline assembly operand to a single LLVM value. This error usually happens when trying to pass in a value to an input inline assembly operand that is actually a pair of values. In particular, this can happen when trying to pass in a slice, for instance a &str. In Rust, these values are represented internally as a pair of values, the pointer and its length. When passed as an input operand, this pair of values can not be coerced into a register and thus we must fail with an error.

**正解**
```rust
#![feature(llvm_asm)]

fn main() {
    unsafe {
        llvm_asm!("" :: "r"("")); // error!
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0669.html


---

## E0670 — Rust 2015 does not permit the use of async fn.

Rust 2015 does not permit the use of async fn. Switch to the Rust 2018 edition to use async fn.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
async fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0670.html


---

## E0671 — Const parameters cannot depend on type parameters.

Note: this error code is no longer emitted by the compiler. Const parameters cannot depend on type parameters. The following is therefore invalid:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn const_id<T, const N: T>() -> T { // error
    N
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0671.html


---

## E0687 — In-band lifetimes cannot be used in fn/Fn syntax.

Note: this error code is no longer emitted by the compiler. In-band lifetimes cannot be used in fn/Fn syntax. Erroneous code examples: Lifetimes used in fn or Fn syntax must be explicitly declared using <...> binders. For example:

**正解**
```rust
#![feature(in_band_lifetimes)]

fn foo(x: fn(&'a u32)) {} // error!

fn bar(x: &Fn(&'a u32)) {} // error!

fn baz(x: fn(&'a u32), y: &'a u32) {} // error!

struct Foo<'a> { x: &'a u32 }

impl Foo<'a> {
    fn bar(&self, x: fn(&'a u32)) {} // error!
}
```

出處：https://doc.rust-lang.org/error_codes/E0687.html


---

## E0688 — In-band lifetimes were mixed with explicit lifetime binders.

Note: this error code is no longer emitted by the compiler. In-band lifetimes were mixed with explicit lifetime binders. In-band lifetimes cannot be mixed with explicit lifetime binders. For example:

**正解**
```rust
#![feature(in_band_lifetimes)]

fn foo<'a>(x: &'a u32, y: &'b u32) {}   // error!

struct Foo<'a> { x: &'a u32 }

impl Foo<'a> {
    fn bar<'b>(x: &'a u32, y: &'b u32, z: &'c u32) {}   // error!
}

impl<'b> Foo<'a> {  // error!
    fn baz() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0688.html


---

## E0689 — A method was called on an ambiguous numeric type.

A method was called on an ambiguous numeric type. This error indicates that the numeric value for the method being passed exists but the type of the numeric value or binding could not be identified. The error happens on numeric literals and on numeric bindings without an identified concrete type: Because of this, you must give the numeric literal or binding a type:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
2.0.neg(); // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::ops::Neg;

let _ = 2.0_f32.neg(); // ok!
let x: f32 = 2.0;
let _ = x.neg(); // ok!
let _ = (2.0 as f32).neg(); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0689.html


---

## E0690 — A struct with the representation hint repr(transparent) had two or more fields
that were not guaranteed to be zero-sized.

A struct with the representation hint repr(transparent) had two or more fields that were not guaranteed to be zero-sized. Because transparent structs are represented exactly like one of their fields at run time, said field must be uniquely determined. If there are multiple fields, it is not clear how the struct should be represented. Note that fields of zero-sized types (e.g., PhantomData) can also exist alongside the field that contains the actual data, they do not count for this error. When generic types are involved (as in the above example), an error is reported because the type parameter could be non-zero-sized. To combine repr(transparent) with type parameters, PhantomData may be useful:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(transparent)]
struct LengthWithUnit<U> { // error: transparent struct needs at most one
    value: f32,            //        non-zero-sized field, but has 2
    unit: U,
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::marker::PhantomData;

#[repr(transparent)]
struct LengthWithUnit<U> {
    value: f32,
    unit: PhantomData<U>,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0690.html


---

## E0691 — A struct, enum, or union with the repr(transparent) representation hint
contains a zero-sized field that requires non-trivial alignment.

Note: this error code is no longer emitted by the compiler. A struct, enum, or union with the repr(transparent) representation hint contains a zero-sized field that requires non-trivial alignment. A transparent struct, enum, or union is supposed to be represented exactly like the piece of data it contains. Zero-sized fields with different alignment requirements potentially conflict with this property. In the example above, Wrapper would have to be aligned to 32 bytes even though f32 has a smaller alignment requirement. Consider removing the over-aligned zero-sized field: Alternatively, PhantomData<T> has alignment 1 for all T, so you can use it if you need to keep the field for some reason: Note that empty arrays [T; 0] have the same alignment requirement as the element type T. Also note that the error is conservatively reported even when the alignment of the zero-sized type is less than or equal to the data field’s alignment.

**正解**
```rust
#![feature(repr_align)]

#[repr(align(32))]
struct ForceAlign32;

#[repr(transparent)]
struct Wrapper(f32, ForceAlign32); // error: zero-sized field in transparent
                                   //        struct has alignment of 32, which
                                   //        is larger than 1
```

出處：https://doc.rust-lang.org/error_codes/E0691.html


---

## E0692 — A repr(transparent) type was also annotated with other, incompatible
representation hints.

A repr(transparent) type was also annotated with other, incompatible representation hints. A type annotated as repr(transparent) delegates all representation concerns to another type, so adding more representation hints is contradictory. Remove either the transparent hint or the other hints, like this: Alternatively, move the other attributes to the contained type: Note that introducing another struct just to have a place for the other attributes may have unintended side effects on the representation: Here, Grams2 is a not equivalent to Grams – the former transparently wraps a (non-transparent) struct containing a single float, while Grams is a transparent wrapper around a float. This can make a difference for the ABI.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(transparent, C)] // error: incompatible representation hints
struct Grams(f32);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(transparent)]
struct Grams(f32);
}
```

出處：https://doc.rust-lang.org/error_codes/E0692.html


---

## E0693 — This error code was replaced by E0539.

Note: this error code is no longer emitted by the compiler. This error code was replaced by E0539. align representation hint was incorrectly declared. Erroneous code examples: This is a syntax error at the level of attribute declarations. The proper syntax for align representation hint is the following:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(align=8)] // error!
struct Align8(i8);

#[repr(align="8")] // error!
struct Align8(i8);
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(align(8))] // ok!
struct Align8(i8);
}
```

出處：https://doc.rust-lang.org/error_codes/E0693.html


---

## E0695 — A break statement without a label appeared inside a labeled block.

A break statement without a label appeared inside a labeled block. Make sure to always label the break: Or if you want to break the labeled block:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
loop {
    'a: {
        break;
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
'l: loop {
    'a: {
        break 'l;
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0695.html


---

## E0696 — A function is using continue keyword incorrectly.

A function is using continue keyword incorrectly. Here we have used the continue keyword incorrectly. As we have seen above that continue pointing to a labeled block. To fix this we have to use the labeled block properly. For example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn continue_simple() {
    'b: {
        continue; // error!
    }
}
fn continue_labeled() {
    'b: {
        continue 'b; // error!
    }
}
fn continue_crossing() {
    loop {
        'b: {
            continue; // error!
        }
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn continue_simple() {
    'b: loop {
        continue ; // ok!
    }
}
fn continue_labeled() {
    'b: loop {
        continue 'b; // ok!
    }
}
fn continue_crossing() {
    loop {
        'b: loop {
            continue; // ok!
        }
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0696.html


---

## E0697 — A closure has been used as static.

A closure has been used as static. Closures cannot be used as static. They “save” the environment, and as such a static closure would save only a static environment which would consist only of variables with a static lifetime. Given this it would be better to use a proper function. The easiest fix is to remove the static keyword.

**錯誤範例**
```rust
fn main() {
    static || {}; // used as `static`
}
```

出處：https://doc.rust-lang.org/error_codes/E0697.html


---

## E0698 — When using coroutines (or async) all type variables must be bound so a
coroutine can be constructed.

Note: this error code is no longer emitted by the compiler. When using coroutines (or async) all type variables must be bound so a coroutine can be constructed. In the above example T is unknowable by the compiler. To fix this you must bind T to a concrete type such as String so that a coroutine can then be constructed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
async fn bar<T>() -> () {}

async fn foo() {
    bar().await; // error: cannot infer type for `T`
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
async fn bar<T>() -> () {}

async fn foo() {
    bar::<String>().await;
    //   ^^^^^^^^ specify type explicitly
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0698.html


---

## E0699 — A method was called on a raw pointer whose inner type wasn’t completely known.

Note: this error code is no longer emitted by the compiler. A method was called on a raw pointer whose inner type wasn’t completely known. Here, the type of bar isn’t known; it could be a pointer to anything. Instead, specify a type for the pointer (preferably something that makes sense for the thing you’re pointing to): Even though is_null() exists as a method on any raw pointer, Rust shows this error because Rust allows for self to have arbitrary types (behind the arbitrary_self_types feature flag). This means that someone can specify such a function: and now when you call .is_null() on a raw pointer to Foo, there’s ambiguity. Given that we don’t know what type the pointer is, and there’s potential ambiguity for some types, we disallow calling methods on raw pointers when the type is unknown.

**錯誤範例**
```rust
#![deny(warnings)]
fn main() {
let foo = &1;
let bar = foo as *const _;
if bar.is_null() {
    // ...
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let foo = &1;
let bar = foo as *const i32;
if bar.is_null() {
    // ...
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0699.html


---

## E0700 — The impl Trait return type captures lifetime parameters that do not
appear within the impl Trait itself.

The impl Trait return type captures lifetime parameters that do not appear within the impl Trait itself. Here, the function foo returns a value of type Cell<&'x u32>, which references the lifetime 'x. However, the return type is declared as impl Trait<'y> – this indicates that foo returns “some type that implements Trait<'y>”, but it also indicates that the return type only captures data referencing the lifetime 'y. In this case, though, we are referencing data with lifetime 'x, so this function is in error. To fix this, you must reference the lifetime 'x from the return type. For example, changing the return type to impl Trait<'y> + 'x would work:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::cell::Cell;

trait Trait<'a> { }

impl<'a, 'b> Trait<'b> for Cell<&'a u32> { }

fn foo<'x, 'y>(x: Cell<&'x u32>) -> impl Trait<'y>
where 'x: 'y
{
    x
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::cell::Cell;

trait Trait<'a> { }

impl<'a,'b> Trait<'b> for Cell<&'a u32> { }

fn foo<'x, 'y>(x: Cell<&'x u32>) -> impl Trait<'y> + 'x
where 'x: 'y
{
    x
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0700.html


---

## E0701 — This error indicates that a #[non_exhaustive] attribute was incorrectly placed
on something other than a struct or enum.

Note: this error code is no longer emitted by the compiler. This error indicates that a #[non_exhaustive] attribute was incorrectly placed on something other than a struct or enum.

**正解**
```rust
#[non_exhaustive]
trait Foo { }
```

出處：https://doc.rust-lang.org/error_codes/E0701.html


---

## E0703 — Invalid ABI (Application Binary Interface) used in the code.

Invalid ABI (Application Binary Interface) used in the code. At present few predefined ABI’s (like Rust, C, system, etc.) can be used in Rust. Verify that the ABI is predefined. For example you can replace the given ABI from ‘Rust’.

**錯誤範例**
```rust
extern "invalid" fn foo() {} // error!
fn main() {}
```

**正解**
```rust
extern "Rust" fn foo() {} // ok!
fn main() { }
```

出處：https://doc.rust-lang.org/error_codes/E0703.html


---

## E0704 — An incorrect visibility restriction was specified.

An incorrect visibility restriction was specified. To make struct Bar only visible in module foo the in keyword should be used: For more information see the Rust Reference on Visibility.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
mod foo {
    pub(foo) struct Bar {
        x: i32
    }
}
}
```

**正解**
```rust
mod foo {
    pub(in crate::foo) struct Bar {
        x: i32
    }
}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0704.html


---

## E0705 — A #![feature] attribute was used for a feature that is stable in the
current edition, but not in all editions.

Note: this error code is no longer emitted by the compiler. A #![feature] attribute was used for a feature that is stable in the current edition, but not in all editions.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(rust_2018_preview)]
#![feature(test_2018_feature)] // error: the feature
                               fn main() {
// `test_2018_feature` is
                               // included in the Rust 2018 edition
}
```

出處：https://doc.rust-lang.org/error_codes/E0705.html


---

## E0706 — async fns are not yet supported in traits in Rust.

Note: this error code is no longer emitted by the compiler. async fns are not yet supported in traits in Rust. async fns return an impl Future, making the following two examples equivalent: But when it comes to supporting this in traits, there are a few implementation issues. One of them is returning impl Trait in traits is not supported, as it would require Generic Associated Types to be supported: Until these issues are resolved, you can use the async-trait crate, allowing you to use async fn in traits by desugaring to “boxed futures” (Pin<Box<dyn Future + Send + 'async>>). Note that using these trait methods will result in a heap allocation per-function-call. This is not a significant cost for the vast majority of applications, but should be considered when deciding whether to use this functionality in the public API of a low-level function that is expected to be called millions of times a second. You might be interested in visiting the async book for further information.

**正解**
```rust
trait T {
    // Neither case is currently supported.
    async fn foo() {}
    async fn bar(&self) {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0706.html


---

## E0708 — async non-move closures with parameters are currently not supported.

Note: this error code is no longer emitted by the compiler. async non-move closures with parameters are currently not supported. async with non-move is currently not supported with the current version, you can use successfully by using move:

**正解**
```rust
fn main() {
    let add_one = async |num: u8| {
        num + 1
    };
}
```

出處：https://doc.rust-lang.org/error_codes/E0708.html


---

## E0710 — An unknown tool name was found in a scoped lint.

An unknown tool name was found in a scoped lint. Erroneous code examples: Please verify you didn’t misspell the tool’s name or that you didn’t forget to import it in you project:

**錯誤範例**
```rust
#[allow(clipp::filter_map)] // error!
fn main() {
    // business logic
}
```

**正解**
```rust
#[allow(clippy::filter_map)] // ok!
fn main() {
    // business logic
}
```

出處：https://doc.rust-lang.org/error_codes/E0710.html


---

## E0711 — This error code is internal to the compiler and will not be emitted with normal Rust code.

This error code is internal to the compiler and will not be emitted with normal Rust code. Feature declared with conflicting stability requirements. In the above example, the foo feature is first defined to be stable since 1.0.0, but is then re-declared stable since 1.29.0. This discrepancy in versions causes an error. Furthermore, foo is then re-declared as unstable, again the conflict causes an error. This error can be fixed by splitting the feature, this allows any stability requirements and removes any possibility of conflict.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// NOTE: this attribute is perma-unstable and should *never* be used outside of
//       stdlib and the compiler.
#![feature(staged_api)]

#![stable(feature = "...", since = "1.0.0")]

#[stable(feature = "foo", since = "1.0.0")]
fn foo_stable_1_0_0() {}

// error: feature `foo` is declared stable since 1.29.0
#[stable(feature = "foo", since = "1.29.0")]
fn foo_stable_1_29_0() {}

// error: feature `foo` is declared unstable
#[unstable(feature = "foo", issue = "none")]
fn foo_unstable() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0711.html


---

## E0712 — A borrow of a thread-local variable was made inside a function which outlived
the lifetime of the function.

A borrow of a thread-local variable was made inside a function which outlived the lifetime of the function.

**錯誤範例**
```rust
#![feature(thread_local)]

#[thread_local]
static FOO: u8 = 3;

fn main() {
    let a = &FOO; // error: thread-local variable borrowed past end of function

    std::thread::spawn(move || {
        println!("{}", a);
    });
}
```

出處：https://doc.rust-lang.org/error_codes/E0712.html


---

## E0713 — This error occurs when an attempt is made to borrow state past the end of the
lifetime of a type that implements the Drop trait.

This error occurs when an attempt is made to borrow state past the end of the lifetime of a type that implements the Drop trait. Here, demo tries to borrow the string data held within its argument s and then return that borrow. However, S is declared as implementing Drop. Structs implementing the Drop trait have an implicit destructor that gets called when they go out of scope. This destructor gets exclusive access to the fields of the struct when it runs. This means that when s reaches the end of demo, its destructor gets exclusive access to its &mut-borrowed string data. allowing another borrow of that string data (p), to exist across the drop of s would be a violation of the principle that &mut-borrows have exclusive, unaliased access to their referenced data. This error can be fixed by changing demo so that the destructor does not run while the string-data is borrowed; for example by taking S by reference: Note that this approach needs a reference to S with lifetime 'a. Nothing shorter than 'a will suffice: a shorter lifetime would imply that after demo finishes executing, something else (such as the destructor!) could access s.data after the end of that shorter lifetime, which would again violate the &mut-borrow’s exclusive access.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
pub struct S<'a> { data: &'a mut String }

impl<'a> Drop for S<'a> {
    fn drop(&mut self) { self.data.push_str("being dropped"); }
}

fn demo<'a>(s: S<'a>) -> &'a mut String { let p = &mut *s.data; p }
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
pub struct S<'a> { data: &'a mut String }

impl<'a> Drop for S<'a> {
    fn drop(&mut self) { self.data.push_str("being dropped"); }
}

fn demo<'a>(s: &'a mut S<'a>) -> &'a mut String { let p = &mut *(*s).data; p }
}
```

出處：https://doc.rust-lang.org/error_codes/E0713.html


---

## E0714 — A #[marker] trait contained an associated item.

A #[marker] trait contained an associated item. The items of marker traits cannot be overridden, so there’s no need to have them when they cannot be changed per-type anyway. If you wanted them for ergonomic reasons, consider making an extension trait instead.

**錯誤範例**
```rust
#![feature(marker_trait_attr)]
#![feature(associated_type_defaults)]

#[marker]
trait MarkerConst {
    const N: usize; // error!
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0714.html


---

## E0715 — An impl for a #[marker] trait tried to override an associated item.

An impl for a #[marker] trait tried to override an associated item. Because marker traits are allowed to have multiple implementations for the same type, it’s not allowed to override anything in those implementations, as it would be ambiguous which override should actually be used.

**錯誤範例**
```rust
#![feature(marker_trait_attr)]

#[marker]
trait Marker {
    const N: usize = 0;
    fn do_something() {}
}

struct OverrideConst;
impl Marker for OverrideConst { // error!
    const N: usize = 1;
}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0715.html


---

## E0716 — A temporary value is being dropped while a borrow is still in active use.

A temporary value is being dropped while a borrow is still in active use. Here, the expression &foo() is borrowing the expression foo(). As foo() is a call to a function, and not the name of a variable, this creates a temporary – that temporary stores the return value from foo() so that it can be borrowed. You could imagine that let p = bar(&foo()); is equivalent to the following, which uses an explicit temporary variable. Whenever a temporary is created, it is automatically dropped (freed) according to fixed rules. Ordinarily, the temporary is dropped at the end of the enclosing statement – in this case, after the let p. This is illustrated in the example above by showing that tmp would be freed as we exit the block. To fix this problem, you need to create a local variable to store the value in rather than relying on a temporary. For example, you might change the original program to the following: By introducing the explicit let value, we allocate storage that will last until the end of the enclosing block (when value goes out of scope). When we borrow &value, we are borrowing a local variable that already exists, and hence no temporary is created. Temporaries are not always dropped at the end of the enclosing statement. In simple cases where the & expression is immediately stored into a variable, the compiler will automatically extend the lifetime of the temporary until the end of the enclosing block. Therefore, an alternative way to fix the original program is to write let tmp = &foo() and not let tmp = foo(): Here, we are still borrowing foo(), but as the borrow is assigned directly into a variable, the temporary will not be dropped until the end of the enclosing block. Similar rules apply when temporaries are stored into aggregate structures like a tuple or struct:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo() -> i32 { 22 }
fn bar(x: &i32) -> &i32 { x }
let p = bar(&foo());
         // ------ creates a temporary
let q = *p;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo() -> i32 { 22 }
fn bar(x: &i32) -> &i32 { x }
let value = foo(); // dropped at the end of the enclosing block
let p = bar(&value);
let q = *p;
}
```

出處：https://doc.rust-lang.org/error_codes/E0716.html


---

## E0717 — This error code is internal to the compiler and will not be emitted with normal Rust code.

This error code is internal to the compiler and will not be emitted with normal Rust code.

出處：https://doc.rust-lang.org/error_codes/E0717.html


---

## E0718 — A #[lang = ".."] attribute was placed on the wrong item type.

Note: this error code is no longer emitted by the compiler. A #[lang = ".."] attribute was placed on the wrong item type.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(lang_items)]

fn main() {
#[lang = "owned_box"]
static X: u32 = 42;
}
```

出處：https://doc.rust-lang.org/error_codes/E0718.html


---

## E0719 — An associated item was specified more than once in a trait object.

An associated item was specified more than once in a trait object. To fix this, remove the duplicate specifier: Corrected example: For more information about associated types, see the book. For more information on associated type bounds, see RFC 2289.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait FooTrait {}
trait BarTrait {}

// error: associated type `Item` in trait `Iterator` is specified twice
type Foo = dyn Iterator<Item = u32, Item = u32>;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
type Foo = dyn Iterator<Item = u32>; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0719.html


---

## E0720 — An impl Trait type expands to a recursive type.

An impl Trait type expands to a recursive type. An impl Trait type must be expandable to a concrete type that contains no impl Trait types. For example the previous example tries to create an impl Trait type T that is equal to [T, T].

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn make_recursive_type() -> impl Sized {
    [make_recursive_type(), make_recursive_type()]
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0720.html


---

## E0722 — Note: this error code is no longer emitted by the compiler
This is because it was too specific to the optimize attribute.

Note: this error code is no longer emitted by the compiler This is because it was too specific to the optimize attribute. Similar diagnostics occur for other attributes too. The example here will now emit E0539 The optimize attribute was malformed. The #[optimize] attribute should be used as follows: #[optimize(size)] – instructs the optimization pipeline to generate code that’s smaller rather than faster #[optimize(speed)] – instructs the optimization pipeline to generate code that’s faster rather than smaller For example: See RFC 2412 for more details.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(optimize_attribute)]

fn main() {
#[optimize(something)] // error: invalid argument
pub fn something() {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(optimize_attribute)]

fn main() {
#[optimize(size)]
pub fn something() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0722.html


---

## E0724 — #[ffi_returns_twice] was used on something other than a foreign function
declaration.

Note: this error code is no longer emitted by the compiler. #[ffi_returns_twice] was used on something other than a foreign function declaration. #[ffi_returns_twice] can only be used on foreign function declarations. For example, we might correct the previous example by declaring the function inside of an extern block.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(ffi_returns_twice)]
#![crate_type = "lib"]

fn main() {
#[ffi_returns_twice] // error!
pub fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0724.html


---

## E0725 — A feature attribute named a feature that was disallowed in the compiler
command line flags.

A feature attribute named a feature that was disallowed in the compiler command line flags. Delete the offending feature attribute, or add it to the list of allowed features in the -Z allow_features flag.

**正解**
```rust
#![feature(never_type)] // error: the feature `never_type` is not in
                        // the list of allowed features
```

出處：https://doc.rust-lang.org/error_codes/E0725.html


---

## E0726 — An argument lifetime was elided in an async function.

An argument lifetime was elided in an async function. When a struct or a type is bound/declared with a lifetime it is important for the Rust compiler to know, on usage, the lifespan of the type. When the lifetime is not explicitly mentioned and the Rust Compiler cannot determine the lifetime of your type, the following error occurs. Specify desired lifetime of parameter content or indicate the anonymous lifetime like content: Content<'_>. The anonymous lifetime tells the Rust compiler that content is only needed until the create function is done with its execution. The implicit elision meaning the omission of suggested lifetime that is pub async fn create<'a>(content: Content<'a>) {} is not allowed here as lifetime of the content can differ from current context: Know more about lifetime elision in this chapter and a chapter on lifetimes can be found here.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use futures::executor::block_on;
struct Content<'a> {
    title: &'a str,
    body: &'a str,
}
async fn create(content: Content) { // error: implicit elided
                                    // lifetime not allowed here
    println!("title: {}", content.title);
    println!("body: {}", content.body);
}
let content = Content { title: "Rust", body: "is great!" };
let future = create(content);
block_on(future);
}
```

**正解**
```rust
async fn create(content: Content<'_>) { // ok!
    println!("title: {}", content.title);
    println!("body: {}", content.body);
}
```

出處：https://doc.rust-lang.org/error_codes/E0726.html


---

## E0727 — A yield clause was used in an async context.

A yield clause was used in an async context. Here, the yield keyword is used in an async block, which is not yet supported. To fix this error, you have to move yield out of the async block:

**錯誤範例**
```rust
#![feature(coroutines, stmt_expr_attributes)]

fn main() {
    let coroutine = #[coroutine] || {
        async {
            yield;
        }
    };
}
```

**正解**
```rust
#![feature(coroutines, stmt_expr_attributes)]

fn main() {
    let coroutine = #[coroutine] || {
        yield;
    };
}
```

出處：https://doc.rust-lang.org/error_codes/E0727.html


---

## E0728 — await has been used outside async function or async block.

await has been used outside async function or async block. await is used to suspend the current computation until the given future is ready to produce a value. So it is legal only within an async context, like an async function or an async block.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::pin::Pin;
use std::future::Future;
use std::task::{Context, Poll};

struct WakeOnceThenComplete(bool);

fn wake_and_yield_once() -> WakeOnceThenComplete {
    WakeOnceThenComplete(false)
}

impl Future for WakeOnceThenComplete {
    type Output = ();
    fn poll(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<()> {
        if self.0 {
            Poll::Ready(())
        } else {
            cx.waker().wake_by_ref();
            self.0 = true;
            Poll::Pending
        }
    }
}

fn foo() {
    wake_and_yield_once().await // `await` is used outside `async` context
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::pin::Pin;
use std::future::Future;
use std::task::{Context, Poll};

struct WakeOnceThenComplete(bool);

fn wake_and_yield_once() -> WakeOnceThenComplete {
    WakeOnceThenComplete(false)
}

impl Future for WakeOnceThenComplete {
    type Output = ();
    fn poll(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<()> {
        if self.0 {
            Poll::Ready(())
        } else {
            cx.waker().wake_by_ref();
            self.0 = true;
            Poll::Pending
        }
    }
}

async fn foo() {
    wake_and_yield_once().await // `await` is used within `async` function
}

fn bar(x: u8) -> impl Future<Output = u8> {
    async move {
        wake_and_yield_once().await; // `await` is used within `async` block
        x
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0728.html


---

## E0729 — Note: this error code is no longer emitted by the compiler
Support for Non-Lexical Lifetimes (NLL) has been included in the Rust compiler
since 1.31, and has been enabled on the 2015 edition since 1.36.

Note: this error code is no longer emitted by the compiler Support for Non-Lexical Lifetimes (NLL) has been included in the Rust compiler since 1.31, and has been enabled on the 2015 edition since 1.36. The new borrow checker for NLL uncovered some bugs in the old borrow checker, which in some cases allowed unsound code to compile, resulting in memory safety issues. What do I do? Change your code so the warning does no longer trigger. For backwards compatibility, this unsound code may still compile (with a warning) right now. However, at some point in the future, the compiler will no longer accept this code and will throw a hard error. Shouldn’t you fix the old borrow checker? The old borrow checker has known soundness issues that are basically impossible to fix. The new NLL-based borrow checker is the fix. Can I turn these warnings into errors by denying a lint? No. When are these warnings going to turn into errors? No formal timeline for turning the warnings into errors has been set. See GitHub issue 58781 for more information. Why do I get this message with code that doesn’t involve borrowing? There are some known bugs that trigger this message.

出處：https://doc.rust-lang.org/error_codes/E0729.html


---

## E0730 — An array without a fixed length was pattern-matched.

An array without a fixed length was pattern-matched. To fix this error, you have two solutions: Use an array with a fixed length. Use a slice. Example with an array with a fixed length: Example with a slice:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn is_123<const N: usize>(x: [u32; N]) -> bool {
    match x {
        [1, 2, ..] => true, // error: cannot pattern-match on an
                            //        array without a fixed length
        _ => false
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn is_123(x: [u32; 3]) -> bool { // We use an array with a fixed size
    match x {
        [1, 2, ..] => true, // ok!
        _ => false
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0730.html


---

## E0731 — An enum with the representation hint repr(transparent) had zero or more than
one variants.

An enum with the representation hint repr(transparent) had zero or more than one variants. Because transparent enums are represented exactly like one of their variants at run time, said variant must be uniquely determined. If there is no variant, or if there are multiple variants, it is not clear how the enum should be represented.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(transparent)]
enum Status { // error: transparent enum needs exactly one variant, but has 2
    Errno(u32),
    Ok,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0731.html


---

## E0732 — An enum with a discriminant must specify a #[repr(inttype)].

An enum with a discriminant must specify a #[repr(inttype)]. A #[repr(inttype)] must be provided on an enum if it has a non-unit variant with a discriminant, or where there are both unit variants with discriminants and non-unit variants. This restriction ensures that there is a well-defined way to extract a variant’s discriminant from a value; for instance:

**錯誤範例**
```rust
enum Enum { // error!
    Unit = 1,
    Tuple() = 2,
    Struct{} = 3,
}
fn main() {}
```

**正解**
```rust
#[repr(u8)]
enum Enum {
    Unit = 3,
    Tuple(u16) = 2,
    Struct {
        a: u8,
        b: u16,
    } = 1,
}

fn discriminant(v : &Enum) -> u8 {
    unsafe { *(v as *const Enum as *const u8) }
}

fn main() {
    assert_eq!(3, discriminant(&Enum::Unit));
    assert_eq!(2, discriminant(&Enum::Tuple(5)));
    assert_eq!(1, discriminant(&Enum::Struct{a: 7, b: 11}));
}
```

出處：https://doc.rust-lang.org/error_codes/E0732.html


---

## E0733 — An async function used recursion without boxing.

An async function used recursion without boxing. The recursive invocation can be boxed: The Box<...> ensures that the result is of known size, and the pin is required to keep it in the same place in memory. Alternatively, the body can be boxed:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
async fn foo(n: usize) {
    if n > 0 {
        foo(n - 1).await;
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
async fn foo(n: usize) {
    if n > 0 {
        Box::pin(foo(n - 1)).await;
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0733.html


---

## E0734 — This error code was replaced by the more generic E0658.

Note: this error code is no longer emitted by the compiler. This error code was replaced by the more generic E0658. A stability attribute has been used outside of the standard library. These attributes are meant to only be used by the standard library and are rejected in your own crates.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[stable(feature = "a", since = "b")] // invalid
#[unstable(feature = "b", issue = "none")] // invalid
fn foo(){}
}
```

出處：https://doc.rust-lang.org/error_codes/E0734.html


---

## E0735 — Type parameter defaults cannot use Self on structs, enums, or unions.

Type parameter defaults cannot use Self on structs, enums, or unions.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo<X = Box<Self>> {
    field1: Option<X>,
    field2: Option<X>,
}
// error: type parameters cannot use `Self` in their defaults.
}
```

出處：https://doc.rust-lang.org/error_codes/E0735.html


---

## E0736 — Functions marked with the #[naked] attribute are restricted in what other
attributes they may be marked with.

Functions marked with the #[naked] attribute are restricted in what other attributes they may be marked with. Notable attributes that are incompatible with #[naked] are: #[inline] #[track_caller] #[test], #[ignore], #[should_panic] These incompatibilities are due to the fact that naked functions deliberately impose strict restrictions regarding the code that the compiler is allowed to produce for this function.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[inline]
#[unsafe(naked)]
fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0736.html


---

## E0737 — #[track_caller] requires functions to have the "Rust" ABI for implicitly
receiving caller location. See RFC 2091 for details on this and other
restrictions.

#[track_caller] requires functions to have the "Rust" ABI for implicitly receiving caller location. See RFC 2091 for details on this and other restrictions.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[track_caller]
extern "C" fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0737.html


---

## E0739 — #[track_caller] must be applied to a function

Note: this error code is no longer emitted by the compiler. #[track_caller] must be applied to a function

**正解**
```rust
#[track_caller]
struct Bar {
    a: u8,
}
```

出處：https://doc.rust-lang.org/error_codes/E0739.html


---

## E0740 — A union was declared with fields with destructors.

A union was declared with fields with destructors. A union cannot have fields with destructors.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
union Test {
    a: A, // error!
}

#[derive(Debug)]
struct A(i32);

impl Drop for A {
    fn drop(&mut self) { println!("A"); }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0740.html


---

## E0741 — A non-structural-match type was used as the type of a const generic parameter.

A non-structural-match type was used as the type of a const generic parameter. Only structural-match types, which are types that derive PartialEq and Eq and implement ConstParamTy, may be used as the types of const generic parameters. To fix the previous code example, we derive PartialEq, Eq, and ConstParamTy:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(adt_const_params)]

fn main() {
struct A;

struct B<const X: A>; // error!
}
```

**正解**
```rust
#![allow(unused)]
#![feature(adt_const_params)]

fn main() {
use std::marker::ConstParamTy;

#[derive(PartialEq, Eq, ConstParamTy)] // We derive both traits here.
struct A;

struct B<const X: A>; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0741.html


---

## E0742 — Visibility is restricted to a module which isn’t an ancestor of the current
item.

Visibility is restricted to a module which isn’t an ancestor of the current item. To fix this error, we need to move the Shark struct inside the sea module: Of course, you can do it as long as the module you’re referring to is an ancestor:

**錯誤範例**
```rust
pub mod sea {}

pub (in crate::sea) struct Shark; // error!

fn main() {}
```

**正解**
```rust
pub mod sea {
    pub (in crate::sea) struct Shark; // ok!
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0742.html


---

## E0743 — The C-variadic type ... has been nested inside another type.

The C-variadic type ... has been nested inside another type. Only foreign functions can use the C-variadic type (...). In such functions, ... may only occur non-nested. That is, y: &'a ... is not allowed. A C-variadic type is used to give an undefined number of parameters to a given function (like printf in C). The equivalent in Rust would be to use macros directly (like println! for example).

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo2(x: u8, y: &...) {} // error!
}
```

出處：https://doc.rust-lang.org/error_codes/E0743.html


---

## E0744 — An unsupported expression was used inside a const context.

Note: this error code is no longer emitted by the compiler. An unsupported expression was used inside a const context. At the moment, .await is forbidden inside a const, static, or const fn. This may be allowed at some point in the future, but the implementation is not yet complete. See the tracking issue for async in const fn.

**正解**
```rust
const _: i32 = {
    async { 0 }.await
};
```

出處：https://doc.rust-lang.org/error_codes/E0744.html


---

## E0745 — The address of temporary value was taken.

The address of temporary value was taken. In this example, 2 is destroyed right after the assignment, which means that ptr now points to an unavailable location. To avoid this error, first bind the temporary to a named local variable:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn temp_address() {
    let ptr = &raw const 2; // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn temp_address() {
    let val = 2;
    let ptr = &raw const val; // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0745.html


---

## E0746 — An unboxed trait object was used as a return value.

An unboxed trait object was used as a return value. Return types cannot be dyn Traits as they must be Sized. To avoid the error there are a couple of options. If there is a single type involved, you can use impl Trait: If there are multiple types involved, the only way you care to interact with them is through the trait’s interface, and having to rely on dynamic dispatch is acceptable, then you can use trait objects with Box, or other container types like Rc or Arc: Finally, if you wish to still be able to access the original type, you can create a new enum with a variant for each type: You can even implement the trait on the returned enum so the callers don’t have to match on the returned value to invoke the associated items: If you decide to use trait objects, be aware that these rely on dynamic dispatch, which has performance implications, as the compiler needs to emit code that will figure out which method to call at runtime instead of during compilation. Using trait objects we are trading flexibility for performance.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait T {
    fn bar(&self);
}
struct S(usize);
impl T for S {
    fn bar(&self) {}
}

// Having the trait `T` as return type is invalid because
// unboxed trait objects do not have a statically known size:
fn foo() -> dyn T { // error!
    S(42)
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait T {
    fn bar(&self);
}
struct S(usize);
impl T for S {
    fn bar(&self) {}
}
// The compiler will select `S(usize)` as the materialized return type of this
// function, but callers will only know that the return type implements `T`.
fn foo() -> impl T { // ok!
    S(42)
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0746.html


---

## E0747 — Generic arguments were not provided in the same order as the corresponding
generic parameters are declared.

Generic arguments were not provided in the same order as the corresponding generic parameters are declared. The argument order should be changed to match the parameter declaration order, as in the following:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct S<'a, T>(&'a T);

type X = S<(), 'static>; // error: the type argument is provided before the
                         // lifetime argument
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct S<'a, T>(&'a T);

type X = S<'static, ()>; // ok
}
```

出處：https://doc.rust-lang.org/error_codes/E0747.html


---

## E0748 — A raw string isn’t correctly terminated because the trailing # count doesn’t
match its leading # count.

A raw string isn’t correctly terminated because the trailing # count doesn’t match its leading # count. To terminate a raw string, you have to have the same number of # at the end as at the beginning. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let dolphins = r##"Dolphins!"#; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let dolphins = r#"Dolphins!"#; // One `#` at the beginning, one at the end so
                               // all good!
}
```

出處：https://doc.rust-lang.org/error_codes/E0748.html


---

## E0749 — An item was added on a negative impl.

An item was added on a negative impl. Negative impls are not allowed to have any items. Negative impls declare that a trait is not implemented (and never will be) and hence there is no need to specify the values for trait methods or other items. One way to fix this is to remove the items in negative impls:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#![feature(negative_impls)]
trait MyTrait {
    type Foo;
}

impl !MyTrait for u32 {
    type Foo = i32; // error!
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#![feature(negative_impls)]
trait MyTrait {
    type Foo;
}

impl !MyTrait for u32 {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0749.html


---

## E0750 — A negative impl was made default impl.

A negative impl was made default impl. Negative impls cannot be default impls. A default impl supplies default values for the items within to be used by other impls, whereas a negative impl declares that there are no other impls. Combining it does not make sense.

**錯誤範例**
```rust
#![feature(negative_impls)]
#![feature(specialization)]
trait MyTrait {
    type Foo;
}

default impl !MyTrait for u32 {} // error!
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0750.html


---

## E0751 — There are both a positive and negative trait implementation for the same type.

There are both a positive and negative trait implementation for the same type. Negative implementations are a promise that the trait will never be implemented for the given types. Therefore, both cannot exist at the same time.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait MyTrait {}
impl MyTrait for i32 { }
impl !MyTrait for i32 { } // error!
}
```

出處：https://doc.rust-lang.org/error_codes/E0751.html


---

## E0752 — The entry point of the program was marked as async.

The entry point of the program was marked as async. fn main() or the specified start function is not allowed to be async. Not having a correct async runtime library setup may cause this error. To fix it, declare the entry point without async:

**錯誤範例**
```rust
async fn main() -> Result<(), ()> { // error!
    Ok(())
}
```

**正解**
```rust
fn main() -> Result<(), ()> { // ok!
    Ok(())
}
```

出處：https://doc.rust-lang.org/error_codes/E0752.html


---

## E0753 — An inner doc comment was used in an invalid context.

An inner doc comment was used in an invalid context. Inner document can only be used before items. For example: In case you want to document the item following the doc comment, you might want to use outer doc comment:

**錯誤範例**
```rust
fn foo() {}
//! foo
// ^ error!
fn main() {}
```

**正解**
```rust
//! A working comment applied to the module!
fn foo() {
    //! Another working comment!
}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0753.html


---

## E0754 — A non-ASCII identifier was used in an invalid context.

A non-ASCII identifier was used in an invalid context. Erroneous code examples: Non-ASCII can be used as module names if it is inlined or if a #[path] attribute is specified. For example:

**錯誤範例**
```rust
mod řųśť; // error!

#[no_mangle]
fn řųśť() {} // error!

fn main() {}
```

**正解**
```rust
mod řųśť { // ok!
    const IS_GREAT: bool = true;
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0754.html


---

## E0755 — The ffi_pure attribute was used on a non-foreign function.

Note: this error code is no longer emitted by the compiler. The ffi_pure attribute was used on a non-foreign function. The ffi_pure attribute can only be used on foreign functions which do not have side effects or infinite loops: You can find more information about it in the unstable Rust Book.

**正解**
```rust
#![feature(ffi_pure)]

#[unsafe(ffi_pure)] // error!
pub fn foo() {}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0755.html


---

## E0756 — The ffi_const attribute was used on something other than a foreign function
declaration.

Note: this error code is no longer emitted by the compiler. The ffi_const attribute was used on something other than a foreign function declaration. The ffi_const attribute can only be used on foreign function declarations which have no side effects except for their return value: You can get more information about it in the unstable Rust Book.

**正解**
```rust
#![feature(ffi_const)]

#[unsafe(ffi_const)] // error!
pub fn foo() {}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0756.html


---

## E0757 — A function was given both the ffi_const and ffi_pure attributes.

A function was given both the ffi_const and ffi_pure attributes. As ffi_const provides stronger guarantees than ffi_pure, remove the ffi_pure attribute: You can get more information about const and pure in the GCC documentation on Common Function Attributes. The unstable Rust Book has more information about ffi_const and ffi_pure.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(ffi_const, ffi_pure)]

fn main() {
extern "C" {
    #[unsafe(ffi_const)]
    #[unsafe(ffi_pure)]
    //~^ ERROR `#[ffi_const]` function cannot be `#[ffi_pure]`
    pub fn square(num: i32) -> i32;
}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(ffi_const)]

fn main() {
extern "C" {
    #[unsafe(ffi_const)]
    pub fn square(num: i32) -> i32;
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0757.html


---

## E0758 — A multi-line (doc-)comment is unterminated.

A multi-line (doc-)comment is unterminated. The same goes for doc comments: You need to end your multi-line comment with */ in order to fix this error:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
/* I am not terminated!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
/* I am terminated! */
/*! I am also terminated! */
}
```

出處：https://doc.rust-lang.org/error_codes/E0758.html


---

## E0759 — Return type involving a trait did not require 'static lifetime.

Note: this error code is no longer emitted by the compiler. Return type involving a trait did not require 'static lifetime. Erroneous code examples: Add 'static requirement to fix them: Both dyn Trait and impl Trait in return types have an implicit 'static requirement, meaning that the value implementing them that is being returned has to be either a 'static borrow or an owned value. In order to change the requirement from 'static to be a lifetime derived from its arguments, you can add an explicit bound, either to an anonymous lifetime '_ or some appropriate named lifetime. These are equivalent to the following explicit lifetime annotations:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
use std::fmt::Debug;

fn foo(x: &i32) -> impl Debug { // error!
    x
}

fn bar(x: &i32) -> Box<dyn Debug> { // error!
    Box::new(x)
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
use std::fmt::Debug;
fn foo(x: &'static i32) -> impl Debug + 'static { // ok!
    x
}

fn bar(x: &'static i32) -> Box<dyn Debug + 'static> { // ok!
    Box::new(x)
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0759.html


---

## E0760 — async fn/impl trait return type cannot contain a projection
or Self that references lifetimes from a parent scope.

Note: this error code is no longer emitted by the compiler. async fn/impl trait return type cannot contain a projection or Self that references lifetimes from a parent scope. To fix this error we need to spell out Self to S<'a>: This will be allowed at some point in the future, but the implementation is not yet complete. See the issue-61949 for this limitation.

**正解**
```rust
struct S<'a>(&'a i32);

impl<'a> S<'a> {
    async fn new(i: &'a i32) -> Self {
        S(&22)
    }
}
```

出處：https://doc.rust-lang.org/error_codes/E0760.html


---

## E0761 — Multiple candidate files were found for an out-of-line module.

Multiple candidate files were found for an out-of-line module. Please remove this ambiguity by deleting/renaming one of the candidate files.

**錯誤範例**
```rust
// file: ambiguous_module/mod.rs

fn foo() {}

// file: ambiguous_module.rs

fn foo() {}

// file: lib.rs

mod ambiguous_module; // error: file for module `ambiguous_module`
                      // found at both ambiguous_module.rs and
                      // ambiguous_module/mod.rs
```

出處：https://doc.rust-lang.org/error_codes/E0761.html


---

## E0762 — A character literal wasn’t ended with a quote.

A character literal wasn’t ended with a quote. To fix this error, add the missing quote:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
static C: char = '●; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
static C: char = '●'; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0762.html


---

## E0763 — A byte constant wasn’t correctly ended.

A byte constant wasn’t correctly ended. To fix this error, add the missing quote:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let c = b'a; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let c = b'a'; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0763.html


---

## E0764 — A mutable reference was used in a constant.

A mutable reference was used in a constant. Mutable references (&mut) can only be used in constant functions, not statics or constants. This limitation exists to prevent the creation of constants that have a mutable reference in their final value. If you had a constant of &mut i32 type, you could modify the value through that reference, making the constant essentially mutable. While there could be a more fine-grained scheme in the future that allows mutable references if they are not “leaked” to the final value, a more conservative approach was chosen for now. const fn do not have this problem, as the borrow checker will prevent the const fn from returning new mutable references. Remember: you cannot use a function call inside a constant or static. However, you can totally use it in constant functions:

**錯誤範例**
```rust
fn main() {
    const OH_NO: &'static mut usize = &mut 1; // error!
}
```

**正解**
```rust
const fn foo(x: usize) -> usize {
    let mut y = 1;
    let z = &mut y;
    *z += x;
    y
}

fn main() {
    const FOO: usize = foo(10); // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0764.html


---

## E0765 — A double quote string (") was not terminated.

A double quote string (") was not terminated. To fix this error, add the missing double quote at the end of the string:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let s = "; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let s = ""; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0765.html


---

## E0766 — A double quote byte string (b") was not terminated.

A double quote byte string (b") was not terminated. To fix this error, add the missing double quote at the end of the string:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let s = b"; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let s = b""; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0766.html


---

## E0767 — An unreachable label was used.

An unreachable label was used. Ensure that the label is within scope. Labels are not reachable through functions, closures, async blocks or modules. Example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
'a: loop {
    || {
        loop { break 'a } // error: use of unreachable label `'a`
    };
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
'a: loop {
    break 'a; // ok!
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0767.html


---

## E0768 — A number in a non-decimal base has no digits.

A number in a non-decimal base has no digits. To fix this error, add the missing digits:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let s: i32 = 0b; // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
let s: i32 = 0b1; // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0768.html


---

## E0769 — A tuple struct or tuple variant was used in a pattern as if it were a struct or
struct variant.

A tuple struct or tuple variant was used in a pattern as if it were a struct or struct variant. To fix this error, you can use the tuple pattern: Alternatively, you can also use the struct pattern by using the correct field names and binding them to new identifiers:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
enum E {
    A(i32),
}

let e = E::A(42);

match e {
    E::A { number } => { // error!
        println!("{}", number);
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
enum E {
    A(i32),
}
let e = E::A(42);
match e {
    E::A(number) => { // ok!
        println!("{}", number);
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0769.html


---

## E0770 — The type of a const parameter references other generic parameters.

The type of a const parameter references other generic parameters. To fix this error, use a concrete type for the const parameter:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo<T, const N: T>() {} // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo<T, const N: usize>() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0770.html


---

## E0771 — Note: this error code is no longer emitted by the compiler
A non-'static lifetime was used in a const generic.

Note: this error code is no longer emitted by the compiler A non-'static lifetime was used in a const generic. This is currently not allowed. To fix this issue, the lifetime in the const generic need to be changed to 'static: For more information, see GitHub issue #74052.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(adt_const_params, unsized_const_params)]

fn main() {
fn function_with_str<'a, const STRING: &'a str>() {} // error!
}
```

**正解**
```rust
#![allow(unused)]
#![feature(adt_const_params, unsized_const_params)]

fn main() {
fn function_with_str<const STRING: &'static str>() {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0771.html


---

## E0772 — A trait object has some specific lifetime '1, but it was used in a way that
requires it to have a 'static lifetime.

Note: this error code is no longer emitted by the compiler. A trait object has some specific lifetime '1, but it was used in a way that requires it to have a 'static lifetime. Example of erroneous code: The trait object person in the function get_is_cool, while already being behind a reference with lifetime 'p, also has it’s own implicit lifetime, '2. Lifetime '2 represents the data the trait object might hold inside, for example: With this scenario, if a trait object of dyn MyTrait + '2 was made from MyStruct<'a>, 'a must live as long, if not longer than '2. This allows the trait object’s internal data to be accessed safely from any trait methods. This rule also goes for any lifetime any struct made into a trait object may have. In the implementation for dyn Person, the '2 lifetime representing the internal data was omitted, meaning that the compiler inferred the lifetime 'static. As a result, the implementation’s is_cool is inferred by the compiler to look like this: While the get_is_cool function is inferred to look like this: Which brings us to the core of the problem; the assignment of type &'_ (dyn Person + '_) to type &'_ (dyn Person + 'static) is impossible. Fixing it is as simple as being generic over lifetime '2, as to prevent the compiler from inferring it as 'static: See the [Rust Reference on Trait Object Lifetime Bounds][trait-objects] for more information on trait object lifetimes.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait BooleanLike {}
trait Person {}

impl BooleanLike for bool {}

impl dyn Person {
    fn is_cool(&self) -> bool {
        // hey you, you're pretty cool
        true
    }
}

fn get_is_cool<'p>(person: &'p dyn Person) -> impl BooleanLike {
    // error: `person` has an anonymous lifetime `'p` but calling
    //        `print_cool_fn` introduces an implicit `'static` lifetime
    //        requirement
    person.is_cool()
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait MyTrait {}

struct MyStruct<'a>(&'a i32);

impl<'a> MyTrait for MyStruct<'a> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0772.html


---

## E0773 — This was triggered when multiple macro definitions used the same
#[rustc_builtin_macro(..)].

Note: this error code is no longer emitted by the compiler. This was triggered when multiple macro definitions used the same #[rustc_builtin_macro(..)]. This is no longer an error.

出處：https://doc.rust-lang.org/error_codes/E0773.html


---

## E0774 — derive was applied on something which is not a struct, a union or an enum.

derive was applied on something which is not a struct, a union or an enum. As said above, the derive attribute is only allowed on structs, unions or enums: You can find more information about derive in the Rust Book.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {
    #[derive(Clone)] // error!
    type Bar;
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[derive(Clone)] // ok!
struct Bar {
    field: u32,
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0774.html


---

## E0775 — #[cmse_nonsecure_entry] is only valid for targets with the TrustZone-M
extension.

Note: this error code is no longer emitted by the compiler. #[cmse_nonsecure_entry] is only valid for targets with the TrustZone-M extension. To fix this error, compile your code for a Rust target that supports the TrustZone-M extension. The current possible targets are: thumbv8m.main-none-eabi thumbv8m.main-none-eabihf thumbv8m.base-none-eabi

**正解**
```rust
#![feature(cmse_nonsecure_entry)]

pub extern "cmse-nonsecure-entry" fn entry_function() {}
```

出處：https://doc.rust-lang.org/error_codes/E0775.html


---

## E0776 — #[cmse_nonsecure_entry] functions require a C ABI
To fix this error, declare your entry function with a C ABI, using extern "C".

Note: this error code is no longer emitted by the compiler. #[cmse_nonsecure_entry] functions require a C ABI To fix this error, declare your entry function with a C ABI, using extern "C".

**正解**
```rust
#![feature(cmse_nonsecure_entry)]

#[no_mangle]
#[cmse_nonsecure_entry]
pub fn entry_function(input: Vec<u32>) {}
```

出處：https://doc.rust-lang.org/error_codes/E0776.html


---

## E0777 — A literal value was used inside #[derive].

A literal value was used inside #[derive]. Only paths to traits are allowed as argument inside #[derive]. You can find more information about the #[derive] attribute in the Rust Book.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[derive("Clone")] // error!
struct Foo;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[derive(Clone)] // ok!
struct Foo;
}
```

出處：https://doc.rust-lang.org/error_codes/E0777.html


---

## E0778 — Note: this error code is no longer emitted by the compiler
The instruction_set attribute was malformed.

Note: this error code is no longer emitted by the compiler The instruction_set attribute was malformed. The parenthesized instruction_set attribute requires the parameter to be specified: or: For more information see the instruction_set attribute section of the Reference.

**錯誤範例**
```rust
#![feature(isa_attribute)]

#[instruction_set()] // error: expected one argument
pub fn something() {}
fn main() {}
```

**正解**
```rust
#![allow(unused)]
#![feature(isa_attribute)]

fn main() {
#[cfg_attr(target_arch="arm", instruction_set(arm::a32))]
fn something() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0778.html


---

## E0779 — Note: this error code is no longer emitted by the compiler
An unknown argument was given to the instruction_set attribute.

Note: this error code is no longer emitted by the compiler An unknown argument was given to the instruction_set attribute. The instruction_set attribute only supports two arguments currently: arm::a32 arm::t32 All other arguments given to the instruction_set attribute will return this error. Example: For more information see the instruction_set attribute section of the Reference.

**錯誤範例**
```rust
#![feature(isa_attribute)]

#[instruction_set(intel::x64)] // error: invalid argument
pub fn something() {}
fn main() {}
```

**正解**
```rust
#![feature(isa_attribute)]

#[cfg_attr(target_arch="arm", instruction_set(arm::a32))] // ok!
pub fn something() {}
fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0779.html


---

## E0780 — Cannot use doc(inline) with anonymous imports

Cannot use doc(inline) with anonymous imports Anonymous imports are always rendered with #[doc(no_inline)]. To fix this error, remove the #[doc(inline)] attribute. Example:

**正解**
```rust
#[doc(inline)] // error: invalid doc argument
pub use foo::Foo as _;
```

出處：https://doc.rust-lang.org/error_codes/E0780.html


---

## E0781 — The cmse-nonsecure-call ABI can only be used with function pointers.

The cmse-nonsecure-call ABI can only be used with function pointers. The cmse-nonsecure-call ABI should be used by casting function pointers to specific addresses.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(abi_cmse_nonsecure_call)]

fn main() {
pub extern "cmse-nonsecure-call" fn test() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0781.html


---

## E0782 — Trait objects must include the dyn keyword.

Trait objects must include the dyn keyword. Trait objects are a way to call methods on types that are not known until runtime but conform to some trait. Trait objects should be formed with Box<dyn Foo>, but in the code above dyn is left off. This makes it harder to see that arg is a trait object and not a simply a heap allocated type called Foo. To fix this issue, add dyn before the trait name. This used to be allowed before edition 2021, but is now an error.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Foo {}
fn test(arg: Box<Foo>) {} // error!
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Foo {}
fn test(arg: Box<dyn Foo>) {} // ok!
}
```

出處：https://doc.rust-lang.org/error_codes/E0782.html


---

## E0783 — The range pattern ... is no longer allowed.

The range pattern ... is no longer allowed. Older Rust code using previous editions allowed ... to stand for inclusive ranges which are now signified using ..=. To make this code compile replace the ... with ..=.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
match 2u8 {
    0...9 => println!("Got a number less than 10"), // error!
    _ => println!("Got a number 10 or more"),
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
match 2u8 {
    0..=9 => println!("Got a number less than 10"), // ok!
    _ => println!("Got a number 10 or more"),
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0783.html


---

## E0784 — A union expression does not have exactly one field.

A union expression does not have exactly one field. The key property of unions is that all fields of a union share common storage. As a result, writes to one field of a union can overwrite its other fields, and size of a union is determined by the size of its largest field. You can find more information about the union types in the Rust reference. Working example:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
union Bird {
    pigeon: u8,
    turtledove: u16,
}

let bird = Bird {}; // error
let bird = Bird { pigeon: 0, turtledove: 1 }; // error
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
union Bird {
    pigeon: u8,
    turtledove: u16,
}

let bird = Bird { pigeon: 0 }; // OK
}
```

出處：https://doc.rust-lang.org/error_codes/E0784.html


---

## E0785 — An inherent impl was written on a dyn auto trait.

An inherent impl was written on a dyn auto trait. Dyn objects allow any number of auto traits, plus at most one non-auto trait. The non-auto trait becomes the “principal trait”. When checking if an impl on a dyn trait is coherent, the principal trait is normally the only one considered. Since the erroneous code has no principal trait, it cannot be implemented at all. Working example:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(auto_traits)]

fn main() {
auto trait AutoTrait {}

impl dyn AutoTrait {}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(auto_traits)]

fn main() {
trait PrincipalTrait {}

auto trait AutoTrait {}

impl dyn PrincipalTrait + AutoTrait + Send {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0785.html


---

## E0786 — A metadata file was invalid.

A metadata file was invalid. When loading crates, each crate must have a valid metadata file. Invalid files could be caused by filesystem corruption, an IO error while reading the file, or (rarely) a bug in the compiler itself. Consider deleting the file and recreating it, or reporting a bug against the compiler.

**正解**
```rust
use ::foo; // error: found invalid metadata files for crate `foo`
```

出處：https://doc.rust-lang.org/error_codes/E0786.html


---

## E0787 — An unsupported naked function definition.

An unsupported naked function definition. The naked function must be defined using a single naked_asm! assembly block. The execution must never fall through past the end of the assembly code, so it must either return or diverge. The asm block can also use att_syntax and raw options, but others options are not allowed. The asm block must not contain any operands other than const and sym. Additional information For more information, please see RFC 2972.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[unsafe(naked)]
pub extern "C" fn f() -> u32 {
    42
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0787.html


---

## E0788 — A #[coverage(off|on)] attribute was found in a position where it is not
allowed.

Note: this error code is no longer emitted by the compiler. A #[coverage(off|on)] attribute was found in a position where it is not allowed. Coverage attributes can be applied to: Function and method declarations that have a body, including trait methods that have a default implementation. Closure expressions, in situations where attributes can be applied to expressions. impl blocks (inherent or trait), and modules. Example of erroneous code: When using the -C instrument-coverage flag, coverage attributes act as a hint to the compiler that it should instrument or not instrument the corresponding function or enclosed functions. The precise effect of applying a coverage attribute is not guaranteed and may change in future compiler versions.

**正解**
```rust
unsafe extern "C" {
    #[coverage(off)]
    fn foreign_fn();
}
```

出處：https://doc.rust-lang.org/error_codes/E0788.html


---

## E0789 — This error code is internal to the compiler and will not be emitted with normal Rust code.

This error code is internal to the compiler and will not be emitted with normal Rust code. The internal rustc_allowed_through_unstable_modules attribute must be used on an item with a stable attribute. Typically when an item is marked with a stable attribute, the modules that enclose the item must also be marked with stable attributes, otherwise the item becomes de facto unstable. #[rustc_allowed_through_unstable_modules] is a workaround which allows an item to “escape” its unstable parent modules. This error occurs when an item is marked with #[rustc_allowed_through_unstable_modules] but no supplementary stable attribute exists. See #99288 for an example of #[rustc_allowed_through_unstable_modules] in use.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
// NOTE: both of these attributes are perma-unstable and should *never* be
//       used outside of the compiler and standard library.
#![feature(rustc_attrs)]
#![feature(staged_api)]
#![allow(internal_features)]

#![unstable(feature = "foo_module", reason = "...", issue = "123")]

#[rustc_allowed_through_unstable_modules = "deprecation message"]
// #[stable(feature = "foo", since = "1.0")]
struct Foo;
// ^^^ error: `rustc_allowed_through_unstable_modules` attribute must be
//            paired with a `stable` attribute
}
```

出處：https://doc.rust-lang.org/error_codes/E0789.html


---

## E0790 — You need to specify a specific implementation of the trait in order to call the
method.

You need to specify a specific implementation of the trait in order to call the method. This error can be solved by adding type annotations that provide the missing information to the compiler. In this case, the solution is to use a concrete type:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait Coroutine {
    fn create() -> u32;
}

struct Impl;

impl Coroutine for Impl {
    fn create() -> u32 { 1 }
}

struct AnotherImpl;

impl Coroutine for AnotherImpl {
    fn create() -> u32 { 2 }
}

let cont: u32 = Coroutine::create();
// error, impossible to choose one of Coroutine trait implementation
// Should it be Impl or AnotherImpl, maybe something else?
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
trait Coroutine {
    fn create() -> u32;
}

struct AnotherImpl;

impl Coroutine for AnotherImpl {
    fn create() -> u32 { 2 }
}

let gen1 = AnotherImpl::create();

// if there are multiple methods with same name (different traits)
let gen2 = <AnotherImpl as Coroutine>::create();
}
```

出處：https://doc.rust-lang.org/error_codes/E0790.html


---

## E0791 — Static variables with the #[linkage] attribute within external blocks
must have one of the following types, which are equivalent to a nullable
pointer in C:

Static variables with the #[linkage] attribute within external blocks must have one of the following types, which are equivalent to a nullable pointer in C: *mut T or *const T, where T may be any type. An enumerator type with no #[repr] attribute and with two variants, where one of the variants has no fields, and the other has a single field of one of the following non-nullable types: Reference type Function pointer type The variants can appear in either order. For example, the following declaration is invalid: The following declarations are valid:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(linkage)]

fn main() {
extern "C" {
    #[linkage = "extern_weak"]
    static foo: i8;
}
}
```

**正解**
```rust
#![allow(unused)]
#![feature(linkage)]

fn main() {
extern "C" {
    #[linkage = "extern_weak"]
    static foo: Option<unsafe extern "C" fn()>;

    #[linkage = "extern_weak"]
    static bar: Option<&'static i8>;

    #[linkage = "extern_weak"]
    static baz: *mut i8;
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0791.html


---

## E0792 — A type alias impl trait can only have its hidden type assigned
when used fully generically (and within their defining scope).
This means

A type alias impl trait can only have its hidden type assigned when used fully generically (and within their defining scope). This means is not accepted. If it were accepted, one could create unsound situations like Instead you need to make the function generic: This means that no matter the generic parameter to foo, the hidden type will always be u32. If you want to link the generic parameter to the hidden type, you can do that, too:

**錯誤範例**
```rust
#![allow(unused)]
#![feature(type_alias_impl_trait)]

fn main() {
type Foo<T> = impl std::fmt::Debug;

#[define_opaque(Foo)]
fn foo() -> Foo<u32> {
    5u32
}
}
```

**正解**
```rust
#![feature(type_alias_impl_trait)]

type Foo<T> = impl std::fmt::Debug;

#[define_opaque(Foo)]
fn foo<U>() -> Foo<U> {
    5u32
}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0792.html


---

## E0793 — An unaligned reference to a field of a packed struct or union was created.

An unaligned reference to a field of a packed struct or union was created. The #[repr(packed)] attribute removes padding between fields, which can cause fields to be stored at unaligned memory addresses. Creating references to such fields violates Rust’s memory safety guarantees and can lead to undefined behavior in optimized code. Creating a reference to an insufficiently aligned packed field is undefined behavior and therefore disallowed. Using an unsafe block does not change anything about this. Instead, the code should do a copy of the data in the packed field or use raw pointers and unaligned accesses. Unions Although creating a reference to a union field is unsafe, this error will still be triggered if the referenced field is not sufficiently aligned. Use addr_of! and raw pointers in the same way as for struct fields. Additional information Note that this error is specifically about references to packed fields. Direct by-value access of those fields is fine, since then the compiler has enough information to generate the correct kind of access. See issue #82523 for more information.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[repr(packed)]
pub struct Foo {
    field1: u64,
    field2: u8,
}

unsafe {
    let foo = Foo { field1: 0, field2: 0 };
    // Accessing the field directly is fine.
    let val = foo.field1;
    // A reference to a packed field causes a error.
    let val = &foo.field1; // ERROR
    // An implicit `&` is added in format strings, causing the same error.
    println!("{}", foo.field1); // ERROR
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[repr(packed)]
pub struct Foo {
    field1: u64,
    field2: u8,
}

unsafe {
    let foo = Foo { field1: 0, field2: 0 };

    // Instead of a reference, we can create a raw pointer...
    let ptr = std::ptr::addr_of!(foo.field1);
    // ... and then (crucially!) access it in an explicitly unaligned way.
    let val = unsafe { ptr.read_unaligned() };
    // This would *NOT* be correct:
    // let val = unsafe { *ptr }; // Undefined Behavior due to unaligned load!

    // For formatting, we can create a copy to avoid the direct reference.
    let copy = foo.field1;
    println!("{}", copy);

    // Creating a copy can be written in a single line with curly braces.
    // (This is equivalent to the two lines above.)
    println!("{}", { foo.field1 });

    // A reference to a field that will always be sufficiently aligned is safe:
    println!("{}", foo.field2);
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0793.html


---

## E0794 — A lifetime parameter of a function definition is called late-bound if it both:

A lifetime parameter of a function definition is called late-bound if it both: appears in an argument type does not appear in a generic type constraint You cannot specify lifetime arguments for late-bound lifetime parameters. The type of a concrete instance of a generic function is universally quantified over late-bound lifetime parameters. This is because we want the function to work for any lifetime instantiated for the late-bound lifetime parameter, no matter where the function is called. Consequently, it doesn’t make sense to specify arguments for late-bound lifetime parameters, since they are not resolved until the function’s call site(s). To fix the issue, remove the specified lifetime: Additional information Lifetime parameters that are not late-bound are called early-bound. Confusion may arise from the fact that late-bound and early-bound lifetime parameters are declared the same way in function definitions. When referring to a function pointer type, universal quantification over late-bound lifetime parameters can be made explicit: In the definition of bar, the lifetime parameter 'a is late-bound, while 'b is early-bound. This is reflected in the type annotation for bar_fn, where 'a is universally quantified and 'b is instantiated with a specific lifetime. It is not allowed to explicitly specify early-bound lifetime arguments when late-bound lifetime parameters are present (as for bar_fn2, see issue #42868), although the types that are constrained by early-bound parameters can be specified (as for bar_fn3).

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn foo<'a>(x: &'a str) -> &'a str { x }
let _ = foo::<'static>;
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
fn foo<'a>(x: &'a str) -> &'a str { x }
let _ = foo;
}
```

出處：https://doc.rust-lang.org/error_codes/E0794.html


---

## E0795 — Invalid argument for the offset_of! macro.

Invalid argument for the offset_of! macro. The offset_of! macro gives the offset of a field within a type. It can navigate through enum variants, but the final component of its second argument must be a field and not a variant. The offset of the contained u8 in the Option<u8> can be found by specifying the field name 0: The discriminant of an enumeration may be read with core::mem::discriminant, but this is not always a value physically present within the enum. Further information about enum layout may be found at https://rust-lang.github.io/unsafe-code-guidelines/layout/enums.html.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(offset_of_enum)]

fn main() {
let x = std::mem::offset_of!(Option<u8>, Some);
}
```

**正解**
```rust
#![allow(unused)]
#![feature(offset_of_enum)]

fn main() {
let x: usize = std::mem::offset_of!(Option<u8>, Some.0);
}
```

出處：https://doc.rust-lang.org/error_codes/E0795.html


---

## E0796 — You have created a reference to a mutable static.

Note: this error code is no longer emitted by the compiler. You have created a reference to a mutable static. A reference to a mutable static has lifetime 'static. This is very dangerous as it is easy to accidentally overlap the lifetime of that reference with other, conflicting accesses to the same static. References to mutable statics are a hard error in the 2024 edition.

**正解**
```rust
#![allow(unused)]
fn main() {
static mut X: i32 = 23;
fn work() {
  let _val = unsafe { X };
}
let x_ref = unsafe { &mut X };
work();
// The next line has Undefined Behavior!
// `x_ref` is a mutable reference and allows no aliases,
// but `work` has been reading the reference between
// the moment `x_ref` was created and when it was used.
// This violates the uniqueness of `x_ref`.
*x_ref = 42;
}
```

出處：https://doc.rust-lang.org/error_codes/E0796.html


---

## E0797 — Struct update syntax was used without a base expression.

Struct update syntax was used without a base expression. Using struct update syntax requires a ‘base expression’. This will be used to fill remaining fields.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    fizz: u8,
    buzz: u8
}

let f1 = Foo { fizz: 10, buzz: 1};
let f2 = Foo { fizz: 10, .. }; // error
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo {
    fizz: u8,
    buzz: u8
}

let f1 = Foo { fizz: 10, buzz: 1};
let f2 = Foo { fizz: 10, ..f1 };
}
```

出處：https://doc.rust-lang.org/error_codes/E0797.html


---

## E0798 — Functions marked as cmse-nonsecure-call place restrictions on their
inputs and outputs.

Functions marked as cmse-nonsecure-call place restrictions on their inputs and outputs. inputs must fit in the 4 available 32-bit argument registers. Alignment is relevant. outputs must either fit in 4 bytes, or be a foundational type of size 8 (i64, u64, f64). no generics can be used in the signature For more information, see arm’s aapcs32. Arguments’ alignment is respected. In the example below, padding is inserted so that the u64 argument is passed in registers r2 and r3. There is then no room left for the final f32 argument

**正解**
```rust
#![feature(abi_cmse_nonsecure_call)]

#[no_mangle]
pub fn test(
    f: extern "cmse-nonsecure-call" fn(u32, u32, u32, u32, u32) -> u32,
) -> u32 {
    f(1, 2, 3, 4, 5)
}
```

出處：https://doc.rust-lang.org/error_codes/E0798.html


---

## E0799 — Something other than a type or const parameter has been used when one was
expected.

Something other than a type or const parameter has been used when one was expected. In the given examples, for bad1, the name main corresponds to a function rather than a type or const parameter. In bad2, the name x corresponds to a function argument rather than a type or const parameter. Only type and const parameters, including Self, may be captured by use<...> precise capturing bounds.

**錯誤範例**
```rust
fn bad1() -> impl Sized + use<main> {}

fn bad2(x: ()) -> impl Sized + use<x> {}

fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0799.html


---

## E0800 — A type or const parameter of the given name is not in scope.

A type or const parameter of the given name is not in scope. Erroneous code examples: To fix this error, please verify you didn’t misspell the type or const parameter, or double-check if you forgot to declare the parameter in the list of generics.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
fn missing() -> impl Sized + use<T> {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0800.html


---

## E0801 — The self parameter in a method has an invalid generic “receiver type”.

The self parameter in a method has an invalid generic “receiver type”. or alternatively, Methods take a special first parameter, termed self. It’s normal to use self, &self or &mut self, which are syntactic sugar for self: Self, self: &Self, and self: &mut Self respectively. But it’s also possible to use more sophisticated types of self parameter, for instance std::rc::Rc<Self>. The set of allowable Self types is extensible using the nightly feature Arbitrary self types. This will extend the valid set of Self types to anything which implements std::ops::Deref<Target=Self>, for example Rc<Self>, Box<Self>, or your own smart pointers that do the same. However, even with that feature, the self type must be concrete. Generic self types are not permitted. Specifically, a self type will be rejected if it is a type parameter defined on the method. These are OK:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Foo {
    fn foo<R: std::ops::Deref<Target=Self>>(self: R) {}
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
struct Foo;

impl Foo {
    fn foo(self) {}
    fn foo2(self: std::rc::Rc<Self>) {} // or some other similar
        // smart pointer if you enable arbitrary self types and
        // the pointer implements Deref<Target=Self>
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0801.html


---

## E0802 — The target of derive(CoercePointee) macro has inadmissible specification for
a meaningful use.

The target of derive(CoercePointee) macro has inadmissible specification for a meaningful use. Erroneous code examples: The target data is not a struct. The target data has a layout that is not transparent, or repr(transparent) in other words. The target data has no data field. The target data is not generic over any data, or has no generic type parameter. The target data has multiple generic type parameters, but none is designated as a pointee for coercion. The target data has multiple generic type parameters that are designated as pointees for coercion. The type parameter that is designated as a pointee is not marked ?Sized. In summary, the CoercePointee macro demands the type to be a struct that is generic over at least one type or over more types, one of which is marked with #[pointee], and has at least one data field and adopts a repr(transparent) layout. The only generic type or the type marked with #[pointee] has to be also marked as ?Sized.

**錯誤範例**
```rust
#![allow(unused)]
#![feature(coerce_pointee)]
fn main() {
use std::marker::CoercePointee;
#[derive(CoercePointee)]
enum NotStruct<'a, T: ?Sized> {
    Variant(&'a T),
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0802.html


---

## E0803 — A trait implementation returns a reference without an
explicit lifetime linking it to self.
It commonly arises in generic trait implementations
requiring explicit lifetime bounds.

A trait implementation returns a reference without an explicit lifetime linking it to self. It commonly arises in generic trait implementations requiring explicit lifetime bounds. The trait method returns &f64 requiring an independent lifetime The struct Container<’a> carries lifetime parameter ’a The compiler cannot verify if the returned reference satisfies ’a constraints Solution Explicitly bind lifetimes to clarify constraints:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
trait DataAccess<T> {
    fn get_ref(&self) -> T;
}

struct Container<'a> {
    value: &'a f64,
}

// Attempting to implement reference return
impl<'a> DataAccess<&f64> for Container<'a> {
    fn get_ref(&self) -> &f64 { // Error: Lifetime mismatch
        self.value
    }
}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
// Modified trait with explicit lifetime binding
trait DataAccess<'a, T> {
    fn get_ref(&'a self) -> T;
}

struct Container<'a> {
    value: &'a f64,
}

// Correct implementation (bound lifetimes)
impl<'a> DataAccess<'a, &'a f64> for Container<'a> {
    fn get_ref(&'a self) -> &'a f64 {
        self.value
    }
}
}
```

出處：https://doc.rust-lang.org/error_codes/E0803.html


---

## E0804 — An auto trait cannot be added to the bounds of a dyn Trait type via
a pointer cast.

An auto trait cannot be added to the bounds of a dyn Trait type via a pointer cast. Adding an auto trait can make the vtable invalid, potentially causing UB in safe code afterwards. For example: To fix this error, you can use transmute rather than pointer casts, but you must ensure that the vtable is valid for the pointer’s type before calling a method on the trait object or allowing other code to do so.

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
let ptr: *const dyn core::any::Any = &();
_ = ptr as *const (dyn core::any::Any + Send);
}
```

**正解**
```rust
use core::{mem::transmute, ptr::NonNull};

trait Trait {
    fn f(&self)
    where
        Self: Send;
}

impl Trait for NonNull<()> {
    fn f(&self) {
        unreachable!()
    }
}

fn main() {
    let unsend: &dyn Trait = &NonNull::dangling();
    let bad: &(dyn Trait + Send) = unsafe { transmute(unsend) };
    // This crashes, since the vtable for `NonNull as dyn Trait` does
    // not have an entry for `Trait::f`.
    bad.f();
}
```

出處：https://doc.rust-lang.org/error_codes/E0804.html


---

## E0805 — An attribute was given an invalid number of arguments

An attribute was given an invalid number of arguments To fix this, either give the right number of arguments the attribute needs. In the case of inline, this could be none at all: or only one:

**錯誤範例**
```rust
#![allow(unused)]
fn main() {
#[inline()] // error! should either have a single argument, or no parentheses
fn foo() {}

#[inline(always, never)] // error! should have only one argument, not two
fn bar() {}
}
```

**正解**
```rust
#![allow(unused)]
fn main() {
#[inline]
fn foo() {}
}
```

出處：https://doc.rust-lang.org/error_codes/E0805.html


---

## E0806 — An externally implementable item is not compatible with its declaration.

An externally implementable item is not compatible with its declaration. The error here is caused by the fact that y implements the externally implementable item foo. It can only do so if the signature of the implementation y matches that of the declaration of foo. So, to fix this, y’s signature must be changed to match that of x: One common way this can be triggered is by using the wrong signature for #[panic_handler]. The signature is provided by core. Should be: window.playground_copyable = true; window.addEventListener('load', function() { window.setTimeout(window.print, 100); });

**錯誤範例**
```rust
#![feature(extern_item_impls)]

#[eii(foo)]
fn x();

#[foo]
fn y(a: u64) -> u64 {
//~^ ERROR E0806
    a
}


fn main() {}
```

**正解**
```rust
#![feature(extern_item_impls)]

#[eii(foo)]
fn x();

#[foo]
fn y() {}


fn main() {}
```

出處：https://doc.rust-lang.org/error_codes/E0806.html
