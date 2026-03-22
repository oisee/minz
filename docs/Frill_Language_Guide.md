# Frill — Functional Language for Z80

**Frill** is an ML-style functional language that compiles to Z80 assembly
through the MinZ compiler pipeline. The name is ironic — "frills" (ruffles,
decorations) for the most constrained hardware imaginable.

## Philosophy

**"What if OCaml/F# ran on a ZX Spectrum?"**

Frill takes the best ideas from ML-family languages and strips them down to
what makes sense on 8-bit hardware with 64KB RAM:

| From | Idea | Frill adaptation |
|------|------|------------------|
| OCaml/F# | ADT, pattern matching, HM inference | ADT as tagged u8, match as if-chain |
| F# | Pipe operator `\|>` | Direct — zero overhead |
| Haskell | Where-clauses, type classes | `let-in` (where = future) |
| Lean 4 | Strict, RC + destructive reuse | Strict by default, stack-based lifetime |
| Idris 2 | Linear types (QTT) | Future: `1 x` = use exactly once |
| Rust | Ownership without borrow checker noise | Implicit through linearity (planned) |

**Three design principles:**

1. **Strict** — no thunks, no lazy evaluation, no GC needed
2. **Size-known** — every type has compile-time-known size (u8, u16, tagged unions)
3. **Zero-cost** — functional abstractions compile to the same Z80 code a hand-written
   assembly programmer would write

## Quick Start

```
(* hello.frl — your first Frill program *)

let double (x : u8) : u8 = x + x
let inc (x : u8) : u8 = x + 1

(* pipe: 3 -> double -> inc = 7 *)
let transform (x : u8) : u8 = x |> double |> inc

assert transform 3 == 7
assert transform 10 == 21
```

Compile and verify:

```bash
minzc hello.frl              # -> hello.a80 (Z80 assembly)
minzc hello.frl -o hello.bin # -> binary
```

Asserts are verified at compile time via the MIR2 VM — no runtime overhead.

## Syntax Reference

### Functions

ML-style: `let name (param : type) ... : return_type = body`

```
let add (a : u8) (b : u8) : u8 = a + b
let square (x : u8) : u8 = x * x
let negate (x : i8) : i8 = 0 - x
```

Compiles to:
```z80
add:     ADD A, C / RET      ; 2 instructions, 8 T-states
square:  JP __mul8            ; tail call
```

### Types

Fixed-size, known at compile time:

| Type | Size | Range |
|------|------|-------|
| `u8` | 1 byte | 0..255 |
| `u16` | 2 bytes | 0..65535 |
| `i8` | 1 byte | -128..127 |
| `i16` | 2 bytes | -32768..32767 |
| `bool` | 1 byte | true/false |

No heap. No GC. No boxing. Ever.

### If-Then-Else (Expression)

```
let max (a : u8) (b : u8) : u8 =
  if a > b then a else b

let clamp (x : u8) (lo : u8) (hi : u8) : u8 =
  if x < lo then lo
  else if x > hi then hi
  else x
```

If is an **expression** — always returns a value, always has both branches.

### Let-In (Scoped Bindings)

```
let distance (x1 : u8) (y1 : u8) (x2 : u8) (y2 : u8) : u8 =
  let dx = abs_diff x1 x2 in
  let dy = abs_diff y1 y2 in
  dx + dy
```

Let-in chains are desugared to HIR VarDeclStmt sequences — zero overhead,
no closures, no heap allocation. Variables live on the Z80 stack or in
registers.

### Pipe Operator |>

```
let process (x : u8) : u8 = x |> double |> inc |> square

(* equivalent to: square(inc(double(x))) *)
```

Pipes with extra arguments:

```
let offset (x : u8) : u8 = x |> add 10

(* equivalent to: add(x, 10) *)
```

The pipe operator is purely syntactic — it rearranges the call, generating
identical code to the nested version.

### Algebraic Data Types (ADT)

```
type Direction = North | South | East | West

type Color = Red | Green | Blue | Yellow | Cyan | Magenta | White | Black
```

Constructors compile to sequential integer tags (North=0, South=1, ...).
An ADT value is a single `u8` — the tag. No heap, no pointers, no vtables.

Constructors work as expressions:

```
let default_dir (x : u8) : u8 = North    (* returns 0 *)
let go_east (x : u8) : u8 = East         (* returns 2 *)
```

### Pattern Matching

