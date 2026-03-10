# The Nanz Language: A Complete Guide (v2)

> Nanz is a typed systems language for Z80 and retro targets.
> It compiles through HIR → MIR2 → Z80 assembly — a clean, modern pipeline
> shared with the PL/M-80 frontend.

**Version:** MinZ compiler v0.19.5 (MIR2 pipeline)
**Date:** 2026-03-10
**Status:** Active development — core features stable, some corners rough
**Source of truth:** `pkg/nanz/parse.go`, `pkg/hir/`, `pkg/mir2/`, `pkg/pipeline/`

---

## Table of Contents

1. [What is Nanz?](#chapter-1-what-is-nanz)
2. [Syntax Reference](#chapter-2-syntax-reference)
3. [Type System](#chapter-3-type-system)
4. [Compilation Pipeline](#chapter-4-compilation-pipeline)
5. [MIR2 Intermediate Representation](#chapter-5-mir2-intermediate-representation)
6. [Optimization Passes](#chapter-6-optimization-passes)
7. [Z80 Code Generation](#chapter-7-z80-code-generation)
8. [Iterator Chains and Fusion](#chapter-8-iterator-chains-and-fusion)
9. [Zero-Cost Abstractions](#chapter-9-zero-cost-abstractions)
10. [QBE Correctness Oracle](#chapter-10-qbe-correctness-oracle)
11. [PL/M-80: Corpus Seeding and Idiomatic Translation](#chapter-11-plm-80-corpus-seeding-and-idiomatic-translation)
12. [Compiled Output Gallery](#chapter-12-compiled-output-gallery)
13. [Relation to MinZ and PL/M](#chapter-13-relation-to-minz-and-plm)
- [Appendix A: Complete Syntax Grammar](#appendix-a-complete-syntax-grammar)
- [Appendix B: Register Classes and Cost Table](#appendix-b-register-classes-and-cost-table)
- [Appendix C: CLI Reference](#appendix-c-cli-reference)
- [Appendix D: Installing External Tools](#appendix-d-installing-external-tools)
- [Appendix E: Known Bugs and Limitations](#appendix-e-known-bugs-and-limitations)

---

## Chapter 1: What is Nanz?

Nanz (`.nanz` extension) is the active frontend language of the MinZ compiler. It is statically typed, imperative, and designed for two audiences:

1. Developers writing Z80 / retro-target programs who want modern syntax with zero-cost abstractions.
2. The MinZ compiler team developing and testing the MIR2 backend.

### 1.1 The Compilation Pipeline

```
source.nanz
    │  nanz.Parse()
    ▼
*hir.Module          ← High-level IR (structured control flow, named variables)
    │  hir.LowerModule()
    ▼
*mir2.Module         ← Mid-level IR (SSA-like, virtual registers, typed ops)
    │  optimization passes (constant fold, DSE, BranchEquiv, CondRetSink, LUTGen)
    ▼
*mir2.Module         ← optimized
    │  OptimizeContracts() → PBQPAllocate()
    ▼
*mir2.Module         ← allocated (virtual → physical registers)
    │  Z80Codegen()
    ▼
output.a80           ← MZA-compatible Z80 assembly
    │  mza (assembler)
    ▼
output.bin / .tap    ← binary / ZX Spectrum tape image
```

Invoke with:

```bash
mz source.nanz -o output.a80
```

### 1.2 Nanz vs. MinZ

MinZ (`.minz`) is the original frontend with its own parser and code generator targeting the older MIR1 IR. That pipeline is **frozen** — it works but is not being developed.

Nanz targets MIR2, which provides:

- SSA-like dataflow with block parameters (Cranelift/MLIR style, not phi nodes)
- PBQP register allocator with weighted cost vectors
- LUT generation: compile-time evaluation for pure, range-bounded functions
- Interprocedural calling convention optimization
- A Z80 emulator used as a constant evaluator and branch equivalence prover

**Rule:** Write new retro programs in Nanz. Keep existing `.minz` programs as-is.

### 1.3 Nanz vs. PL/M-80

PL/M-80 (`.plm`) is Intel's 1970s language for CP/M. The MinZ compiler includes a PL/M-80 parser that compiles through the **same HIR → MIR2 → Z80 pipeline** as Nanz:

```bash
mz legacy.plm -o legacy.a80     # same backend as Nanz
mz legacy.plm --emit=nanz       # translate PL/M to Nanz source
```

### 1.4 Design Philosophy

Nanz is deliberately minimal. The grammar fits in a screenful. No garbage collector, no runtime, no dynamic dispatch. Every abstraction — lambdas, iterators, struct methods, interfaces — compiles to direct Z80 instructions with no overhead beyond what you would write by hand.

---

## Chapter 2: Syntax Reference

The parser is a hand-written recursive-descent parser with a Pratt expression parser (`pkg/nanz/parse.go`). Source: ~2000 LOC Go.

### 2.1 Module Structure

A Nanz source file is a module: a sequence of top-level declarations in any order.

```nanz
// line comment
/* block comment */

struct Point { x: u8, y: u8 }
interface Drawable { draw }
global counter: u8
fun add(a: u8, b: u8) -> u8 { return a + b }
```

There are no imports; linking is handled at the assembler level.

### 2.2 Types

| Syntax | Width | Z80 mapping | Notes |
|--------|-------|-------------|-------|
| `u8` | 8 bit | A/B/C/D/E/H/L | unsigned byte |
| `u16` | 16 bit | HL/DE/BC | unsigned word |
| `i8` | 8 bit | same regs, different arithmetic | signed byte |
| `i16` | 16 bit | same | signed word |
| `bool` | 8 bit | false=0, true=1 | |
| `void` | — | return type only | |
| `ptr` | 16 bit | HL/DE/BC | untyped pointer |
| `^T` | 16 bit | HL/DE/BC | typed pointer to T |
| `[T; N]` | N×width(T) | — | fixed-size array |
| `u8<lo..hi>` | 8 bit | — | ranged type (LUT candidate) |
| `u16<lo..hi>` | 16 bit | — | ranged type |
| `StructName` | sum of fields | passed by pointer | struct type |
| `InterfaceName` | — | resolved at compile time | interface as param type |

**Pointer types:** `^T` records the element type for field resolution. When T is a struct, `^Struct` enables typed field access through the pointer (e.g., `self.val` on a `^Acc` receiver auto-dereferences and resolves field offsets). This is NOT merely syntactic sugar — the parser uses `varPtrElem[paramName]` to track the pointed-to struct for field resolution and UFCS dispatch.

**Ranged types:** `u8<0..255>` declares a ranged input. The range is inclusive in source (`0..255`), stored exclusive internally (`[0, 256)`). Functions with a single ranged parameter and pure body are candidates for LUT generation (see §6.4).

### 2.3 Functions

```nanz
fun name(param1: Type1, param2: Type2) -> ReturnType {
    // body
}
```

`fn` is accepted as an alias for `fun`. Functions with no return value omit `-> ReturnType` (implicitly `void`).

```nanz
fn add(a: u8, b: u8) -> u8 { return a + b }

fun clear(buf: ^u8, n: u8) {
    var i: u8 = 0
    while i < n { buf[i] = 0; i = i + 1 }
}
```

### 2.4 Extern Functions

Functions implemented outside the Nanz module are declared with `@extern`:

```nanz
@extern fun process(x: u8) -> void
@extern fun rom_print(s: ptr) -> void
```

The body is omitted. The compiler assigns register classes to parameters following the standard calling convention.

**Status:** The annotated form `@extern("sym", params=[z80_a], returns=[z80_a])` described in some documentation is **not yet implemented** in the parser. Currently, `@extern` functions get automatic register assignment only.

### 2.5 Variable Declarations

**`var` — explicit type, optional initializer:**

```nanz
var i: u8 = 0
var buf: ^u8
var port: u8 at(0xFE)          // memory-mapped at absolute address
```

**`let` — type inferred from initializer:**

```nanz
let x = 42                      // x: u8
let ptr = @ptr(u8, 0xFE)       // ptr: ptr
let y: u16 = 1000              // explicit type override
```

The `at(addr)` clause maps a variable to a fixed memory address. Reads and writes become `LD (addr), r` / `LD r, (addr)`.

### 2.6 Global Variables

```nanz
global counter: u8
global vram: u8 at(0x4000)            // memory-mapped
global table: [u8; 4] = [1, 2, 4, 8]  // initialized array
global palette: Color                  // struct global
```

Globals are emitted in the data section of `.a80` output. Array globals with initializers get `DB` directives.

### 2.7 Literals

```nanz
42          // decimal — u8 if ≤255, u16 otherwise
0xFF        // hexadecimal
0           // zero (u8)
true        // bool
false       // bool
"hello"     // string → ptr to NUL-terminated bytes (interned)
```

### 2.8 Operators

**Arithmetic** (left-associative):
`+` `-` `*` `/` `%`

**Bitwise:**
`&` `|` `^` (XOR) `~` (NOT, unary) `<<` `>>`

**Comparison** (produce `bool`):
`==` `!=` `<` `<=` `>` `>=`

**Logical:**
`!` (NOT, unary)

**Precedence** (highest to lowest):

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

```nanz
let val = ptr^            // dereference (load byte at ptr)
^ptr = 42                 // store through pointer
let b = buf[3]            // index load (buf + 3)
buf[i] = 0               // index store
let p = &counter          // address-of global
let kb = @ptr(u8, 0xFE)  // typed constant pointer to absolute address
```

`@ptr(T, addr)` is the idiomatic way to reference hardware registers and ROM routines.

### 2.10 Casts

```nanz
u8(expr)     // truncate to u8
u16(expr)    // zero-extend to u16
i8(expr)     // reinterpret as signed 8-bit
i16(expr)    // reinterpret as signed 16-bit
```

Casts are explicit — no implicit widening. On Z80, `u8→u16` costs `LD H, 0 / LD L, A`; the compiler won't hide that.

### 2.11 Control Flow

```nanz
// if / else
if condition { /* then */ } else { /* else */ }

// while
while condition { /* body */ }

// for range (exclusive end)
for i in 0..n { /* i = 0, 1, ..., n-1 */ }

// for each (sequential memory scan)
for x: u8 in buf[0..n] { /* x loaded from buf[0]..buf[n-1] */ }

// break and continue
while true { if done { break } if skip { continue } }

// return
return           // void
return expr      // value

// switch
switch value {
    case 0: return 10
    case 1: return 20
    default: return 0
}
```

`for x: u8 in ptr[start..end]` compiles to a DJNZ loop with `HL` as pointer and `B` as counter — the tightest Z80 loop.

Switch cases do not fall through. Each case body terminates at the next `case`, `default`, or `}`.

### 2.12 Structs

```nanz
struct Point { x: u8, y: u8 }
struct Vec3d { x: u16, y: u16, z: u8 }
```

Fields are laid out sequentially, no padding. Offsets computed at parse time:
- `Point.x` → offset 0, `Point.y` → offset 1
- `Vec3d.y` → offset 2, `Vec3d.z` → offset 4

Struct values are always passed **by pointer** (HL on Z80). Field access: `Load(ptr + offset)`.

### 2.13 Struct Methods and UFCS

Methods are declared with `TypeName.methodName` syntax:

```nanz
struct Acc { val: u8 }

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}
```

The compiler stores this as `Acc_add`. At call sites, UFCS rewrites:

```nanz
acc_g.add(5)    // → Acc_add(&acc_g, 5) — direct CALL, no vtable
```

**How UFCS resolution works** (parse time):
1. Parser sees `base.method(args)`.
2. Looks up `exprTy(base)` — if it's a struct, check `methodTable[structName][method]`.
3. If base is `^Struct` pointer, check `varPtrElem[name]` for the struct type.
4. If base is an interface-typed variable, call `findImplementors()`.
5. Rewrite to `CallExpr{Fn: "StructName_method", Args: [base, args...]}`.

**Pointer receivers** (`self: ^Acc`):
- `self.val` auto-dereferences — resolves field offset through `varPtrElem`.
- `self^.val` also works (explicit dereference, same result).
- The pointer travels in HL (ClassPointer). Field reads: `LD reg, (HL)` at offset 0, `INC HL; LD reg, (HL)` at offset 1, etc.

**Value receivers** (`self: Acc`):
- Struct passed by pointer — same as `^Acc` at the ABI level.
- Parser records in `methodTable` with full return type info.

### 2.14 Operator Overloading

```nanz
fun +(a: Vec2, b: Vec2) -> Vec2 { return a }
fun -(a: Vec2, b: Vec2) -> Vec2 { return a }

fun compute(a: Vec2, b: Vec2) -> Vec2 {
    return a + b    // dispatched to op_add(a, b) because a is Vec2
}
```

Overloadable operators: `+` `-` `*` `/` `%` `==` `!=` `<` `<=` `>` `>=` `&` `|` `^`.

Mangled names: `op_add`, `op_sub`, `op_mul`, `op_div`, `op_rem`, `op_eq`, `op_ne`, `op_lt`, `op_le`, `op_gt`, `op_ge`, `op_and`, `op_or`, `op_xor`.

**Important:** For primitive types (`u8 + u8`), the built-in `BinExpr` is always used, regardless of overloads in scope. Overloads only fire when the left operand is a struct type.

### 2.15 Interfaces

```nanz
interface Animal {
    speak
    move
}
```

Interfaces are **compile-time contracts**. No vtable, no fat pointer, no method dispatch table.

**As a parameter type:**

```nanz
fun feed(a: Animal) -> u8 {
    return a.speak()      // monomorphized at compile time
}
```

**Resolution rules:**
- **One implementor** → monomorphize: `a.speak()` → `Dog_speak(a)`.
- **Multiple implementors** → compile error: `"ambiguous dispatch: ... multiple types implement Animal: [Dog Cat]; use concrete type"`.
- **Zero implementors** → compile error.

**Interface as global type:**

```nanz
global g_thing: Drawable
```

Works the same: UFCS dispatch resolves to the unique implementor.

### 2.16 Lambdas

```nanz
let double = |x: u8| x * 2          // expression body
let add = |a: u8, b: u8| a + b      // multi-param
let greet = |x: u8| { return x + 1 }  // block body
let inc = |x| x + 1                 // type defaults to u8
```

Each lambda becomes a top-level function `lambda_N` (sequential counter). The reference at the call site is a `VarRefExpr{Name: "lambda_N"}`.

**Capture rules:**
- **Fused iterator lambdas** (forEach/map/filter): may capture and mutate outer locals. The compiler detects free variables via `hasFreeVars()`, skips standalone lowering, and threads captured vars as block params through the DJNZ loop. Zero heap, zero spill.
- **Non-fused lambdas** (function pointers): cannot capture outer locals — no runtime frame. Only globals accessible.

### 2.17 Iterator Methods (UFCS on Pointers)

```nanz
buf.forEach(|x: u8| { process(x) }, n)       // execute for each element
buf.map(|x: u8| (x * 2))                     // transform elements
buf.filter(|x: u8| (x > 5))                  // keep matching elements
buf.mapInPlace(|x: u8| (x + 2), n)           // in-place transform
```

These are recognized by the parser as UFCS method calls on pointer expressions. The HIR lowerer's `tryLowerIterChain` fuses chains into a single DJNZ loop. See Chapter 8.

---

## Chapter 3: Type System

### 3.1 MIR2 Types

The MIR2 IR (`pkg/mir2/types.go`) supports:

| Type | Width | Go representation |
|------|-------|-------------------|
| `TyVoid` | 0 | `&IntTy{Bits: 0}` |
| `TyBool` | 8 | `&IntTy{Bits: 8}` |
| `TyU8` | 8 | `&IntTy{Bits: 8, Signed: false}` |
| `TyI8` | 8 | `&IntTy{Bits: 8, Signed: true}` |
| `TyU16` | 16 | `&IntTy{Bits: 16, Signed: false}` |
| `TyI16` | 16 | `&IntTy{Bits: 16, Signed: true}` |
| `TyU24` | 24 | `&IntTy{Bits: 24}` (eZ80 future) |
| `TyPtr` | 16 | `&PtrTy{}` |
| `StructTy` | sum of fields | `&StructTy{Name, Fields}` |
| `ArrayTy` | N×elem | `&ArrayTy{Len, Elem, Layout}` |
| `TupleTy` | sum of elems | `&TupleTy{Elems}` (multi-return) |

Additionally, ranged types wrap a base type with `[Lo, Hi)` bounds:
```go
type RangedTy struct {
    Base Ty
    Lo, Hi int  // [Lo, Hi) — exclusive upper bound
}
```

### 3.2 Register Class Assignment

Parameters are assigned register classes based on position and type (`classForParam` in `hir/lower.go`):

| Position | Type | Class | Z80 Physical |
|----------|------|-------|-------------|
| 1st | `u8`, `bool` | `ClassAcc` | A |
| 1st | `u16`, `ptr`, struct | `ClassPointer` | HL |
| 2nd | `u8` | `ClassGeneral` | C |
| 2nd | `u16` | `ClassIndex` | DE |
| 3rd | `u8` | `ClassCounter` | B |
| 3rd+ | `u16` | `ClassPair` | BC/DE |

Return values: `u8` → `ClassAcc` (A), `u16`/`ptr` → `ClassPointer` (HL).

### 3.3 Struct Layout

Fields are packed sequentially, no alignment padding:

```nanz
struct Color { r: u8, g: u8, b: u8 }  // total: 3 bytes
// r at offset 0, g at offset 1, b at offset 2
```

The parser computes offsets at parse time by summing `Ty.Width() / 8` for each preceding field. Global struct fields get EQU labels in assembly: `palette__r EQU palette + 0`.

---

## Chapter 4: Compilation Pipeline

The full pipeline is orchestrated by `pkg/pipeline/pipeline.go`.

### 4.1 Pipeline Stages

```go
func CompileHIRSteps(hm *hir.Module) (Steps, error)
```

**Stage 1 — HIR → MIR2 Lowering** (`hir.LowerModule`):
- Named variables → virtual registers (fresh reg per assignment)
- Structured control flow → basic blocks with block parameters
- Loop mutations → block parameters at loop headers (detected via `scanMutations`)
- ForEachStmt → DJNZ-friendly pointer+counter loop

**Stage 2 — Per-Function Optimizations** (in order):
1. `EliminateDeadBlocks` — remove unreachable blocks
2. `ReorderBlocks` — improve fall-through
3. **Constant pipeline** (iterated to fixpoint):
   - `PropagateConstants` — trace constants through moves
   - `FoldConstants` — evaluate pure ops at compile time
   - `SimplifyIdentities` — `PtrAdd(x, 0)` → `Move(x)`, etc.
   - `ConstantCallElim` — fold calls with constant args via VM
4. `DeadStoreElim` — remove pure instructions with unused results (iterated)
5. `BranchEquiv` — VM-based branch elimination (proves redundant guards)
6. `CondRetSink` — hoist trivial else-blocks into conditional returns

**Stage 3 — Module-Level: LUTGen**:
- Pure functions with ranged params → compile-time lookup tables

**Stage 4 — Verification** (`Verify`):
- Unique block labels, valid terminators, type consistency

**Stage 5 — Interprocedural Optimization**:
- `OptimizeContracts` — greedy DP on call graph for register class assignment
- `ApplyContracts` — rewrite function signatures

**Stage 6 — Register Allocation**:
- `ComputeLiveness` — backward dataflow to fixpoint
- `PBQPAllocate` — weighted cost assignment of virtuals to Z80 physicals

**Stage 7 — Code Generation**:
- `Z80Codegen` → assembly text

### 4.2 Emit Formats

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz source translation
mz source.nanz -o output.a80     # Z80 assembly (default)
```

---

## Chapter 5: MIR2 Intermediate Representation

MIR2 is the core IR. It uses **block arguments** (Cranelift/MLIR style) instead of phi nodes.

### 5.1 Module Structure

```
Module
├── Funcs[] — each with Blocks[]
├── Globals[] — module-level data
├── Strings[] — interned string pool
└── Structs[] — struct type definitions
```

### 5.2 Instructions

Each instruction: `%dst = Op %src0, %src1 : Ty [Class]`

**Arithmetic:** `OpAdd`, `OpSub`, `OpMul`, `OpDiv`, `OpSDiv`, `OpMod`
**Bitwise:** `OpAnd`, `OpOr`, `OpXor`, `OpShl`, `OpShr`, `OpSar`
**Unary:** `OpNeg`, `OpNot`
**Conversion:** `OpExt` (zero-extend), `OpSext` (sign-extend), `OpTrunc`
**Comparison:** `OpCmp` with conditions: `CmpEq`, `CmpNe`, `CmpLt`, `CmpLe`, `CmpGt`, `CmpGe`, `CmpUlt`, `CmpUle`, `CmpUgt`, `CmpUge`, `CmpSubCarry`
**Data movement:** `OpConst` (immediate), `OpMove` (register copy)
**Memory:** `OpLoad`, `OpStore`, `OpAddrOf`, `OpAlloca`, `OpField` (ptr+offset), `OpPtrAdd` (ptr+runtime offset), `OpPtrBump` (ptr+compile-time stride)
**Calls:** `OpCall` (direct), `OpCallIndirect` (through pointer)
**SMC:** `OpPatchSlot`, `OpLoadPatched`, `OpPatch` (self-modifying code primitives)

### 5.3 Block Parameters

```
block @loop_head(%ptr: u16 [pointer], %cnt: u8 [counter]):
    %cond = cmp.gt %cnt, %limit : bool [flag]
    br_if %cond, @exit(), @body(%ptr, %cnt)
```

Block arguments define registers on entry. Arguments are passed at each edge (jump/branch). This replaces phi nodes and makes parallel copies explicit per edge.

### 5.4 Terminators

| Terminator | Semantics | Z80 emission |
|------------|-----------|-------------|
| `TermJmp(target, args)` | Unconditional jump | `JP label` |
| `TermBrIf(cond, then, else)` | Conditional branch | `JP Z/NZ/C/NC label` |
| `TermDJNZ(counter, body, exit)` | Decrement B, jump if non-zero | `DJNZ rel8` |
| `TermCondRet(cond, vals, then)` | Conditional return | `RET CC` |
| `TermRet(vals)` | Return | `RET` |

`TermCondRet` is produced by the CondRetSink optimization pass — it enables the Z80's single-instruction conditional return.

---

## Chapter 6: Optimization Passes

### 6.1 Constant Pipeline (Fixpoint)

Four passes iterate until no changes:

1. **PropagateConstants** — if `%r = const 42`, replace all uses of `%r` with 42.
2. **FoldConstants** — `const(3) + const(5)` → `const(8)`.
3. **SimplifyIdentities** — `PtrAdd(x, Const(0))` → `Move(x)`. Eliminates redundant zero-offset pointer arithmetic (critical for `^Struct` receivers).
4. **ConstantCallElim** — pure function with all-constant args → evaluate via MIR2 VM.

### 6.2 Dead Store Elimination

Removes pure instructions whose results are never used. Iterated to fixpoint because removing one dead instruction may make its sources dead.

**Never removed:** `OpStore`, `OpCall`, `OpCallIndirect`, `OpAsm`, `OpPatch` (side effects).

### 6.3 BranchEquiv (VM-Based Branch Elimination)

Proves conditional branches redundant by exhaustive VM testing.

**Example:** `abs_diff` with an `if a == b { return 0 }` guard. BranchEquiv runs all 256 `(v, v)` inputs through both the original and patched function. Both return 0 → branch provably redundant → replace `BrIf(eq, @zero, @diff)` with `Jmp(@diff)`.

**Sound for u8:** exhaustive 256-value test. For wider types: heuristic sampling (safe to extend later).

### 6.4 CondRetSink

Finds `BrIf(cond, @then, @else)` where `@else` is trivial (single predecessor, pure instructions, `TermRet`). Hoists `@else` instructions into the current block and replaces `BrIf` with `TermCondRet`.

**Fused optimizations fired immediately after hoisting:**
- **SubSwapNeg:** If the hoisted `sub(a, b)` has a reversed `sub(b, a)` in the then-block → replace with `neg(result)`. Saves `LD A,r; SUB r2` → one `NEG`.
- **HoistReorderSubBeforeCmp + CmpSubCarry:** Reorder `sub` before `cmp_lt` on the same operands → the carry flag from `SUB` IS the comparison result. Eliminates a `CP` instruction entirely.

### 6.5 LUTGen (Module-Level)

Pure functions with a single ranged parameter → lookup table.

**Eligibility:** 1 param of `u8<lo..hi>` or `u16<lo..hi>`, range ≤ 256, single return, no extern calls, no globals written.

**Process:**
1. VM-evaluate function at every input in range
2. Emit page-aligned `DB` table as global
3. Replace function body with table lookup:
   ```asm
   LD H, lut^H    ; 7T — page base (high byte only)
   LD L, C         ; 4T — index
   LD A, (HL)      ; 7T — lookup
   RET
   ```

**Result:** 18 T-states regardless of original function complexity. An 8-iteration popcount loop becomes 3 instructions.

### 6.6 PBQP Register Allocator

Replaces the old greedy allocator. PBQP (Partitioned Boolean Quadratic Program) minimizes:

```
Σ nodeCost[r][loc(r)] + Σ edgeCost[interfering pairs]
```

**Node cost:** `useCount[r] × costTable.Cost(r.Class, location)`. Hot registers pay more for expensive locations.

**Reduction rules:**
- **R0:** degree-0 (isolated) → assign cheapest location immediately.
- **R1:** degree-1 (leaf) → fold edge cost into neighbour, defer.
- **RN:** degree ≥ 2 → greedy by **delta** (`2nd_best − best`). Registers with large delta (high penalty if displaced) are allocated first.

**Result for 4 simultaneous ClassPointer registers:**
```
p0 → HL (cost 0)
p1 → DE (cost 4)
p2 → BC (cost 6)
p3 → IX (cost 8)  — no $F0xx memory spill
```

### 6.7 Post-Allocation Copy Coalescing

After PBQP assigns physical locations, `coalesceAllocResult` eliminates redundant copies at block boundaries:

- Collect affinity edges from `OpMove` and block param↔arg pairs.
- Single-pass recolouring: if no neighbour in the interference graph uses the target's location, recolor to match. A `recolored` lock prevents rotation cycles in loop phi-webs.

### 6.8 Interprocedural Contract Optimization

`OptimizeContracts` performs greedy DP on the call graph:

1. Topological sort (leaves first).
2. For each function: enumerate candidate register class vectors.
3. Cost = internal adapter cost + edge cost over all callers.
4. Pick minimum-cost assignment.

This chooses per-function calling conventions globally, reducing register moves at call sites.

---

## Chapter 7: Z80 Code Generation

`pkg/mir2/z80codegen.go` — converts allocated MIR2 to assembly text.

### 7.1 Instruction Emission

| MIR2 Op | Z80 Output |
|---------|-----------|
| `OpConst` u8 | `LD r, imm8` |
| `OpConst` u16 | `LD rr, imm16` |
| `OpAdd` u8 | `ADD A, r` |
| `OpAdd` u16 | `ADD HL, rr` |
| `OpSub` u8 | `SUB r` |
| `OpSub` u16 | `AND A; SBC HL, rr` |
| `OpNeg` | `NEG` (8-bit only, A register) |
| `OpCmp` | `CP r` (result in flags, ClassFlag) |
| `OpLoad` | `LD A, (HL)` / `LD A, (rr)` |
| `OpStore` | `LD (HL), r` |
| `OpCall` | `CALL label` |

### 7.2 IX/IY Addressing

When PBQP assigns a pointer to IX, codegen uses displacement addressing:

```asm
LD A, (IX+0)     ; 8-bit load, offset 0
LD (IX+1), A     ; 8-bit store, offset 1
LD L, (IX+0)     ; 16-bit load (lo)
LD H, (IX+1)     ; 16-bit load (hi)
```

**16-bit register↔IX copy:** DE/BC→IX uses undocumented byte-copy:
```asm
LD IXH, D    ; DD 62 — 8T (D not substituted by DD prefix)
LD IXL, E    ; DD 6B — 8T (total 16T)
```

**HL→IX byte-copy is INVALID:** The DD prefix substitutes H→IXH and L→IXL in both source and destination operand positions, so `LD IXH, H` decodes as `LD IXH, IXH` (NOP). Use `PUSH HL; POP IX` (21T) instead.

### 7.3 Peephole Optimizations in Codegen

**Dead-const suppression:** `OpConst` whose only use is `OpCmp` → emit `CP imm8` directly, skip `LD r, imm`.

**Copy propagation:** Track `holdsPhys[A] = D` — if A already holds D's value, skip `LD A, D`.

**Last-flags tracking:** If flags are already set by a prior instruction with the same operands, suppress redundant `CP`.

**Page-aligned LUT:** Pre-scan blocks for LUT access patterns → `LD H, sym^H; LD L, idx; LD A, (HL)` (18T).

**Global struct field direct-addressing:** Pre-scan for `AddrOf(global) + Field(offset) + Load/Store` → `LD A, (sym__field)` directly (13T).

**HL-chain detection:** Multiple consecutive field stores to the same struct → single `LD HL, sym` + `LD (HL), r; INC HL` chain (saves reloading HL).

---

## Chapter 8: Iterator Chains and Fusion

### 8.1 The Four Iterator Methods

| Method | Signature | Semantics |
|--------|-----------|-----------|
| `forEach(lambda, n)` | Execute lambda for n elements | Terminal |
| `map(lambda)` | Transform each element | Intermediate |
| `filter(lambda)` | Keep elements where lambda returns true | Intermediate |
| `mapInPlace(lambda, n)` | Transform and write back | Terminal |

These are recognized as UFCS on pointer expressions. The HIR lowerer's `tryLowerIterChain` fuses chains.

### 8.2 Fusion

```nanz
buf.map(|x| x * 2).filter(|x| x > 5).forEach(|x| { process(x) }, n)
```

Fused into a single `ForEachStmt`:

```
for x in buf[0..n]:
    let mapped = x * 2         // map body inlined
    if mapped > 5 {            // filter: skip if false
        process(mapped)        // forEach body inlined
    }
```

This becomes a single DJNZ loop:
```asm
.loop:
    LD A, (HL)     ; load element
    INC HL
    ADD A, A       ; map: x * 2
    CP 6           ; filter: x > 5 → x >= 6
    JR C, .skip
    CALL process   ; forEach body
.skip:
    DJNZ .loop
```

No intermediate array. No lambda CALL. Three stages, one loop.

### 8.3 Closure Capture in Fused Chains

```nanz
fun sum(buf: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    buf.forEach(|x: u8| { s = s + x }, n)
    return s
}
```

`s` is a free variable in the lambda. The compiler:
1. Detects free reference via `hasFreeVars()`.
2. Skips standalone lowering of `lambda_N`.
3. Threads `s` as a block parameter through the DJNZ loop.

Result: `s` lives in a register (e.g., C) throughout — zero spill, zero heap.

### 8.4 Per-Element Cost (sum_chain)

```asm
LD D, (HL)     ; 7T — load element
LD A, C        ; 4T
ADD A, D       ; 4T — s + x
LD C, A        ; 4T
INC HL         ; 6T
DJNZ .loop     ; 13T (taken)
; Total: 38T per element
```

Compare: separate function call per element adds CALL(17T) + RET(10T) = 27T overhead before any work.

### 8.5 mapInPlace Write-Back

```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

The `MutateInPlace` flag triggers a store-back after the lambda body:

```asm
LD A, (HL)     ; load
ADD A, 2       ; transform
LD (HL), A     ; write back
INC HL
DJNZ .loop
```

### 8.6 Known Broken Iterators

- **enumerate:** B register conflict — B is both DJNZ counter and enumeration index.
- **reduce:** A register overwritten between two SMC parameters.

---

## Chapter 9: Zero-Cost Abstractions

Every abstraction in Nanz compiles to direct Z80 instructions. Here's the proof.

### 9.1 UFCS — Zero Cost

`obj.method(args)` desugars to `method(obj, args)` at parse time. The method table is a `map[string]map[string]methodInfo` consulted only during parsing.

**Cost: zero.** `CALL Acc_add` is identical to a hand-written call.

### 9.2 Interfaces — Zero Cost

```nanz
interface Animal { speak }
struct Dog {}
fun Dog.speak(self: Dog) -> u8 { return 1 }
```

No vtable, no fat pointer. At `g_dog.speak()`, the compiler resolves Dog at parse time and emits `CALL Dog_speak`.

**Cost: zero.** 17T for a direct `CALL` vs ~55T for Go-style interface dispatch.

**Limitation:** The concrete type must be statically known. True dynamic dispatch is not implemented (and would violate the zero-overhead principle).

### 9.3 Lambdas — Zero Cost

Each `|x| expr` becomes `lambda_N` — a regular function. When used in iterator chains, the body is inlined. When used as a function pointer, it's a standard `CALL`.

**Cost: zero allocation, zero closure struct.** CALL overhead only when not inlined.

### 9.4 Struct Methods — Zero Cost

`fun Vec2.add(self: Vec2, ...)` is stored as `Vec2_add`. Name mangling is the only "overhead" (compile-time, not runtime). Generated assembly is indistinguishable from a free function.

### 9.5 Operator Overloading — Zero Cost

`a + b` on struct types dispatches to `op_add(a, b)` — a regular function call. No runtime type checking.

---

## Chapter 10: QBE Correctness Oracle

### 10.1 What QBE Is

QBE (https://c9x.me/compile/) is a small, fast compiler backend that converts QBE IL (a simple SSA format) to native x86-64 or arm64 assembly. It is NOT a Nanz target — it is a **testing tool**.

### 10.2 The Correctness Oracle Pipeline

```
source.nanz
    ├─→ HIR → MIR2 → Z80Codegen → MZE emulator → Result A
    └─→ HIR → MIR2 → QBE IL → qbe → cc → native binary → Result B

If A ≠ B: Bug is in Z80 codegen (MIR2 semantics proven correct)
If A = B: Both pipelines agree — correctness confirmed
```

The QBE pipeline stops BEFORE Z80-specific steps (contract optimization, register allocation). QBE does its own register allocation.

### 10.3 E2E Tests

`pkg/mir2qbe/e2e_test.go` contains 7 E2E tests:

| Test | What it verifies |
|------|-----------------|
| `TestE2E_PLM_AbsDiff` | PL/M → HIR → MIR2 → QBE → native |
| `TestE2E_PLM_Fib` | Fibonacci with loop |
| `TestE2E_Nanz_SumArray` | Pointer arithmetic, `ptr[i]` loop |
| `TestE2E_Nanz_AbsDiff` | Control flow (if/else) |
| `TestE2E_Nanz_StructFields` | Global struct field access |
| `TestE2E_Nanz_UFCS` | Method dispatch on globals |
| `TestE2E_Nanz_Interface_ZeroCost` | Interface dispatch monomorphization |

Tests auto-skip if `qbe` is not in PATH (`exec.LookPath("qbe")`).

### 10.4 MIR2 → QBE Translation

Key mappings (`pkg/mir2qbe/codegen.go`):
- All integer types (u8, u16, i8, i16, bool) → QBE `w` (32-bit word)
- `ptr` → QBE `l` (64-bit long, native pointer)
- Block parameters → phi nodes (QBE uses SSA with phi, not block args)
- Z80-specific ops (SMC, push/pop, inline asm) → skipped

### 10.5 Installing QBE

See [Appendix D](#appendix-d-installing-external-tools).

---

## Chapter 11: PL/M-80: Corpus Seeding and Idiomatic Translation

### 11.1 PL/M's Role in Creating the Nanz Ecosystem

PL/M-80 served as the **bootstrap corpus** for the MIR2 backend. The 26 Intel 80 Tools files (ALGOL-M, BASIC-80, ML80 macro assembler, etc.) — all real production code from the 1970s — provided 1,338 functions and 11,661 statements to test the HIR→MIR2→Z80 pipeline before Nanz even existed.

The workflow:

```
Step 1: Parse PL/M corpus (26/26 files, 100% coverage)
Step 2: Lower to HIR → verify correctness
Step 3: Lower to MIR2 → verify optimizations
Step 4: Emit Z80 → compare with Intel's PL/M-80 V4.0 output
Step 5: --emit=nanz → generate Nanz source from HIR
Step 6: Write idiomatic Nanz by hand, guided by the mechanical translation
```

This means every HIR node, every MIR2 optimization, and every Z80 codegen pattern was first validated on **real PL/M code** before Nanz programs used it.

### 11.2 Mechanical Translation: `mz program.plm --emit=nanz`

The `--emit=nanz` flag runs `plm.Compile()` → HIR → `nanz.Print()`, producing syntactically valid Nanz source. The translation is structural — it preserves the PL/M program's logic exactly.

**PL/M source:**
```plm
SUM_ARRAY: PROCEDURE (PTR, N) BYTE;
    DECLARE PTR ADDRESS;
    DECLARE (N, S, I) BYTE;
    S = 0;
    I = 0;
    DO WHILE I < N;
        S = S + PTR(I);
        I = I + 1;
    END;
    RETURN S;
END SUM_ARRAY;
```

**Mechanical Nanz output** (`mz sum.plm --emit=nanz`):
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

**Syntactic mapping:**

| PL/M-80 | Nanz | Notes |
|---------|------|-------|
| `PROCEDURE name(a,b) BYTE;` | `fun name(a: u8, b: u8) -> u8` | Types inline |
| `DECLARE X BYTE;` | `var x: u8` | Lowercase convention |
| `DECLARE (A,B) WORD;` | `var a: u16; var b: u16` | Multi-decl expanded |
| `DO WHILE cond; ... END;` | `while cond { ... }` | |
| `DO I = 0 TO N; ... END;` | `for i in 0..n { ... }` | Counted loop |
| `DO CASE X; ... END;` | `switch x { case 0: ...; }` | |
| `IF cond THEN s1; ELSE s2;` | `if cond { ... } else { ... }` | |
| `ARR(I)` | `arr[i]` | Index notation |
| `CALL fn(a,b);` | `fn(a, b)` | |
| `DECLARE X LITERALLY 'Y'` | *(expanded before parsing)* | Macros gone |

Both compile to **identical Z80 assembly** — same HIR, same MIR2, same codegen.

### 11.3 From Mechanical to Idiomatic: Three Levels

The same sum-of-array algorithm demonstrates the three levels:

**Level 1 — Mechanical PL/M translation** (indexed loop):
```nanz
fun sum_array(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    var i: u8 = 0
    while i < n {
        s = s + ptr[i]      // random-access: ADD HL,DE per element (~15-20T)
        i = i + 1
    }
    return s
}
```

**Level 2 — Idiomatic Nanz** (sequential scan):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    for x: u8 in ptr[0..n] {   // sequential: INC HL per element (6T)
        s = s + x
    }
    return s
}
```

**Level 3 — Iterator chain with closure** (fully fused):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    ptr.forEach(|x: u8| { s = s + x }, n)  // DJNZ loop, s in register
    return s
}
```

**Z80 cost per element:**

| Level | Key instruction | T-states/element | Notes |
|-------|----------------|-----------------|-------|
| 1 (indexed) | `ADD HL, DE` | ~64T | Compute ptr+i every iteration |
| 2 (for-each) | `INC HL` | ~43T | Sequential pointer advance |
| 3 (forEach) | `INC HL` + `DJNZ` | ~38T | Fused, s in register |

At 3.5 MHz with 100 elements: Level 1 = 1.83ms, Level 3 = 1.09ms — **40% faster** from a purely syntactic change.

### 11.4 PL/M Patterns → Nanz Iterator Chains

The most impactful translation: PL/M's manual DO WHILE loops → Nanz iterator chains.

**PL/M: filter + process**
```plm
I = 0;
DO WHILE I < N;
    V = BUF(I) * 2;
    IF V > THRESHOLD THEN CALL PROCESS(V);
    I = I + 1;
END;
```

**Nanz mechanical:**
```nanz
var i: u8 = 0
while i < n {
    let v = buf[i] * 2
    if v > threshold { process(v) }
    i = i + 1
}
```

**Nanz idiomatic (fused chain):**
```nanz
buf.map(|x: u8| (x * 2))
   .filter(|x: u8| (x > threshold))
   .forEach(|x: u8| { process(x) }, n)
```

The chain version: **one DJNZ loop, zero intermediate arrays, all lambdas inlined.** Three stages fused into ~6 Z80 instructions per element.

**PL/M: find maximum**
```plm
MAX = 0;
I = 0;
DO WHILE I < N;
    IF BUF(I) > MAX THEN MAX = BUF(I);
    I = I + 1;
END;
```

**Nanz idiomatic (forEach with capture):**
```nanz
var m: u8 = 0
buf.forEach(|x: u8| {
    if x > m { m = x }
}, n)
```

The captured variable `m` is threaded as a block parameter through the DJNZ loop — it lives in a register, never spills to memory.

**PL/M: transform in-place**
```plm
I = 0;
DO WHILE I < N;
    BUF(I) = BUF(I) + 2;
    I = I + 1;
END;
```

**Nanz idiomatic:**
```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

One loop: load, transform, store back. The `MutateInPlace` flag triggers write-back.

### 11.5 What PL/M Cannot Express

These Nanz features have no PL/M equivalent:

| Nanz feature | PL/M equivalent | Why it matters |
|-------------|----------------|----------------|
| `u8<0..255>` ranged types | None | Enables LUTGen (compile-time table generation) |
| `^Struct` typed pointers | BASED (untyped) | Field resolution, auto-deref, UFCS dispatch |
| `interface Animal { speak }` | None | Compile-time contract, zero-cost dispatch |
| `buf.map().filter().forEach()` | Manual DO WHILE | Single fused loop, no intermediate arrays |
| `\|x\| { s = s + x }` closure capture | None | Loop-carried vars as block params |
| Operator overloading | None | `a + b` on struct types |

### 11.6 PL/M-80 V4.0 vs MIR2: Code Quality

Real comparison (from Report #036) — the **same PL/M source** compiled by Intel's original compiler vs our MIR2 backend:

| Function | Intel PL/M-80 V4.0 | MIR2 Z80 | Savings |
|----------|-------------------|----------|---------|
| `abs_diff` | 33 bytes | 12 bytes | **−64%** |
| `fib` | 47 bytes | 31 bytes | **−34%** |
| **Total** | **80 bytes** | **43 bytes** | **−46%** |

Intel's compiler stores all parameters and locals in memory (8080 calling convention). MIR2's register-first ABI keeps values in A/B/C/D/HL — zero memory traffic in hot loops.

---

## Chapter 12: Compiled Output Gallery

Every code block below is **actual compiler output** from `mz <file>.nanz -o <file>.a80` on the current master build (2026-03-10). Source files archived in `reports/showcase-src/2026-03-10/`.

### 12.1 Struct Field Access — HL-Chain Optimization

**Source** (`ex1_struct.nanz`):
```nanz
struct Color { r: u8, g: u8, b: u8 }
global palette: Color

fun set_rgb(rv: u8, gv: u8, bv: u8) -> void {
    palette.r = rv
    palette.g = gv
    palette.b = bv
}
fun get_r() -> u8 { return palette.r }
fun get_g() -> u8 { return palette.g }
fun get_b() -> u8 { return palette.b }
```

**Compiled Z80** (`ex1_struct.a80`):
```z80
set_rgb:
    LD HL, palette      ; one base load for all three fields     10T
    LD (HL), C          ; palette.r = rv                          7T
    INC HL              ;                                         6T
    LD (HL), D          ; palette.g = gv                          7T
    INC HL              ;                                         6T
    LD (HL), E          ; palette.b = bv                          7T
    RET                 ;                                        10T

get_r:
    LD A, (palette__r)  ; direct addressing via EQU label        13T
    RET
get_g:
    LD A, (palette__g)
    RET
get_b:
    LD A, (palette__b)
    RET

; globals
palette:
    DB 0, 0, 0
palette__r    EQU  palette
palette__g    EQU  palette + 1
palette__b    EQU  palette + 2
```

**Optimization:** HL-chain detection fuses three field stores into one `LD HL` + `INC HL` sequence: 53T vs 79T naive (−33%).

### 12.2 UFCS Method Dispatch — Zero Vtable

**Source** (`ex2_ufcs.nanz`):
```nanz
struct Acc { val: u8 }
global acc_g: Acc

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}
fun Acc.reset(self: ^Acc) -> void { self.val = 0 }

fun sum_two(a: u8, b: u8) -> u8 {
    acc_g.reset()       // UFCS → Acc_reset(&acc_g)
    acc_g.add(a)        // UFCS → Acc_add(&acc_g, a)
    acc_g.add(b)
    return acc_g.val
}
```

**Compiled Z80** (`ex2_ufcs.a80`):
```z80
Acc_add:
    LD D, (HL)          ; load self.val (HL = pointer to Acc)
    LD A, D
    ADD A, C            ; + amount (C = 2nd param)
    LD C, A
    LD (HL), C          ; store back
    LD A, (HL)          ; return value
    RET

Acc_reset:
    LD C, 0
    LD (HL), C          ; self.val = 0
    RET

sum_two:
    LD HL, acc_g        ; addr_of(acc_g) — direct CALL, no vtable
    CALL Acc_reset
    LD HL, acc_g
    LD A, C             ; a
    CALL Acc_add
    LD HL, acc_g
    LD A, mem           ; b  (known bug: register spill for 2nd arg)
    CALL Acc_add
    LD A, (acc_g__val)  ; direct-address return
    RET

; globals
acc_g:
    DB 0
acc_g__val    EQU  acc_g
```

### 12.3 Zero-Cost Interface Dispatch

**Source** (`ex3_iface.nanz`):
```nanz
interface Animal { speak }
struct Dog {}
struct Cat {}
global g_dog: Dog
global g_cat: Cat

fun Dog.speak(self: Dog) -> u8 { return 1 }
fun Cat.speak(self: Cat) -> u8 { return 2 }

fun demo() -> u8 { return g_dog.speak() }
```

**Compiled Z80** (`ex3_iface.a80`):
```z80
Dog_speak:
    LD C, 1
    LD A, C
    RET

Cat_speak:
    LD C, 2
    LD A, C
    RET

demo:
    LD HL, g_dog
    CALL Dog_speak      ; direct CALL — no vtable, no indirection
    RET
```

17T for the dispatch (CALL alone). Go-style interface dispatch: ~55T.

### 12.4 abs_diff — Five Optimization Passes to the Minimum

**Source** (`ex4a_abs_diff.nanz`):
```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a == b { return 0 }
    if a < b { return b - a }
    return a - b
}
```

**Compiled Z80** (`ex4a_abs_diff.a80`):
```z80
abs_diff:
    SUB D               ; A = a - b, carry set if a < b
    LD C, A             ; (regression: contract assigned b→D, result→C)
    RET NC              ; a >= b → return a-b
.abs_diff_if_then3:
    NEG                 ; A = -(a-b) = b-a
    RET
```

BranchEquiv eliminated the `a == b` guard. CondRetSink hoisted `sub` before `cmp`. CmpSubCarry eliminated `CP`. Result: 4 core instructions.

### 12.5 LUTGen — Compile-Time Loop → 3-Instruction Lookup

**Source** (`ex5_lut.nanz`):
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

**Compiled Z80** (`ex5_lut.a80`):
```z80
popcount:
    LD H, popcount_lut^H    ; 7T — page base (high byte only)
    LD L, C                 ; 4T — index (param in C)
    LD A, (HL)              ; 7T — table lookup
    RET

    ALIGN 256
popcount_lut:
    DB 0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3, 4, ...  ; 256 bytes
```

**18T total.** The while loop never runs at runtime — it was evaluated at compile time by the MIR2 VM for all 256 inputs.

### 12.6 forEach with Closure Capture — DJNZ Loop

**Source** (`ex6_foreach.nanz`):
```nanz
fun sum_chain(buf: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    buf.forEach(|x: u8| { s = (s + x) }, n)
    return s
}

fun max_chain(buf: ^u8, n: u8) -> u8 {
    var m: u8 = 0
    buf.forEach(|x: u8| {
        if x > m { m = x }
    }, n)
    return m
}
```

**Compiled Z80** (`ex6_foreach.a80`):
```z80
sum_chain:
    LD D, 0             ; s = 0
    LD B, C             ; B = n (DJNZ counter)
    LD C, D             ; C = s
.sum_chain_fe_head1:
    LD A, B
    AND A               ; n == 0?
    JRS Z, .sum_chain_fe_exit4
.sum_chain_fe_body2:
    LD D, (HL)          ; x = *buf
    LD A, C
    ADD A, D            ; s + x  (lambda body inlined)
    LD C, A
.sum_chain_fe_cont3:
    INC HL              ; buf++
    DJNZ .sum_chain_fe_body2
.sum_chain_fe_exit4:
    LD A, C
    RET

max_chain:
    LD D, 0             ; m = 0
    LD B, C             ; B = n
    LD C, D             ; C = m
.max_chain_fe_head1:
    LD A, B
    AND A
    JRS Z, .max_chain_fe_exit4
.max_chain_fe_body2:
    LD D, (HL)          ; x = *buf
    LD A, C
    CP D                ; m > x?
    JRS NC, .max_chain_trmp0
.max_chain_if_then5:
.max_chain_fe_cont3:
    INC HL
    DEC B
    LD C, D             ; m = x (captured var update)
    JRS .max_chain_fe_head1
.max_chain_fe_exit4:
    LD A, C
    RET
.max_chain_trmp0:
    LD D, C
    JRS .max_chain_if_join6
```

**sum_chain: 38T/element.** No CALL to lambda. Captured `s` lives in C throughout.

### 12.7 mapInPlace — In-Place Transform with Write-Back

**Source** (`ex7_mapinplace.nanz`):
```nanz
fun add2_inplace(buf: ^u8, n: u8) -> void {
    buf.mapInPlace(|x: u8| (x + 2), n)
}
```

**Compiled Z80** (`ex7_mapinplace.a80`):
```z80
add2_inplace:
    LD B, C             ; B = n
.add2_inplace_fe_head1:
    LD A, B
    AND A
    JRS Z, .add2_inplace_fe_exit4
.add2_inplace_fe_body2:
    LD C, (HL)          ; load element
    LD D, 2             ; (dead load — regression)
    INC C               ; +1
    INC C               ; +1 (INC C × 2 instead of ADD A,2)
    LD (HL), C          ; write back
.add2_inplace_fe_cont3:
    INC HL
    DJNZ .add2_inplace_fe_body2
.add2_inplace_fe_exit4:
    RET

lambda_0:               ; standalone lambda emitted but never called
    LD C, 2
    ADD A, C
    LD C, A
    RET
```

**Known regression:** `LD D, 2` is dead, `INC C; INC C` replaces `ADD A, 2` (51T vs 40T/element). The standalone `lambda_0` is also dead code.

### 12.8 GCD — Loop with Two Mutable Variables

**Source** (`ex8_gcd.nanz`):
```nanz
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}
```

**Compiled Z80** (`ex8_gcd.a80`):
```z80
gcd:
.gcd_loop_head1:
    LD A, C
    CP D                ; a == b?
    JRS Z, .gcd_trmp0
.gcd_loop_body2:
    LD A, D
    CP C                ; a > b?
    JRS NC, .gcd_if_else6
.gcd_if_then4:
    LD A, C
    SUB D               ; a = a - b
    LD C, A
.gcd_if_join5:
    JRS .gcd_loop_head1
.gcd_if_else6:
    LD A, D
    SUB C               ; b = b - a
    LD D, A
    JRS .gcd_if_join5
.gcd_loop_exit3:
    LD A, D
    RET
.gcd_trmp0:
    LD A, C
    LD C, D
    LD D, A
    JRS .gcd_loop_exit3
```

Phase 6c coalescer eliminated all block-boundary copy overhead from the hot loop. Only the exit trampoline (cold path) has register swaps.

---

## Chapter 13: Relation to MinZ and PL/M

### 13.1 The Three Frontends

| Language | Extension | Parser | Backend | Status |
|----------|-----------|--------|---------|--------|
| MinZ | `.minz` | `pkg/parser/participle/` | MIR1 → old codegen | Frozen |
| Nanz | `.nanz` | `pkg/nanz/parse.go` | HIR → MIR2 → Z80 | **Active** |
| PL/M-80 | `.plm` | `pkg/plm/` | HIR → MIR2 → Z80 | Active |

The router in `cmd/minzc/main.go`:
```go
if ext == ".plm" || ext == ".nanz" {
    return compileViaHIR(src, ext)
} else {
    // old MIR1 path for .minz
}
```

### 13.2 When to Use Which

**Nanz** — new Z80/CP/M programs, modern syntax, LUTGen, PBQP allocation.
**MinZ** — existing `.minz` programs that work. Has metafunctions (`@define`, `@print`) not yet in Nanz.
**PL/M-80** — porting legacy CP/M software. 100% of Intel 80 Tools corpus parses.

### 13.3 Feature Gaps (Nanz vs. MinZ)

Nanz currently lacks:
- `@define` preprocessor macros
- `@print` optimized string output
- Ruby-style string interpolation (`"Hello #{name}"`)
- Multiple return values syntax (MIR2 supports it, parser doesn't)
- Function overloading (MinZ has it; Nanz requires distinct names)
- `@extern` with register class annotations (documented but not yet parsed)

### 13.4 Other Backends (MinZ-only)

The MinZ old pipeline (`-b` flag) supports multiple targets. These are NOT available for Nanz (which always goes through MIR2 → Z80):

| Backend | Flag | Status | Output |
|---------|------|--------|--------|
| Z80 | `-b z80` | Production | `.a80` |
| 6502 | `-b 6502` | Beta | `.s` |
| 68000 | `-b m68k` | Alpha | `.s` |
| i8080 | `-b i8080` | Beta | `.s` |
| Game Boy | `-b gb` | Active | `.s` |
| WASM | `-b wasm` | Alpha | `.wat` |
| C | `-b c` | Beta | `.c` |
| Crystal | `-b crystal` | Beta | `.cr` |
| LLVM | `-b llvm` | Planned | `.ll` |

These use the old IR (MIR1), not MIR2. The long-term plan is to retire MIR1 and route all frontends through HIR → MIR2.

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

fun_decl       = ('fun' | 'fn') fun_decl_inner
fun_decl_inner = (op_symbol | IDENT ('.' IDENT)?) '(' params ')' ('->' type)?
                 ('{' stmt* '}' | /* extern: no body */)
params         = (IDENT ':' type (',' IDENT ':' type)*)?
op_symbol      = '+' | '-' | '*' | '/' | '%'
               | '==' | '!=' | '<' | '<=' | '>' | '>='
               | '&' | '|' | '^'

type           = '^' type
               | '[' type ';' INT ']'
               | 'u8' ('<' INT '..' INT '>')?
               | 'u16' ('<' INT '..' INT '>')?
               | 'i8' | 'i16' | 'bool' | 'void' | 'ptr'
               | IDENT     (* struct or interface name *)

stmt           = var_decl | let_decl | if_stmt | while_stmt
               | for_stmt | return_stmt | 'break' | 'continue'
               | switch_stmt | block | expr_stmt

var_decl       = 'var' IDENT ':' type at_clause? ('=' (array_init | expr))?
let_decl       = 'let' IDENT (':' type)? '=' expr
array_init     = '[' expr (',' expr)* ']'

if_stmt        = 'if' expr block ('else' block)?
while_stmt     = 'while' expr block
for_stmt       = 'for' IDENT (':' type)? 'in'
                 (expr '[' expr? '..' expr ']' block    (* ForEachStmt *)
                 | expr '..' expr block)                 (* ForRangeStmt *)
return_stmt    = 'return' expr?
switch_stmt    = 'switch' expr '{' case_clause* default_clause? '}'
case_clause    = 'case' INT ':' stmt*
default_clause = 'default' ':' stmt*
block          = '{' stmt* '}'

expr_stmt      = expr ('=' expr)?     (* assignment or bare call *)

expr           = binary_expr
binary_expr    = unary_expr (binop binary_expr)*
binop          = '+' | '-' | '*' | '/' | '%' | '&' | '|' | '^'
               | '<<' | '>>' | '==' | '!=' | '<' | '<=' | '>' | '>='

unary_expr     = '-' unary_expr
               | '!' unary_expr
               | '~' unary_expr
               | '&' IDENT
               | postfix_expr

postfix_expr   = primary
                 ( '^'                      (* dereference *)
                 | '[' expr ']'             (* index *)
                 | '.' IDENT               (* field access *)
                 | '.' IDENT '(' args ')'  (* UFCS method call *)
                 | '(' args ')'            (* function call *)
                 )*

primary        = INT | 'true' | 'false' | STRING
               | ('u8' | 'u16' | 'i8' | 'i16') '(' expr ')'   (* cast *)
               | '@ptr' '(' type ',' expr ')'
               | '|' lambda_params '|' (block | expr)
               | '(' expr ')'
               | IDENT

lambda_params  = (IDENT (':' type)? (',' IDENT (':' type)?)*)?
args           = (expr (',' expr)*)?
```

**Lexical notes:**
- Comments: `//` (line) and `/* */` (block)
- Whitespace is insignificant
- Integers: decimal or `0x`/`0X` hexadecimal
- Strings: double-quoted, no escape sequences

---

## Appendix B: Register Classes and Cost Table

### Z80 Physical Registers

| Register | Class | Cost | Notes |
|----------|-------|------|-------|
| A | ClassAcc | 0T | ALU accumulator, return value, 1st u8 param |
| B | ClassCounter | 0T | DJNZ counter, 3rd param |
| C, D, E, H, L | ClassGeneral | 0T | General 8-bit |
| HL | ClassPointer | 0T | Primary pointer, 1st u16/ptr param, return |
| DE | ClassIndex | 0T | 2nd u16 param, LDIR source |
| BC | ClassPair | 0T | 3rd u16 param |
| IX | ClassIX | 8T | Overflow pointer (+4T DD prefix per access) |
| IY | ClassIY | 8T | Rarely used (system reserved on some platforms) |
| IXH/IXL | ClassIXY8 | 8T | Undocumented 8-bit halves |
| $F0xx | ClassMem | 26T | Memory-backed "register file extension" |

### Register Class Hierarchy

| Tier | Classes | Cost | Mechanism |
|------|---------|------|-----------|
| 0 — Primary | Acc, Counter, General, Pointer, Index, Pair, Flag | 0T | Direct use |
| 1 — IX/IY | IX, IY, IXY8 | 4-8T | DD/FD prefix |
| 2 — Shadow | Shadow, AccShadow | 8T | EXX / EX AF,AF' |
| 3 — Stack | Stack | 21T | PUSH + POP |
| 4 — Memory | Mem | 26T | LD (addr) / LD addr |

ClassFlag is special: it represents Z80 CPU flags (Z, CY, etc.) and costs 0T. Boolean results from comparisons are kept in flags without materializing to a register.

---

## Appendix C: CLI Reference

### Compiling Nanz Programs

```bash
mz source.nanz -o output.a80              # compile to Z80 assembly
mz source.nanz -o output.a80 --target=cpm # target CP/M
mz source.nanz -o output.a80 --target=spectrum  # target ZX Spectrum
mz source.nanz -o output.a80 --target=agon      # target Agon Light 2
```

### Intermediate Representations

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz translation
```

### Optimization Flags

```bash
mz source.nanz --disable-optimize      # disable all optimizations
mz source.nanz --disable-ir-opt        # disable MIR-level opts
mz source.nanz --disable-asm-opt       # disable peephole
mz source.nanz --disable-smc           # disable self-modifying code
mz source.nanz --compile-trace         # show all optimization steps
```

### Running Tests

```bash
cd minzc

# All Go tests (23+ packages)
go test ./pkg/... -vet=off

# Nanz parser tests only
go test ./pkg/nanz/... -vet=off -v

# MIR2 tests (LUTGen, contracts, PBQP)
go test ./pkg/mir2/... -vet=off -v

# QBE E2E tests (requires qbe and cc in PATH)
go test ./pkg/mir2qbe/... -vet=off -v
```

### Example Programs

| File | Description |
|------|-------------|
| `examples/nanz/01_sum_array.nanz` | While loop with `ptr[i]` |
| `examples/nanz/02_sum_array_idiomatic.nanz` | For-each and forEach iterator |
| `examples/nanz/03_filter_map_chain.nanz` | Full map/filter/forEach chain |
| `examples/nanz/04_lut_popcount.nanz` | LUT generation via ranged type |
| `examples/nanz/05_four_pointers.nanz` | PBQP: 4 ClassPointer regs |
| `examples/nanz/06_pbqp_weighted.nanz` | Weighted cost allocation |
| `examples/nanz/07_ix_load_store.nanz` | IX overflow addressing |

---

## Appendix D: Installing External Tools

### QBE (Correctness Oracle)

QBE is needed only for running E2E correctness tests (`pkg/mir2qbe/`). It is NOT needed for normal Nanz compilation.

**Linux (build from source):**
```bash
git clone git://c9x.me/qbe.git
cd qbe
make
sudo cp qbe /usr/local/bin/
```

**macOS:**
```bash
brew install qbe
```

**Verify:**
```bash
qbe --version        # should print version
echo 'export function w $main() { @start ret 0 }' | qbe
```

**C compiler** is also needed (any C99 compiler: `gcc`, `clang`). Usually pre-installed.

If `qbe` is not in PATH, the E2E tests auto-skip with `t.Skip("qbe not in PATH")`.

### MZA Assembler (Built-in)

No external installation needed. MZA is part of the MinZ toolchain:

```bash
cd minzc && make mza
# or: make install-user (installs all tools to ~/.local/bin/)
```

### MZE Emulator (Built-in)

```bash
cd minzc && make mze
```

Used for: running compiled Z80 binaries, constant evaluation inside the compiler (LUTGen, BranchEquiv).

---

## Appendix E: Known Bugs and Limitations

### Parser

| Issue | Status | Workaround |
|-------|--------|------------|
| `@extern` with `params=`/`returns=` annotations | Not implemented | Use basic `@extern fun` (auto register assignment) |
| Function overloading | Not implemented | Use distinct names (`abs_diff`, `abs_diff_u16`) |
| Multiple return values | Not parsed | MIR2 supports it; parser doesn't |
| String escape sequences | Not implemented | No `\n`, `\t` etc. |
| `@asm` inline assembly blocks | Not implemented | Use `@extern` with asm wrappers |

### Codegen

| Issue | Status | Details |
|-------|--------|---------|
| `applySubSwapNeg` on u16 | Bug | Forces ClassAcc (8-bit) on 16-bit NEG result. Guard `Ty.Width() <= 8` missing. |
| Zero-size struct globals | Bug | `struct Dog {}` emits no data; symbol undefined at link time. Fix: emit `Dog: EQU $`. |
| `LD A, mem` sentinel | Bug | Register allocation failure emits literal string "mem" in assembly. |
| Caller-save for 2nd arg | Bug | Second argument to repeated calls may be clobbered. |
| mapInPlace constant-add | Regression | `ADD A, imm` not firing when element is in C (not A). |

### Iterator Chains

| Issue | Status | Details |
|-------|--------|---------|
| `enumerate` | Broken on Z80 | B = counter conflicts with B = enumeration index |
| `reduce` | Broken on Z80 | A overwritten between two SMC parameters |
| Non-fused closure capture | Undefined behavior | Lambdas passed as function pointers cannot capture outer locals |

### Register Allocator

| Issue | Status | Details |
|-------|--------|---------|
| Memory-backed registers | Performance | ~207T actual vs ~43T ideal per iterator element when regs spill to $F0xx |
| Contract optimizer drift | Known | Can assign suboptimal classes (e.g., `b → D` instead of `b → B` for abs_diff) |
| PBQP edge costs | Deferred | Correlated allocation decisions (BC★ for LUT) not yet modeled |

---

*Nanz: Modern syntax, zero-cost abstractions, Z80 performance.*

*Pipeline: `.nanz` → `nanz.Parse()` → `*hir.Module` → `hir.LowerModule()` → `*mir2.Module` → optimization → allocation → Z80Codegen → `.a80`*
