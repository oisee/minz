# The Nanz Language: A Complete Guide

> Nanz is a typed systems language designed for Z80 and other retro targets.
> It compiles through HIR → MIR2 → Z80 assembly — a clean, modern pipeline
> that sits on the same backend as the PL/M-80 frontend.

**Version:** MinZ compiler v0.19.5 (MIR2 pipeline)
**Status:** Active development — core features stable, some corners rough

---

## Table of Contents

1. [What is Nanz?](#chapter-1-what-is-nanz)
2. [Syntax Reference](#chapter-2-syntax-reference)
3. [Semantics and Type System](#chapter-3-semantics-and-type-system)
4. [Compilation Deep Dive](#chapter-4-compilation-deep-dive)
5. [Iterator Chains](#chapter-5-iterator-chains)
6. [Zero-Cost Abstractions on Z80](#chapter-6-zero-cost-abstractions-on-z80)
7. [Relation to MinZ and PL/M](#chapter-7-relation-to-minz-and-plm)
8. [Comparison with Other Languages](#chapter-8-comparison-with-other-languages)
9. [Z80 Extern Functions and Register Contracts](#chapter-9-z80-extern-functions-and-register-contracts)
10. [Deep Optimization: CondRetSink and Flag Fusion](#chapter-10-deep-optimization-condret-sink-and-flag-fusion)
- [Appendix A: Complete Syntax Grammar](#appendix-a-complete-syntax-grammar)
- [Appendix B: Register Class Reference](#appendix-b-register-class-reference)
- [Appendix C: CLI Usage](#appendix-c-cli-usage)

---

## Chapter 1: What is Nanz?

Nanz (`.nanz` extension) is the active frontend language of the MinZ compiler system. It is a statically typed, imperative language designed for two audiences:

1. Developers writing new Z80 / retro-target programs who want modern syntax and zero-cost abstractions.
2. The MinZ compiler team developing and testing the MIR2 backend.

### 1.1 The Compilation Pipeline

```
source.nanz
    │
    ▼  nanz.Parse()
*hir.Module           ← High-level IR (structured control flow, named vars)
    │
    ▼  hir.LowerModule()
*mir2.Module          ← Mid-level IR (SSA-like, virtual registers, typed ops)
    │
    ▼  mir2 optimization passes
*mir2.Module          ← (constant folding, dead store elim, LUTGen, contracts)
    │
    ▼  z80codegen.Emit()
source.a80            ← MZA-compatible Z80 assembly
    │
    ▼  mza (assembler)
source.bin / .tap     ← binary / ZX Spectrum tape
```

The pipeline is invoked with:

```
mz source.nanz -o output.a80
```

### 1.2 Nanz vs. MinZ

MinZ (`.minz`) is the original frontend. It has its own parser, semantic analyzer, and code generator targeting the older MIR1 IR. That pipeline is **frozen** — it still works but is not being developed further.

Nanz is the **replacement** frontend. It targets MIR2, which has:

- Proper SSA-like dataflow with block parameters
- Interprocedural register allocation (calling conventions optimized per function)
- LUT generation: compile-time function evaluation for pure, range-bounded functions
- PBQP register allocator (Phase 6): cost-weighted assignment of virtuals to Z80 physicals
- A Z80 emulator used as a constant evaluator inside the compiler

The rule: write new retro programs in Nanz, keep existing `.minz` programs as-is.

### 1.3 Nanz vs. PL/M-80

PL/M-80 (`.plm`) is an Intel language from the 1970s used to write CP/M and early microcomputer software. The MinZ compiler includes a PL/M-80 parser that compiles PL/M programs through the **same HIR → MIR2 → Z80 pipeline** as Nanz.

This lets you:
- Modernize PL/M-80 programs by porting them to Nanz without changing the backend.
- Compare PL/M idioms with Nanz idioms side-by-side (the `examples/nanz/` directory includes paired files).

### 1.4 Design Philosophy

Nanz is deliberately minimal. The grammar fits in a screenful. There is no garbage collector, no runtime system, no dynamic dispatch. Every abstraction — lambdas, iterators, struct methods, interfaces — compiles to direct Z80 instructions with no overhead beyond what you would write by hand.

---

## Chapter 2: Syntax Reference

### 2.1 Module Structure

A Nanz source file is a **module**: a sequence of top-level declarations.

```
module = (struct_decl | interface_decl | global_decl | fun_decl)*
```

Declarations may appear in any order within a file. There are no imports; linking multiple modules is handled at the assembler level.

Comments:

```nanz
// line comment
/* block comment — may span
   multiple lines */
```

### 2.2 Types

| Syntax | Width | Notes |
|--------|-------|-------|
| `u8`   | 8 bit | unsigned byte — maps to A/B/C/D/E/H/L on Z80 |
| `u16`  | 16 bit | unsigned word — maps to HL/DE/BC on Z80 |
| `i8`   | 8 bit | signed byte (same register, different arithmetic) |
| `i16`  | 16 bit | signed word |
| `bool` | 8 bit | false=0, true=1 |
| `void` | — | return type only |
| `ptr`  | 16 bit | untyped pointer (Z80 address) |
| `^T`   | 16 bit | typed pointer to T (syntax sugar; internally `ptr`) |
| `[T; N]` | N×width(T) | fixed-size array of N elements of type T |
| `u8<lo..hi>` | 8 bit | ranged type — enables LUT generation (see §2.14) |
| `u16<lo..hi>` | 16 bit | ranged type |
| `StructName` | sum of fields | struct type (passed as pointer to function) |

**Pointer types note:** `^u8` and `ptr` are identical at the MIR2 level. The element type in `^T` is parsed and discarded; it exists only for human readability. Index operations (`ptr[i]`) and dereferences (`ptr^`) produce values of type `u8` by default.

### 2.3 Functions

```nanz
fun name(param1: Type1, param2: Type2) -> ReturnType {
    // body
}
```

The keyword `fn` is accepted as an alias for `fun`:

```nanz
fn add(a: u8, b: u8) -> u8 {
    return a + b
}
```

Functions with no return value omit the `-> ReturnType` clause (implicitly `void`):

```nanz
fun clear(buf: ^u8, n: u8) {
    var i: u8 = 0
    while i < n {
        buf[i] = 0
        i = i + 1
    }
}
```

A function body that falls off the end of its last block gets an implicit `return` (safe for void functions; a MIR2 verifier would catch the error for value functions).

### 2.4 Extern Functions

Functions implemented outside the Nanz module (in Z80 assembly, ROM, or another object file) are declared with `@extern`:

```nanz
@extern fun process(x: u8) -> void
@extern fun rom_print(s: ptr) -> void
```

The body is omitted. The assembler will resolve the symbol at link time. `@extern` functions participate fully in the calling convention: the compiler assigns register classes to their parameters as it would for any other function.

### 2.5 Variable Declarations

Two forms:

**`var` — explicit type required, optional initializer:**

```nanz
var i: u8 = 0
var buf: ^u8          // uninitialized (value undefined)
var arr: [u8; 8]      // array on stack (not yet fully supported)
```

**`let` — type inferred from initializer:**

```nanz
let x = 42            // x: u8 (42 fits in u8)
let ptr = @ptr(u8, 0xFE)  // ptr: ptr
let y: u16 = 1000     // explicit type overrides inference
```

Both `var` and `let` introduce a new binding in the current function scope. There is no shadowing detection — the later binding replaces the earlier one in the environment.

**Hardware-mapped variables** (`at` clause):

```nanz
var port: u8 at(0xFE)    // local: references address 0x00FE
global screen: [u8; 6912] at(0x4000)  // global: ZX Spectrum VRAM
```

The `at` clause tells the compiler to treat the variable as a memory-mapped location at the given absolute address. Reads and writes generate `LD (addr), r` and `LD r, (addr)` instructions.

### 2.6 Global Variables

Module-level variables are declared with `global`:

```nanz
global counter: u8
global vram: u8 at(0x4000)
global table: [u8; 4] = [1, 2, 4, 8]
global lut: [u8; 256]
```

Globals are emitted in the `.a80` output's data section. Array globals with `= [...]` initializers get a `DB` directive.

### 2.7 Literals

```nanz
42          // decimal integer — u8 if fits (≤255), u16 otherwise
0xFF        // hex integer
0           // zero — u8
true        // bool literal
false       // bool literal
"hello"     // string — becomes a pointer to a NUL-terminated byte sequence
```

String literals are interned. The expression `"hello"` produces a `ptr` (address of the string's data).

### 2.8 Operators

**Arithmetic** (left-associative, standard precedence):

```nanz
a + b    // add
a - b    // subtract
a * b    // multiply (compiler expands to shifts/adds for constants)
a / b    // divide
a % b    // modulo
```

**Bitwise:**

```nanz
a & b    // AND
a | b    // OR
a ^ b    // XOR (when used as infix — ^ is also the prefix/postfix deref operator)
~a       // bitwise NOT (unary)
a << n   // left shift
a >> n   // right shift
```

**Comparison (produce `bool`):**

```nanz
a == b   a != b
a <  b   a <= b
a >  b   a >= b
```

**Logical:**

```nanz
!a       // logical NOT (unary)
```

**Precedence table** (highest to lowest):

| Level | Operators |
|-------|-----------|
| 8 | `*` `/` `%` |
| 7 | `+` `-` |
| 6 | `<<` `>>` |
| 5 | `<` `<=` `>` `>=` |
| 4 | `==` `!=` |
| 3 | `&` |
| 2 | `^` (XOR) |
| 1 | `\|` |

Parentheses override precedence: `(a + b) * c`.

### 2.9 Pointer Operations

**Dereference (load)** — postfix `^`:

```nanz
let val = ptr^    // load the byte at address ptr
```

**Store through pointer** — `^expr = expr` (prefix `^` on the left side):

```nanz
^ptr = 42         // store 42 at address ptr
```

**Index (load)** — `ptr[i]`:

```nanz
let b = buf[3]    // load byte at buf+3
```

**Index store:**

```nanz
buf[i] = 0        // store 0 at buf+i
```

**Address-of** — `&name`:

```nanz
let p = &counter  // p is a ptr to the global `counter`
```

**Typed constant pointer** — `@ptr(T, addr)`:

```nanz
let keyboard = @ptr(u8, 0xFE)  // keyboard port at I/O address 0xFE
let val = keyboard^             // read the port
keyboard^ = 255                 // write the port
```

`@ptr` is the idiomatic way to reference hardware registers and ROM routines at absolute addresses.

### 2.10 Casts

```nanz
u8(expr)    // truncate or extend to u8
u16(expr)   // zero-extend to u16
i8(expr)    // reinterpret as signed 8-bit
i16(expr)   // reinterpret as signed 16-bit
```

Casts are explicit — there is no implicit widening. This is intentional: on Z80, widening a `u8` to `u16` requires extra instructions (`LD H, 0` / `LD L, A`), and the compiler will not hide that cost.

```nanz
fun sum_bytes(buf: ^u8, len: u8) -> u16 {
    var total: u16 = 0
    for b: u8 in buf[0..len] {
        total = total + u16(b)   // explicit zero-extend
    }
    return total
}
```

### 2.11 Control Flow

**if / else:**

```nanz
if condition {
    // then branch
} else {
    // else branch (optional)
}
```

**while:**

```nanz
while condition {
    // body
}
```

**for range** (integer loop, inclusive start, exclusive end):

```nanz
for i in 0..n {
    // i: u8, iterates 0, 1, ..., n-1
}
```

The loop variable `i` is inferred as `u8` if `n` fits in a byte. Use `u16` literals or casts for larger ranges.

**for each** (sequential scan over a memory region):

```nanz
for x: u8 in buf[0..n] {
    // x is loaded from buf[0], buf[1], ..., buf[n-1]
    // HL advances sequentially — no index register needed
}
```

This compiles to a DJNZ-based loop that keeps a pointer in HL and a counter in B, never computing `buf + i` explicitly.

**break** and **continue:**

```nanz
while true {
    if done { break }
    if skip { continue }
    // body
}
```

**return:**

```nanz
return         // for void functions
return expr    // for value functions
```

**switch:**

```nanz
switch value {
    case 0: return 10
    case 1: return 20
    default: return 0
}
```

Cases do not fall through. Each case body is terminated when the next `case`, `default`, or closing `}` is encountered.

### 2.12 Structs

```nanz
struct Point {
    x: u8
    y: u8
}

struct Vec3d {
    x: u16
    y: u16
    z: u8
}
```

Fields are laid out sequentially with no padding. Field byte offsets are computed at parse time:
- `Point.x` → offset 0
- `Point.y` → offset 1
- `Vec3d.x` → offset 0 (2 bytes)
- `Vec3d.y` → offset 2 (2 bytes)
- `Vec3d.z` → offset 4 (1 byte)

**Field access:**

```nanz
fun get_y(p: Point) -> u8 {
    return p.y    // compiles to: load from base+1
}
```

Struct values are always passed **by pointer** to functions (HL on Z80). Field reads are translated to `Load(ptr + offset)`. Field writes are `Store(ptr + offset, val)`.

### 2.13 Struct Methods

Methods are declared with `TypeName.methodName` syntax:

```nanz
struct Vec2 {
    x: u8
    y: u8
}

fun Vec2.add(self: Vec2, other: Vec2) -> Vec2 {
    // self and other are received as pointers (HL, DE on Z80)
    return self
}

fun Vec2.scale(self: Vec2, factor: u8) -> Vec2 {
    return self
}
```

The compiler stores these as top-level functions with mangled names: `Vec2_add`, `Vec2_scale`. There is no hidden vtable.

**Calling methods via UFCS:**

```nanz
fun compute(a: Vec2, b: Vec2) -> Vec2 {
    return a.add(b)    // UFCS: rewritten to Vec2_add(a, b)
}
```

UFCS (Uniform Function Call Syntax) rewrites `receiver.method(args...)` to `method(receiver, args...)`. The compiler consults the method table at parse time to resolve the mangled name and return type.

### 2.14 Operator Overloading

```nanz
fun +(a: Vec2, b: Vec2) -> Vec2 {
    return a    // body must be filled in properly
}

fun -(a: Vec2, b: Vec2) -> Vec2 {
    return a
}

fun compute(a: Vec2, b: Vec2) -> Vec2 {
    return a + b    // dispatched to op_add(a, b) because a is Vec2
}
```

Operator overloads are registered globally. The compiler checks at each binary expression site: if the left operand is a struct type and that operator has a registered overload, the expression becomes a function call. For primitive types (`u8 + u8`), the built-in binary expression is always used regardless of any overloads in scope.

Overloadable operators: `+` `-` `*` `/` `%` `==` `!=` `<` `<=` `>` `>=` `&` `|` `^`.

Mangled names: `op_add`, `op_sub`, `op_mul`, `op_div`, `op_rem`, `op_eq`, `op_ne`, `op_lt`, `op_le`, `op_gt`, `op_ge`, `op_and`, `op_or`, `op_xor`.

### 2.15 Interfaces

```nanz
interface Animal {
    speak
    move
}

struct Dog {
    name: u16
}

fun Dog.speak(self: Dog) -> void {
    // implementation
}
```

Interface declarations capture the set of method names. No vtable is generated. At each call site `animal.speak()`, the compiler resolves `animal`'s concrete type at compile time and emits a direct `CALL Dog_speak`. This is **static dispatch** with zero overhead — the same mechanism as C++ templates or Rust monomorphization, but simpler: the Nanz compiler currently requires the concrete type to be statically known.

### 2.16 Lambdas

```nanz
let double = |x: u8| x * 2

let add = |a: u8, b: u8| a + b

let greet = |x: u8| {
    process(x)
    return x + 1
}
```

Parameter types default to `u8` if omitted: `|x| x + 1` is valid.

Each lambda becomes a top-level function named `lambda_N` (N is a sequential counter). **Capture rules** depend on context:

- **Fused iterator lambdas** (`forEach`/`map`/`filter` chains that get inlined): may read and write outer local variables. The compiler detects free variables, skips standalone lowering of the lambda, and threads the captured vars as SSA block params through the DJNZ loop — zero spill, zero heap.
- **Non-fused lambdas** (passed as function pointers): cannot capture outer locals (no runtime frame to address them). Only globals are accessible.

```nanz
// |x: u8| x * 2  becomes:
fun lambda_0(x: u8) -> u8 {
    return x * 2
}
```

The reference to `lambda_0` at the call site is a direct function reference — zero allocation, no closure struct.

**Lambdas as arguments to iterators** (the primary use case):

```nanz
buf.map(|x: u8| x * 2).filter(|x: u8| x > threshold).forEach(|x: u8| process(x), n)
```

The iterator chain fusion optimizer (see Chapter 5) inlines these lambdas directly into the generated DJNZ loop, eliminating all function call overhead.

### 2.17 Ranged Types

```nanz
fun popcount(x: u8<0..255>) -> u8 { ... }
fun clamp(x: u8) -> u8<0..63> { return 0 }
fun table(i: u16<0..1023>) -> u16 { return 0 }
```

`u8<lo..hi>` is a ranged type where `lo` and `hi` are inclusive integer bounds. Internally, hi is stored as exclusive (hi+1) following Go convention, but the source syntax is always inclusive.

Ranged types enable the **LUTGen** optimization pass (§4.5): if a function has a single parameter of ranged type where the range fits in ≤256 values, a base type of `u8`, and a return type of `u8`, the compiler evaluates the function at compile time for all inputs and replaces the body with a 3-instruction table lookup.

---

## Chapter 3: Semantics and Type System

### 3.1 Type Inference

Nanz uses **local type inference**: the type of a `let` binding is inferred from its initializer expression. There is no global Hindley-Milner inference — function parameters and global variables require explicit types.

Numeric literals are typed as `u8` if their value is in [0, 255], `u16` otherwise.

The result type of a binary expression:
- Comparison operators (`==`, `!=`, `<`, etc.) produce `bool`.
- If either operand is `u16`, the result is `u16`.
- If either operand is `ptr`, the result is `ptr`.
- Otherwise, the result is the left operand's type.

### 3.2 Calling Convention

The HIR lowerer assigns Z80 register classes to function parameters based on position and type:

| Parameter position | Type | Z80 register class |
|-------------------|------|--------------------|
| 0 | `u8` / `i8` / `bool` | **ClassAcc** (A) |
| 0 | `u16` / `i16` | **ClassPointer** (HL) |
| 0 | `ptr` or struct | **ClassPointer** (HL) |
| 1 | any 8-bit | **ClassGeneral** (C/D/E) |
| 1 | `u16` | **ClassIndex** (DE) |
| 2 | any 8-bit | **ClassCounter** (B) |
| ≥3 | any | **ClassGeneral** |

Return values:
- 8-bit: **ClassAcc** (A)
- 16-bit or pointer: **ClassPointer** (HL)

This convention is Z80-aware: the accumulator (A) is the only register that can participate in ALU operations, so functions that receive an 8-bit value as their first argument can start using it immediately without a `LD A, r` move.

**Interprocedural refinement:** The MIR2 contract optimizer (Phase 5b) analyzes the call graph and refines these initial class assignments. If a function's parameter is always used as a loop counter (fed into DJNZ), the optimizer can promote it to ClassCounter (B) even if it is position-1, saving a `LD B, C` move at the call site.

### 3.3 SSA-Like Variable Semantics

The HIR lowerer implements "almost SSA": each assignment to a local variable creates a new virtual register, and variables that are live across loop or branch boundaries are promoted to **block parameters**.

For a while loop:

```nanz
var i: u8 = 0
var s: u8 = 0
while i < n {
    s = s + buf[i]
    i = i + 1
}
```

The lowerer detects that both `i` and `s` are mutated in the loop body, and also that `n` is read in the loop condition. All three become block parameters of the `loop_head` block. On each back-edge (end of body → head), the updated values are passed as block arguments.

This design avoids a full dominance-frontier computation while still correctly handling all structured control flow.

### 3.4 Structs are Pointer Values

When a struct is passed to a function, the compiler passes its **address**, not a copy. This is the correct behavior for Z80: copying a struct would require LDIR or a manual loop, and for typical small structs (2-8 bytes) it is always faster to pass a pointer.

The `self` parameter in a method receives the address of the struct in HL. Reading a field `self.x` (offset 0) is `LD A, (HL)`; reading `self.y` (offset 1) is `INC HL; LD A, (HL)` or `LD A, (IY+1)` depending on register pressure.

---

## Chapter 4: Compilation Deep Dive — End to End

### 4.1 Example: Simple Addition

**Source:**

```nanz
fun add(a: u8, b: u8) -> u8 {
    return a + b
}
```

**HIR (conceptual):**

```
Func add(a: u8 @ ClassAcc, b: u8 @ ClassGeneral) -> u8 @ ClassAcc:
  entry:
    r0 = BinExpr(+, a, b)  [u8, ClassGeneral]
    Ret(r0)
```

**MIR2 (conceptual):**

```
add(%r0: u8[Acc], %r1: u8[General]) -> u8[Acc]:
  entry:
    %r2 = Add %r0, %r1  [u8, Acc]
    Ret %r2
```

**Z80 output:**

```asm
add:
    ADD A, C    ; a(A) + b(C) → A
    RET
```

Two instructions. The parameter convention (`a` in A, `b` in C) enables the Z80 `ADD A, C` instruction directly — no register moves needed.

### 4.2 Example: While Loop with Array Access

**Source (`examples/nanz/01_sum_array.nanz`):**

```nanz
fun sum_array(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    var i: u8 = 0
    while i < n {
        s = s + ptr[i]
        i = i + 1
    }
    return s
}
```

**Z80 output (`examples/nanz/01_sum_array.a80`):**

```asm
sum_array:
    LD D, 0
    LD E, 0
    LD A, E
    LD E, D
    LD D, C
    LD C, A
.sum_array_loop_head1:
    LD A, C
    CP D
    JRS NC, .sum_array_loop_exit3
.sum_array_loop_body2:
    LD E, C        ; i → E
    LD D, 0
    EX DE, HL
    PUSH DE
    POP BC
    ADD DE, BC     ; ptr + i → DE
    LD H, (DE)     ; load byte
    LD A, E
    ADD A, H       ; s + element
    LD E, A
    INC C          ; i++
    JRS .sum_array_loop_head1
.sum_array_loop_exit3:
    LD A, E
    RET
```

This output uses random-access indexing (`ptr[i]` requires `ADD DE, BC` each iteration). The idiomatic Nanz alternative (§4.3) avoids this.

### 4.3 Example: For-Each Loop (Sequential Scan)

**Source (`examples/nanz/02_sum_array_idiomatic.nanz`):**

```nanz
fun sum_array_idiomatic(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    for x: u8 in ptr[0..n] {
        s = s + x
    }
    return s
}
```

**Conceptual Z80 output** (the ForEachStmt lowers to a DJNZ loop):

```asm
sum_array_idiomatic:
    LD B, C        ; counter = n (in C from calling convention)
    ; HL already holds ptr
    LD E, 0        ; s = 0
.fe_head:
    LD A, (HL)     ; x = *ptr
    INC HL         ; advance pointer
    ADD A, E       ; s + x → A
    LD E, A        ; s = result
    DJNZ .fe_head
    LD A, E        ; return s
    RET
```

The sequential scan uses `INC HL` (6T) instead of computing `ptr + i` (11T for `ADD HL, DE` plus setup). For 100 elements, that saves approximately 500 T-states — a meaningful difference at 3.5 MHz.

### 4.4 Example: Iterator Chain

**Source (`examples/nanz/03_filter_map_chain.nanz`):**

```nanz
@extern fun process(x: u8) -> void

fun filter_map_forEach(buf: u16, n: u8, threshold: u8) -> void {
    buf.map(|x: u8| (x * 2)).filter(|x: u8| (x > threshold)).forEach(|x: u8| { process(x) }, n)
}
```

**Conceptual Z80 output** (all three stages fused into one DJNZ loop):

```asm
filter_map_forEach:
    LD B, C        ; counter = n
    ; HL = buf, D = threshold
.loop:
    LD A, (HL)     ; x = *buf (load element)
    INC HL         ; advance pointer
    ADD A, A       ; x * 2 (map body, no CALL)
    CP D           ; compare with threshold+1
    JR NC, .skip   ; filter: skip if not > threshold
    CALL process   ; forEach body
.skip:
    DJNZ .loop
    RET
```

No intermediate array. No calls to lambda functions. Three stages, one loop — this is the key performance win from iterator chain fusion.

### 4.5 Example: LUT via Ranged Types

**Source (`examples/nanz/04_lut_popcount.nanz`):**

```nanz
fun popcount(x: u8<0..255>) -> u8 {
    var n: u8 = 0
    var v: u8 = x
    while v != 0 {
        n = n + (v & 1)
        v = v >> 1
    }
    return n
}
```

The `u8<0..255>` annotation tells LUTGen: this function is pure (no I/O, no globals written), the input fits in a byte, and the full domain [0, 255] is covered. The compiler evaluates `popcount(0)` through `popcount(255)` using the MIR2 virtual machine at compile time, then replaces the function body with a table lookup.

**Z80 output (`examples/nanz/04_lut_popcount.a80`, actual output):**

```asm
popcount:
    LD H, popcount_lut^H  ; 7T — load page base (high byte only; ^H = addr>>8)
    LD L, C               ; 4T — L = x (parameter in C via contract opt)
    LD A, (HL)            ; 7T — A = popcount_lut[x]
    RET

; globals
    ALIGN 256       ; page-aligned for fast path
popcount_lut:
    DB 0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, ...
```

The 256-entry table is aligned to a 256-byte boundary. Since `ALIGN 256` guarantees the low byte of `popcount_lut`'s address is always zero, only the **high byte** (the page base) needs loading — `LD H, popcount_lut^H` (7T) instead of `LD HL, popcount_lut` (10T). Then `LD L, C` sets the index. This is the **page-aligned fast path**: **18 T-states** (body only; 7+4+7) vs ~70-150 T-states for the iterative loop version. (`^H` is MZA's high-byte extraction suffix, equivalent to `(address >> 8) & 0xFF`.)

Without the range annotation, `popcount(x: u8)` compiles to a real loop.

### 4.6 Example: Struct with Methods

**Source:**

```nanz
struct Color {
    r: u8
    g: u8
    b: u8
}

fun Color.brightness(self: Color) -> u8 {
    return self.r + self.g + self.b
}

fun main() -> void {
    var c: Color
    c.r = 100
    c.g = 150
    c.b = 200
    let b = c.brightness()
}
```

**What the compiler does:**
1. `Color.brightness` is stored as `Color_brightness(self: Color) -> u8`.
2. `c.brightness()` is rewritten by UFCS to `Color_brightness(c)`.
3. `c` is passed as its address (HL points to the Color struct's memory).
4. `self.r` → `LD A, (HL)` (offset 0).
5. `self.g` → load at HL+1.
6. `self.b` → load at HL+2.

---

## Chapter 5: Iterator Chains

### 5.1 The Three Core Iterators

Nanz supports three chaining methods on pointers/arrays:

| Method | Signature | Semantics |
|--------|-----------|-----------|
| `ptr.map(lambda)` | `ptr map(lambda) -> ptr` | Transform each element |
| `ptr.filter(lambda)` | `ptr filter(lambda) -> ptr` | Keep elements where lambda returns true |
| `ptr.forEach(lambda, n)` | `void forEach(lambda, n)` | Execute lambda for each of n elements |

These are not runtime library functions. The parser recognizes `.map(...)`, `.filter(...)`, and `.forEach(...)` as method-call syntax on pointer expressions, and the HIR lowerer's `tryLowerIterChain` function detects the chain pattern before emitting any MIR2 instructions.

### 5.2 Fusion: How It Works

When the lowerer encounters a statement like:

```nanz
arr.map(|x| f(x)).filter(|x| g(x)).forEach(|x| h(x), n)
```

it recognizes this as an iterator chain and fuses all stages into a single `ForEachStmt` with a synthesized body:

```
for x in arr[0..n]:
    let mapped = f(x)      // map body
    if g(mapped) {         // filter: skip if false
        h(mapped)          // forEach body
    }
```

This single `ForEachStmt` then lowers to a DJNZ loop with:
- B = counter (n, decremented by DJNZ)
- HL = pointer (INC HL each iteration)
- A = current element

Lambda bodies are inlined directly — the `lambda_N` functions still exist in the module (the parser generates them), but the lowerer substitutes their bodies at the call site when fusing. No `CALL lambda_N` instruction is ever emitted.

### 5.3 T-State Analysis

For a single `forEach` over n elements with a simple body (e.g., `CALL process`):

| Instruction | T-states |
|------------|----------|
| LD A, (HL) | 7T |
| INC HL | 6T |
| CALL process | 17T |
| DJNZ | 13T (taken) |
| **Total per element** | **~43T** |

Compare with the indexed access version (without chain fusion, using `ptr[i]`):

| Instruction | T-states |
|------------|----------|
| LD HL, buf | 10T |
| ADD HL, DE | 11T (DE = i) |
| LD A, (HL) | 7T |
| INC DE | 6T |
| CALL process | 17T |
| DJNZ | 13T |
| **Total per element** | **~64T** |

The fusion saves approximately 21 T-states per element — nearly 33%. At 3.5 MHz with 100 elements, that is ~6 µs per call, ~60 ms per second of 10,000-element arrays.

### 5.4 Filter Compilation

A filter inverts its predicate and uses a conditional jump:

```nanz
arr.filter(|x: u8| (x > 5)).forEach(|x: u8| process(x), n)
```

The filter body `x > 5` is a comparison. After inversion (`x <= 5`, i.e., `x < 6`), the generated code is:

```asm
    LD A, (HL)
    INC HL
    CP 6          ; if x < 6 (original filter: x > 5 false)
    JR C, .skip   ; skip the forEach body
    CALL process
.skip:
    DJNZ .loop
```

### 5.5 Current Limitations

- **Fused-lambda capture (supported):** When a lambda is used directly inside a fused iterator chain (`forEach`/`map`/`filter`), it may reference and mutate outer local variables. The HIR lowerer detects the free variable, threads it as a loop-carried block param, and eliminates the standalone lambda function. Example: `buf.forEach(|x: u8| { s = s + x }, n)` with `s` declared in the enclosing function — `s` is threaded as a register (not spilled to memory) through the DJNZ loop. This is implemented as **block-param threading**, not heap closure allocation.

- **Non-fused closure capture (not supported):** Lambdas passed as function pointers to non-inlinable functions cannot capture outer locals — there is no runtime frame to address them. Only globals (compile-time addresses) are accessible from non-inlined callbacks. A compile-time error is planned; currently the behavior is undefined.

- **`enumerate` is broken on Z80:** The `enumerate` combinator (index + value) conflicts with the DJNZ counter in B. This is a known bug (ADR-0006).

- **`reduce` is broken on Z80:** The accumulator register A is overwritten between the two SMC (self-modifying code) parameters in the reduction pattern.

- **Non-zero-start ranged LUT:** Functions with ranged types where lo > 0 have a known contract optimization bug that can cause class mismatches.

---

## Chapter 6: Zero-Cost Abstractions on Z80

### 6.1 UFCS

`obj.method(args)` desugars to `method(obj, args)` at **parse time**, with no runtime representation of the method table. The parse-time method table is a `map[structName]map[methodName]methodInfo` that maps to the mangled function name. By the time MIR2 is generated, all UFCS calls are direct `CallExpr` nodes.

**Cost: zero.** The generated `CALL Vec2_add` is identical to what you would write by hand.

### 6.2 Interfaces

```nanz
interface Animal {
    speak
    move
}
```

Interfaces are declaration-only. They record which methods a type is expected to implement, but no vtable, no fat pointer, no method dispatch table is emitted. When the compiler sees `animal.speak()` where `animal: Dog`, it resolves this to `Dog_speak(animal)` at compile time.

**Cost: zero.** Same `CALL Dog_speak` as calling the function directly.

**Current limitation:** The compiler requires that the concrete type of the receiver is statically known. Dynamic dispatch (calling through an interface where the concrete type is not known at compile time) is not implemented. This is consistent with the MinZ philosophy: no runtime overhead.

### 6.3 Lambdas

Each lambda `|x: u8| expr` becomes a top-level function `lambda_N`. The reference in the source code (`let cb = |x| x+1`) becomes a VarRefExpr pointing to `lambda_N`. When the lambda is called:

- If called directly via iterator chain fusion, the body is inlined into the loop — zero call overhead.
- If called through a variable (`cb(v)`), it compiles to `CALL lambda_N` — same cost as any function call.

There is no closure struct. There is no heap allocation. There is no function pointer indirection beyond a standard Z80 `CALL` instruction.

### 6.4 Struct Methods

`fun Vec2.add(self: Vec2, other: Vec2) -> Vec2` stores as the top-level function `Vec2_add`. Name mangling is the only overhead. The Z80 calling convention assigns `self` to HL (first pointer parameter), `other` to... the second parameter position.

Struct methods are indistinguishable from free functions in the generated assembly.

---

## Chapter 7: Relation to MinZ and PL/M

### 7.1 The Two Frontends

The MinZ compiler repository contains two independently maintained source language parsers:

| Language | Extension | Parser | Backend | Status |
|----------|-----------|--------|---------|--------|
| MinZ | `.minz` | `pkg/parser/participle/` | MIR1 → old codegen | Frozen — works, not developed |
| Nanz | `.nanz` | `pkg/nanz/parse.go` | HIR → MIR2 → Z80 | **Active development** |
| PL/M-80 | `.plm` | `pkg/plm/` | HIR → MIR2 → Z80 | Active (100% of Intel 80 Tools parse) |

The router is in `cmd/minzc/main.go`:

```go
if ext == ".plm" || ext == ".nanz" {
    return compileViaHIR(src, ext)
} else {
    // old MIR1 path for .minz
}
```

### 7.2 When to Use Which

**Use Nanz when:**
- Writing new Z80 or CP/M programs from scratch.
- You want modern syntax: for-each, iterators, struct methods, operator overloading.
- You need LUT generation or interprocedural register allocation.
- You are testing or developing the MIR2 backend.

**Use MinZ when:**
- You have existing `.minz` programs that compile and work.
- You need features that Nanz does not yet have (some MinZ-specific metafunctions like `@define`, `@print`).

**Use PL/M-80 when:**
- You are porting legacy CP/M software.
- You have existing PL/M source you want to modernize through the new backend.
- You want the exact PL/M-80 language semantics (DO WHILE, PROCEDURE, END, etc.).

### 7.3 Feature Gaps

Nanz currently lacks some features present in MinZ:

- `@define` preprocessor macros
- `@print` optimized string output
- `@extern FFI` for ROM calls (Nanz has `@extern fun` but not `@extern` for arbitrary symbols)
- Ruby-style string interpolation

These are planned additions as Nanz matures.

---

## Chapter 8: Comparison with Other Languages

### 8.1 Nanz vs. C

A C bit-counting loop:

```c
uint8_t popcount(uint8_t x) {
    uint8_t n = 0;
    while (x) {
        n += x & 1;
        x >>= 1;
    }
    return n;
}
```

The Nanz version with range annotation:

```nanz
fun popcount(x: u8<0..255>) -> u8 {
    var n: u8 = 0
    var v: u8 = x
    while v != 0 {
        n = n + (v & 1)
        v = v >> 1
    }
    return n
}
```

The C version compiles to a loop (~70-150 T-states depending on the Z80 C compiler). The Nanz version compiles to a 3-instruction table lookup (21 T-states) because the range annotation enables LUTGen.

C has no equivalent optimization without manual table declarations.

### 8.2 Nanz vs. PL/M-80

The same sum-of-array computation in PL/M-80:

```plm
SUM_ARRAY: PROCEDURE (PTR, N) BYTE;
    DECLARE (PTR) ADDRESS, (N, I, S) BYTE;
    S = 0;
    I = 0;
    DO WHILE I < N;
        S = S + MEMORY(PTR + I);
        I = I + 1;
    END;
    RETURN S;
END SUM_ARRAY;
```

In Nanz:

```nanz
fun sum_array(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    var i: u8 = 0
    while i < n {
        s = s + ptr[i]
        i = i + 1
    }
    return s
}
```

Both compile through HIR → MIR2 → Z80 and produce equivalent assembly. Nanz wins on readability; PL/M-80 wins on compatibility with legacy codebases.

The idiomatic Nanz version further beats the PL/M-80 version:

```nanz
fun sum_array(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    for x: u8 in ptr[0..n] {
        s = s + x
    }
    return s
}
```

The for-each loop avoids `ptr[i]` random-access indexing and compiles to a DJNZ loop with `INC HL` — saving ~21 T-states per element compared to the explicit index version.

### 8.3 Nanz vs. Hand-Written Z80 Assembly

A hand-optimized sum-of-array in Z80 assembly:

```asm
sum_array:
    ; Input: HL = ptr, B = n
    ; Output: A = sum
    LD A, 0         ; s = 0
    LD C, A         ; save s in C
.loop:
    LD A, (HL)      ; x = *ptr
    INC HL
    ADD A, C        ; s += x
    LD C, A
    DJNZ .loop
    LD A, C         ; return s
    RET
```

This is ~8 instructions, ~43 T-states per element. The Nanz `for x: u8 in ptr[0..n]` version produces assembly of comparable quality. The compiler may emit one or two additional register moves if virtual registers are spilled to the $F0xx memory region, but with PBQP register allocation this is increasingly rare for small functions.

The real advantage of Nanz over assembly is not performance — it is **composability**. Iterator chains, struct methods, and operator overloading let you build larger programs without losing track of register assignments, while the compiler ensures the generated code remains correct.

---

## Chapter 9: Z80 Extern Functions and Register Contracts

### 9.1 The @extern Annotation

`@extern` declares a function whose body lives outside the Nanz module — in Z80 assembly, ROM, or a separately assembled file. The simplest form:

```nanz
@extern fun print_char(c: u8) -> void
@extern fun rom_print(s: ptr) -> void
```

The compiler assigns register classes to the parameters and return value exactly as it would for any Nanz function. `print_char(c: u8)` will get `ClassAcc` (A) for `c` and `ClassPointer` (HL) for a pointer, following the standard register-first calling convention.

### 9.2 Annotated @extern: Machine-Specific Register Contracts

Some external functions demand **specific** physical registers — ROM routines, BIOS calls, OS syscalls, and RST handlers all have fixed ABIs baked into their implementation. You cannot move them.

The extended `@extern` syntax lets you annotate parameter and return register classes explicitly:

```nanz
@extern("rst28", params=[z80_a], returns=[z80_a])
fun rst28_sqrt(x: u8) -> u8

@extern("bdos", params=[z80_c, z80_de], returns=[z80_a])
fun bdos_call(fn_code: u8, param: u16) -> u8

@extern("set_palette", params=[z80_hl, z80_bc])
fun set_palette(ptr: u16, count: u16) -> void
```

The first string argument is the symbol name in the linker output. The `params=` and `returns=` lists map left-to-right to the function's parameter and return types.

### 9.3 Z80 Register Class Aliases

The `z80_*` aliases are readable names for MIR2 register classes, usable in `@extern` annotations:

| Alias | MIR2 class | Z80 physical | Typical use |
|-------|-----------|-------------|-------------|
| `z80_a` | `ClassAcc` | A | 8-bit ALU result, first u8 param/return |
| `z80_b` | `ClassCounter` | B | DJNZ counter, loop variables |
| `z80_c` | `ClassGeneral` (C slot) | C | Second 8-bit param |
| `z80_hl` | `ClassPointer` | HL | Pointer, u16 param/return |
| `z80_de` | `ClassIndex` | DE | Second u16 param, index register |
| `z80_bc` | `ClassCounter16` | BC | 16-bit counter / loop bound |

These names exist for human readability. Inside the compiler they map to MIR2 `RegClass` values. The calling convention constraint becomes a hard edge in the PBQP interference graph — the allocator cannot reassign the parameter to another register class.

**Effect on the caller:** If the compiled Nanz code holds `x` in C (from some prior allocation), and you call `rst28_sqrt(x)` with `params=[z80_a]`, the compiler emits `LD A, C` before the call. If `x` is already in A, the move is elided. The function's contract annotation gives the compiler the information it needs to generate the correct prefix code.

### 9.4 Annotated @extern Example: ZX Spectrum ROM Calls

The ZX Spectrum ROM is full of routines with fixed register ABIs. In Nanz:

```nanz
// CLS — clear screen.  No parameters.
@extern("$0DAF")
fun cls() -> void

// PRINT-A — print character in A to lower screen.
@extern("$0010", params=[z80_a])
fun print_char(c: u8) -> void

// PIXEL-ADD — convert (y=H, x=L) to HL=screen address, B=pixel mask.
@extern("$22AA", params=[z80_hl], returns=[z80_hl])
fun pixel_addr(yx: u16) -> u16
```

The first string argument is a symbol or absolute address (prefixed `$` for hex). The assembler/linker resolves it.

### 9.5 RST Instructions and Inline-Parameter ABIs

Several Z80 systems use RST (restart) instructions in a non-standard way: a **function code byte** is placed immediately after the `RST` instruction in the instruction stream, rather than in a register:

```asm
RST 8       ; call OS
DB  0x42    ; function code — in the instruction stream, not in A/B/C/HL
```

The called routine reads `(SP)` to find the opcode stream pointer, extracts the DB byte, increments the return address past it, then dispatches.

**This is not a register-passing ABI.** The parameter is embedded in the instruction stream — there is no register that holds it before the call. `@extern` cannot model this: `@extern` constrains which physical register a Nanz value lives in, but the function-code byte does not correspond to any Nanz value.

#### The Right Abstraction: Metafunction or Inline ASM

**Option 1: `@rst` metafunction (recommended)**

```nanz
// Expands to: RST 8 ; DB code
@rst(8, 0x42)          // OS call with function code 0x42

// With a Nanz helper:
fun os_call(code: u8) -> void {
    // This requires @asm; the code byte is a compile-time literal.
    // Use @rst metafunction instead:
}
```

**Option 2: Inline `@asm` block**

```nanz
fun dos_open(filename: u8) -> void {
    @asm {
        RST 8
        DB  0x66        // DOS OPEN function code
    }
}
```

**Option 3: Per-function wrappers (always safe)**

```nanz
// One Nanz function per RST code:
@extern("_rst8_open")   // defined in a hand-written asm file
fun dos_open(filename_ptr: ptr) -> u8

@extern("_rst8_close")
fun dos_close(handle: u8) -> u8
```

The wrappers in assembly:
```asm
_rst8_open:    ; HL = filename_ptr (from ClassPointer ABI)
    RST  8
    DB   0x66  ; OPEN
    RET

_rst8_close:   ; A = handle (from ClassAcc ABI)
    RST  8
    DB   0x69  ; CLOSE
    RET
```

This separates the "call with inline literal" awkwardness from Nanz — each wrapper is a tiny asm stub, and the Nanz side sees a clean function with a normal register ABI.

**Recommendation:** Use Option 3 (asm wrappers) for any RST-with-DB routine. It is the most maintainable, the most compiler-friendly, and it lets the PBQP allocator optimize callers normally. Reserve `@asm` blocks for truly one-off sequences where writing a separate function is overkill.

---

## Chapter 10: Deep Optimization: CondRetSink and Flag Fusion

This chapter walks through the complete optimization pipeline that transforms a naive `abs_diff` implementation into the tightest possible Z80 assembly. It demonstrates five MIR2 passes working in concert, and explains what happens when the same function operates on `u8` vs `u16` values.

### 10.1 The Example: abs_diff

```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a < b {
        return b - a
    }
    return a - b
}
```

Naive Z80 output (before optimization):

```asm
abs_diff:          ; a=A, b=B (from PBQP contract)
    LD C, A        ; save a
    CP B           ; compare a vs b
    JR NC, .else
.then:
    LD A, B        ; b
    SUB C          ; b - a
    RET
.else:
    LD A, C        ; a
    SUB B          ; a - b
    RET
```

8 instructions, ~28-40 T-states. The optimal output is 4 instructions:

```asm
abs_diff:
    SUB B          ; A = a - b, carry = (a < b unsigned)
    RET NC         ; if a >= b: return a-b
    NEG            ; A = -(a-b) = b-a
    RET
```

Getting there requires five passes.

### 10.2 Pass 1: CondRetSink (`mir2/condret.go`)

**What it does:** Finds a `BrIf(cond, @then, @else)` where the `@else` branch is a trivial exit (no parameters, single predecessor, all-pure instructions, terminates with `Ret`). Hoists `@else`'s instructions into the current block and replaces the `BrIf` with `TermCondRet`.

**MIR2 before:**
```
block @entry:
    %cond = cmp.ult %a, %b
    br_if %cond, @then(), @else()

block @else:
    %r2 = sub %a, %b
    ret %r2

block @then:
    %r1 = sub %b, %a
    ret %r1
```

**MIR2 after:**
```
block @entry:
    %cond = cmp.ult %a, %b
    %r2   = sub %a, %b          ← hoisted from @else
    cond_ret %cond, [%r2], @then()

block @then:                     ← unchanged
    %r1 = sub %b, %a
    ret %r1
```

`TermCondRet` semantics: if `%cond == 0` → return `[%r2]`; if `%cond != 0` → jump `@then`. Z80 emits `RET <invertedCC>`.

**Z80 after this pass:**

```asm
abs_diff:
    CP B           ; cmp.ult(a, b): sets carry = (A < B)
    SUB B          ; r2 = a - b (hoisted; may clobber flags)
    RET NC         ; if a >= b: return r2
    ; @then: r1 = b - a
    LD A, B
    SUB C          ; (A was clobbered; C holds original a)
    RET
```

Progress: eliminated one conditional branch and one label.

### 10.3 Pass 2: SubSwapNeg (`applySubSwapNeg` in `condret.go`)

Fired by CondRetSink immediately after hoisting.

**What it does:** If a hoisted `sub(%a, %b)` was placed in the current block, and the then-block contains `sub(%b, %a)` (reversed operands), replace the then-block instruction with `neg(%r2)`. This is correct because `neg(a-b) = -(a-b) = b-a`.

**MIR2 after:**
```
block @entry:
    %cond = cmp.ult %a, %b
    %r2   = sub %a, %b
    cond_ret %cond, [%r2], @then()

block @then:
    %r1 = neg %r2               ← was: sub %b, %a
    ret %r1
```

The `neg` result is forced to `ClassAcc` (A register), because Z80's `NEG` instruction always reads and writes A.

**Z80 after this pass:**

```asm
abs_diff:
    CP B           ; cmp.ult
    SUB B          ; r2 = a - b
    RET NC
    NEG            ; r1 = -r2 = b - a
    RET
```

Five instructions. The `CP B` is now redundant — `SUB B` sets carry in the same way.

### 10.4 Pass 3: hoistReorderSubBeforeCmp

**What it does:** Finds a `cmp.uolt(x, y)` at position i and a `sub(x, y)` at a later position j in the same block. If no instruction between i and j redefines x or y, moves the Sub to position i (just before the Cmp) and converts the Cmp to `cmp.sub_carry(r_sub, y)`.

**Semantics of `CmpSubCarry`:** Given `r = x − y` (unsigned), `CmpSubCarry(r, y) ≡ (x < y unsigned) ≡ carry flag from the preceding `SUB``. No Z80 instruction is emitted — the carry is already in the flags register from the `SUB`.

**MIR2 after:**
```
block @entry:
    %r2   = sub %a, %b              ← moved before cmp
    %cond = cmp.sub_carry %r2, %b   ← carry from the SUB above
    cond_ret %cond, [%r2], @then()
```

**Z80 after this pass:**

```asm
abs_diff:
    SUB B          ; r2 = a − b, carry = (a < b)   [CP B eliminated!]
    RET NC         ; if a >= b: return r2
    NEG            ; r1 = -(a-b) = b-a
    RET
```

Four instructions. `CP B` is gone.

### 10.5 Pass 4: PBQP Register Allocation

The CmpSubCarry transformation changes the Cmp's source from `%a` (the original parameter) to `%r2` (the sub result). This is the critical register interference fix.

Before the transformation: PBQP sees `%cond = cmp.ult(%a, %b)` using `%a`. Both `%a` and `%r2` (the sub result) need A for arithmetic, but they cannot simultaneously occupy A — PBQP inserts a spill (`LD C, A`).

After `CmpSubCarry`: `%a` is no longer a live source of the Cmp. The interference edge `%a(→A) ↔ %r2(→A)` disappears. PBQP can allocate `%r2` directly to A, no spill needed.

### 10.6 Pass 5: Dead Block Elimination

`@else` was left in place by CondRetSink (now unreachable). EliminateDeadBlocks removes it.

### 10.7 Full Optimization Summary

| Pass | Action | Before → After |
|------|--------|----------------|
| 1. CondRetSink | Hoist @else, replace BrIf | 8 instr → 6 instr |
| 2. SubSwapNeg | sub(b,a) → neg(r2) in @then | removes LD A,B |
| 3. HoistReorder | Sub before Cmp; Cmp→CmpSubCarry | 6 instr → 5 instr |
| 3a. CP elim | CmpSubCarry emits nothing | 5 instr → 4 instr |
| 4. PBQP | CmpSubCarry removes interference | spill LD C,A gone |
| 5. DeadBlock | Remove unreachable @else | cleanup |

### 10.8 Inline Survival

What happens when `abs_diff` is inlined at a call site?

After inlining, the `TermCondRet` becomes a `TermBrIf`:

```
block @call_site:
    ...
    %result = call abs_diff(%x, %y)   ← before inlining
    ; after inlining:
    %r2   = sub %x, %y
    %cond = cmp.sub_carry %r2, %y
    br_if %cond, @call_neg(%r2), @after(%r2)   ← TermCondRet → TermBrIf

block @call_neg(%v):
    %neg_v = neg %v
    jmp @after(%neg_v)

block @after(%res):
    ; use %res ...
```

The Z80 output for the inlined site:

```asm
    SUB E          ; r2 = x - y (y in E, x in A after contract opt)
    JR NC, .after  ; if x >= y: skip negation
    NEG            ; r = -(x-y) = y-x
.after:
    ; use A ...
```

Same 4 instructions, zero call overhead (no `CALL`/`RET`). The `RET NC` becomes `JR NC, .label`. The optimization survives inlining intact.

### 10.9 abs_diff(u16, u16) — The 16-bit Case

```nanz
fun abs_diff16(a: u16, b: u16) -> u16 {
    if a < b {
        return b - a
    }
    return a - b
}
```

The same five passes fire at the IR level. The result at the MIR2 level is identical:

```
%r2   = sub %a, %b : u16
%cond = cmp.sub_carry %r2, %b
cond_ret %cond, [%r2], @then

@then:
    %r1 = neg %r2 : u16
    ret %r1
```

But the Z80 ISA introduces constraints that make the u16 case longer.

#### 10.9.1 16-bit Subtraction

Z80 has no direct `SUB HL, DE`. The only 16-bit subtraction with carry output is `SBC HL, rr`, which requires carry to be clear first:

```asm
abs_diff16:        ; a=HL, b=DE
    AND A          ; 4T — clear carry (OR A also works, same timing)
    SBC HL, DE     ; 15T — HL = a−b, carry = (a < b unsigned)
    RET NC         ; 11T/5T — if a≥b: return HL=a-b
```

Three instructions for the fast path (30T total when taken). Compare to `SUB B` + `RET NC` for u8 (8+11 = 19T when taken).

#### 10.9.2 16-bit NEG

Z80 has no 16-bit `NEG` instruction. The two's complement negation of HL uses the NEG+SBC trick:

```asm
    ; HL = x (we want HL = -x = b-a)
    LD A, L        ; 4T — low byte
    NEG            ; 4T — A = -L mod 256, carry = (L != 0) = borrow into high byte
    LD L, A        ; 4T — store low byte of result
    LD A, 0        ; 7T — (cannot use XOR A: that clears carry)
    SBC A, H       ; 4T — A = 0 − H − carry = -(H + borrow) = correct high byte
    LD H, A        ; 4T — store high byte
    RET            ; 10T
```

Total slow path (a < b): 4 + 15 + 5 + 4 + 4 + 4 + 7 + 4 + 4 + 10 = **61T**, 10 instructions.

Note: `LD A, 0` (7T, 2 bytes `3E 00`) is needed because `XOR A` would clear the carry left by `NEG`. No shorter alternative preserves carry while zeroing A.

#### 10.9.3 Current Limitation: ClassAcc for u16 NEG

The `applySubSwapNeg` pass currently forces `ClassAcc` on the `neg` result (`inst.Cls = ClassAcc`). This is correct for u8 — `NEG` always operates on A. For u16, however, `ClassAcc` is the 8-bit A register, and a u16 value does not fit.

**Status:** This is a known bug for u16 abs_diff with the current compiler. The fix requires two changes:
1. `applySubSwapNeg`: skip or use a different class when `h.Ty.Width() > 8`
2. `z80codegen OpNeg` for u16: emit the 6-instruction NEG+SBC sequence instead of a single `NEG`

Until fixed, `abs_diff16` compiles correctly via the fallback path (without the SubSwapNeg optimization), producing slightly more code but correct output.

#### 10.9.4 Summary: u8 vs u16

| Property | abs_diff(u8,u8) | abs_diff(u16,u16) |
|----------|----------------|-------------------|
| Fast path (a≥b) | `SUB B; RET NC` — 2 instr, 19T | `AND A; SBC HL,DE; RET NC` — 3 instr, 30T |
| Slow path (a<b) | `NEG; RET` — 2 instr, 14T | 7-instruction 16-bit NEG, 37T |
| Single NEG instruction | ✅ `NEG` | ❌ (must use NEG+SBC, 6 instr) |
| CP elimination | ✅ CmpSubCarry | ✅ same (no `AND A; SBC` repeated) |
| applySubSwapNeg | ✅ works | ⚠️ bug (ClassAcc mismatch, fix pending) |

The u8 case is special because the Z80 ISA offers `NEG` and `SUB r` as single-instruction primitives that perfectly match the two operations needed. The u16 case is structurally identical at the MIR2 level but requires more Z80 instructions due to ISA limitations.

---

## Appendix A: Complete Syntax Grammar

```ebnf
module      = top_decl*
top_decl    = struct_decl
            | interface_decl
            | global_decl
            | fun_decl
            | '@extern' 'fun' fun_decl_inner

struct_decl    = 'struct' IDENT '{' field_decl* '}'
field_decl     = IDENT ':' type ','?

interface_decl = 'interface' IDENT '{' method_name* '}'
method_name    = 'fun'? IDENT ','?

global_decl    = 'global' IDENT ':' type at_clause? init_clause?
at_clause      = 'at' '(' expr ')'
init_clause    = '=' ('[' expr (',' expr)* ']' | expr)

fun_decl       = 'fun' fun_decl_inner
fun_decl_inner = (op_symbol | IDENT ('.' IDENT)?) '(' params ')' ('->' type)?
                 ('{' stmt* '}' | /* extern: no body */)
params         = (IDENT ':' type (',' IDENT ':' type)*)?
op_symbol      = '+' | '-' | '*' | '/' | '%' | '==' | '!=' | '<' | '<=' | '>' | '>=' | '&' | '|' | '^'

type           = '^' type
               | '[' type ';' INT ']'
               | 'u8' ('<' INT '..' INT '>')?
               | 'u16' ('<' INT '..' INT '>')?
               | 'i8'
               | 'i16'
               | 'bool'
               | 'void'
               | 'ptr'
               | IDENT     (* struct or interface name *)

stmt           = var_decl
               | let_decl
               | if_stmt
               | while_stmt
               | for_stmt
               | return_stmt
               | 'break'
               | 'continue'
               | switch_stmt
               | block
               | expr_stmt

var_decl       = 'var' IDENT ':' type at_clause? ('=' (array_init | expr))?
let_decl       = 'let' IDENT (':' type)? '=' expr
array_init     = '[' expr (',' expr)* ']'

if_stmt        = 'if' expr block ('else' block)?
while_stmt     = 'while' expr block
for_stmt       = 'for' IDENT (':' type)? 'in'
                 (expr '[' expr? '..' expr ']' block   (* ForEachStmt *)
                 | expr '..' expr block)                (* ForRangeStmt *)
return_stmt    = 'return' expr?
switch_stmt    = 'switch' expr '{' case_clause* default_clause? '}'
case_clause    = 'case' INT ':' stmt*
default_clause = 'default' ':' stmt*
block          = '{' stmt* '}'

expr_stmt      = expr ('=' expr)?     (* assignment or bare call *)

expr           = binary_expr
binary_expr    = unary_expr (binop binary_expr)*
binop          = '+' | '-' | '*' | '/' | '%' | '&' | '|' | '^' | '<<' | '>>'
               | '==' | '!=' | '<' | '<=' | '>' | '>='

unary_expr     = '-' unary_expr
               | '!' unary_expr
               | '~' unary_expr
               | '&' IDENT
               | postfix_expr

postfix_expr   = primary
                 ( '^'                      (* dereference *)
                 | '[' expr ']'             (* index *)
                 | '.' IDENT               (* field *)
                 | '.' IDENT '(' args ')'  (* UFCS method call *)
                 | '(' args ')'            (* function call *)
                 )*

primary        = INT
               | 'true' | 'false'
               | STRING
               | ('u8' | 'u16' | 'i8' | 'i16') '(' expr ')'   (* cast *)
               | '@ptr' '(' type ',' expr ')'
               | '|' lambda_params '|' (block | expr)            (* lambda *)
               | '(' expr ')'
               | IDENT

lambda_params  = (IDENT (':' type)? (',' IDENT (':' type)?)*)?

args           = (expr (',' expr)*)?
```

**Lexical notes:**
- Comments: `//` (line) and `/* */` (block)
- Whitespace (including newlines) is insignificant — the lexer is whitespace-agnostic
- Integers: decimal or `0x`/`0X` hexadecimal
- Strings: double-quoted, no escape sequences currently supported

---

## Appendix B: Register Class Reference

### Z80 Physical Registers

| Register | Typical use in MIR2 | T-state cost to access |
|----------|--------------------|-----------------------|
| A | ALU accumulator, return value (8-bit), first u8 param | 0T (ALU ops use A implicitly) |
| B | Loop counter (DJNZ), third param | 4T (LD B, r) |
| C | General 8-bit, second param | 4T (LD C, r) |
| D | General 8-bit | 4T |
| E | General 8-bit | 4T |
| H | High byte of HL (cannot be used independently when HL is a pointer) | 4T |
| L | Low byte of HL | 4T |
| HL | 16-bit pointer/value, first u16/ptr param, return (16-bit) | 0T (many Z80 ops use HL directly) |
| DE | 16-bit index, second u16 param | 11T (ADD HL, DE) |
| BC | 16-bit, third u16 param / loop counter | 11T (ADD HL, BC) |
| IX | Overflow pointer when HL/DE/BC all occupied | 19-21T |
| IY | Rarely used (sometimes reserved for system) | 19-21T |

### MIR2 Register Classes

| Class | Z80 physical | When assigned |
|-------|-------------|---------------|
| `ClassAcc` | A | First u8 param; 8-bit return; ALU-heavy values |
| `ClassCounter` | B | DJNZ loops; third param; loop variables |
| `ClassPointer` | HL | Pointers; u16 values; 16-bit return; first u16 param |
| `ClassIndex` | DE | Second u16 param; index register for ADD HL,DE |
| `ClassGeneral` | C, D, E, H, L | Intermediate 8-bit values; second+ params |

### Memory-Backed Registers ($F0xx)

When all physical registers in a class are exhausted, MIR2 spills to `$F0xx` addresses. These are fast RAM locations used as a "register file extension." An 8-bit load from $F0xx costs `LD A, ($F0xx)` (13T) vs `LD A, C` (4T). The PBQP allocator minimizes spills by cost-weighting assignments.

---

## Appendix C: CLI Usage

### Compiling a Nanz Program

```bash
# Compile to Z80 assembly (.a80 format for MZA assembler)
mz source.nanz -o output.a80

# The mz binary is at: minzc/mz (from the project root)
/Users/alice/dev/minz-ts/minzc/mz source.nanz -o output.a80
```

### Emit Intermediate Representations

```bash
# Emit HIR dump
mz source.nanz --emit=hir

# Emit MIR2 dump
mz source.nanz --emit=mir2

# Emit Z80 assembly (same as -o, printed to stdout)
mz source.nanz --emit=asm
```

### Other Flags

```bash
# Enable optimizations (constant folding, dead store elim, LUTGen, contracts)
mz source.nanz -O

# Specify target system
mz source.nanz --target=spectrum   # ZX Spectrum
mz source.nanz --target=cpm        # CP/M
mz source.nanz --target=agon       # Agon Light 2 (eZ80)
```

### Running Tests

From the `minzc/` directory:

```bash
# Run all Go tests (23/23 packages)
go test ./pkg/... -vet=off

# Run only Nanz parser tests
go test ./pkg/nanz/... -vet=off -v

# Run only MIR2 tests (includes LUTGen, contracts)
go test ./pkg/mir2/... -vet=off -v

# Run E2E tests (requires qbe and cc in PATH)
go test ./pkg/mir2qbe/... -vet=off -v
```

### Example Programs

The `examples/nanz/` directory contains complete examples with both source and compiled output:

| File | Description |
|------|-------------|
| `01_sum_array.nanz` | While loop with indexed array access |
| `01_sum_array.a80` | Compiled Z80 output |
| `02_sum_array_idiomatic.nanz` | For-each loop and iterator chain versions |
| `03_filter_map_chain.nanz` | Full map/filter/forEach chain with analysis |
| `04_lut_popcount.nanz` | LUT generation via ranged type annotation |
| `04_lut_popcount.a80` | Compiled output showing 3-instruction lookup |
| `05_four_pointers.nanz` | Four live pointer params (PBQP showcase) |
| `06_pbqp_weighted.nanz` | Weighted cost allocation (hot register wins A) |
| `07_ix_load_store.nanz` | IX/IY overflow addressing when HL/DE/BC full |

---

*Nanz: Modern syntax, zero-cost abstractions, Z80 performance.*

*Pipeline: `.nanz` → `nanz.Parse()` → `*hir.Module` → `hir.LowerModule()` → `*mir2.Module` → Z80 codegen → `.a80`*