```
let describe (d : u8) : u8 =
  match d with
  | North -> 0
  | South -> 1
  | East  -> 2
  | West  -> 3
  | _     -> 99
  end

let is_weekend (d : u8) : u8 =
  match d with
  | Sat -> 1
  | Sun -> 1
  | _   -> 0
  end
```

`match` compiles to a chain of comparisons — equivalent to a switch/case
on the tag value. The `_` wildcard is the default/else branch.

On Z80 this becomes:
```z80
  CP 5         ; == Sat?
  JR Z, .yes
  CP 6         ; == Sun?
  JR Z, .yes
  XOR A        ; default: 0
  RET
.yes:
  LD A, 1
  RET
```

### Assertions (Compile-Time Tests)

```
assert add 3 5 == 8
assert max 10 20 == 20
assert is_weekend 5 == 1
```

Asserts run on the MIR2 virtual machine at compile time. They are NOT
included in the binary — zero runtime cost. If an assert fails,
compilation stops with an error showing expected vs actual.

### Comments

```
(* This is a block comment — OCaml style *)
-- This is a line comment — Haskell/SQL style
```

## How It Works: The Pipeline

```
  Frill source (.frl)
       |
       v
  [Frill Parser]  ---- pkg/frill/frill.go (809 LOC)
       |                Lexer + recursive descent parser
       |                Desugars: pipe, let-in, match, ADT
       v
  [HIR Module]     ---- pkg/hir/hir.go
       |                Typed AST: Func, VarDecl, If, Return, Call, BinExpr...
       |                Target-neutral, structured control flow
       v
  [MIR2 Module]    ---- pkg/hir/lower.go → pkg/mir2/
       |                SSA form, basic blocks, virtual registers
       |                Optimization: constant fold, DSE, inline, PBQP alloc
       v
  [LIR Backend]    ---- pkg/lir/
       |                ISLE instruction combining + WFC register allocation
       |                Pattern-driven: data tables, not code
       v
  [Z80 Assembly]   ---- .a80 file
       |
       v
  [MZA Assembler]  ---- pkg/z80asm/
       |                Table-driven encoder, 1335/1335 FUSE tests
       v
  [Binary]         ---- .bin / .sna / .tap / .com
```

Each Frill construct maps to HIR nodes:

| Frill | HIR |
|-------|-----|
| `let f (x : u8) : u8 = body` | `Func{Name, Params, RetTy, Body}` |
| `x + y` | `BinExpr{"+", VarRef{x}, VarRef{y}}` |
| `if c then a else b` | `CondExpr{c, a, b}` |
| `let x = e in body` | `VarDeclStmt{x, e}` + body |
| `x \|> f` | `CallExpr{f, [x]}` |
| `match x with \| P -> e end` | Nested `CondExpr` chain |
| `type T = A \| B` | Constructor registry (tag integers) |
| `assert f x == n` | `Assert{f, [x], n, via:"mir2"}` |

## What's Implemented (Frill-0)

- [x] Function definitions with typed parameters
- [x] Arithmetic: `+ - * / %`
- [x] Comparisons: `== != < <= > >=`
- [x] If-then-else expressions
- [x] Let-in bindings with chaining
- [x] Pipe operator `|>`
- [x] Algebraic data types (tagged constructors)
- [x] Pattern matching with `match/with/end`
- [x] Wildcard `_` default pattern
- [x] Compile-time assertions via MIR2 VM
- [x] OCaml-style block comments `(* ... *)`
- [x] Haskell-style line comments `-- ...`
- [x] CLI: `minzc program.frl`
- [x] MZV: `mzv program.frl`
- [x] 14 unit tests, 18 E2E assertions

## What's Missing (Roadmap)

### Frill-1: Full ML

- [ ] **Nested function calls** — `f (g x)` currently needs let-in workaround
- [ ] **Recursion** — works at MIR2 level, parser needs to allow self-reference
- [ ] **Multi-line function bodies** — `where` clause (Haskell style)
- [ ] **Type inference** — omit `: u8` when obvious from context
- [ ] **Tuples** — `let (a, b) = divmod x y`
- [ ] **Strings** — `"hello"` as `^u8` (null-terminated pointer)
- [ ] **ADT with payload** — `type Option = None | Some of u8` (tagged union, 2 bytes)
- [ ] **Exhaustiveness checking** — warn if match doesn't cover all constructors
- [ ] **Imports** — `import math` for stdlib modules

### Frill-2: Advanced Types

- [ ] **Type classes** — `class Num a where (+) : a -> a -> a`
  - Compiled via dictionary passing (struct of function pointers)
- [ ] **Parametric polymorphism** — `let id (x : 'a) : 'a = x`
  - Monomorphized at call site (like C++ templates, no runtime cost)
- [ ] **Records** — `type Point = { x : u8, y : u8 }`
  - Compile to structs with known offsets

### Frill-3: Linear Types & Dependent Types

- [ ] **Linear types (QTT)** — `let consume (1 buf : Buffer) : ()` — use exactly once
  - Compiler auto-inserts `free()` at last use
  - Destructive reuse: if refcount=1, update in place (Lean 4 style)
- [ ] **Erased types** — `let size (0 T : Type) : u16` — type-level only, zero runtime
- [ ] **Dependent types** — `Vec (n : u8) (a : Type)` — length-indexed vectors
  - Proofs erased at compile time, no runtime representation

### Why These Matter on Z80

**Linear types** formalize what Z80 programmers do intuitively. When you pass
a buffer to a function and never use it again, the compiler knows it can
reuse the memory. No GC, no RC overhead — the stack frame IS the allocator.

**Dependent types** catch buffer overflows at compile time. `Vec 10 u8`
guarantees exactly 10 elements — the compiler rejects `index 11`.

**Type classes** replace vtables with monomorphization. `print x` where `x : u8`
compiles to `CALL print_u8` — zero indirection.

## Examples

### Collatz Conjecture

```
let collatz_step (n : u8) : u8 =
  if n == 1 then 1
  else if n % 2 == 0 then n / 2
  else n * 3 + 1

assert collatz_step 6 == 3
assert collatz_step 5 == 16
assert collatz_step 1 == 1
```

### Day of Week

```
type Day = Mon | Tue | Wed | Thu | Fri | Sat | Sun

let is_weekend (d : u8) : u8 =
  match d with
  | Sat -> 1
  | Sun -> 1
  | _ -> 0
  end

assert is_weekend 5 == 1    (* Sat *)
assert is_weekend 0 == 0    (* Mon *)
```

### Pipeline Processing

```
let double (x : u8) : u8 = x + x
let inc (x : u8) : u8 = x + 1
let square (x : u8) : u8 = x * x

let process (x : u8) : u8 = x |> double |> inc |> square

assert process 3 == 49    (* 3 -> 6 -> 7 -> 49 *)
```

### Complex with Let-In

```
let compute (x : u8) : u8 =
  let a = x + 1 in
  let b = a * 2 in
  let c = b - x in
  c |> double

assert compute 4 == 12   (* a=5, b=10, c=6, double=12 *)
```

## Design Decisions

**Why strict?** Lazy evaluation requires thunks (heap-allocated closures).
On Z80 with 48KB usable RAM and no MMU, heap allocation means manual memory
management or GC — both unacceptable. Strict evaluation uses the stack
naturally.

**Why no GC?** The Z80 runs at 3.5 MHz. A GC pause of even 1000 cycles
(0.3ms) is visible in games running at 50fps. Linear types + stack
allocation give deterministic performance.

**Why tags as u8?** An ADT with <=256 constructors fits in a single byte.
Pattern matching becomes a CP instruction (4 T-states). Tagged unions
with payload will use 1 byte tag + N bytes payload — still fixed-size,
still stack-allocated.

**Why desugar match to if-chain?** For <=8 arms, a chain of CP/JR Z
instructions is faster than a jump table on Z80 (no multiply for index,
no memory load for target). The compiler can switch to jump tables for
larger matches in the future.

## Relationship to Other MinZ Languages

MinZ has 8 frontend languages, all sharing the same backend:

```
Frill (.frl)  ----\
Nanz (.nanz)  ----+
C89 (.c)      ----+
Pascal (.pas) ----+---> HIR ---> MIR2 ---> LIR ---> Z80/eZ80
Lanz (.lanz)  ----+
Lizp (.lizp)  ----+
PL/M (.plm)   ----+
ABAP (.abap)  ----/
```

Frill is the **functional** member of the family. Use it when you want:
- Pattern matching on structured data
- Pipeline-style data transformation
- Compile-time verified correctness (assert)
- Clean, expression-oriented code

Use Nanz when you need:
- Mutable state, while loops, imperative I/O
- Inline assembly (`asm z80 { ... }`)
- Stdlib imports (TUI, graphics, sound)
- Closures and iterators
