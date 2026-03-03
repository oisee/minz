# MIR Language Compatibility — Why Different Languages Score Differently

**Date:** 2026-03-03
**Depends on:** [Report #020 — MIR Analysis](2026-03-02-020-MIR_Analysis_Multi_Language_IR_Feasibility.md)

---

## The Scores and Why They Differ

```
PL/M-80 ████████░░ 9/10   — almost identical worldview
BASIC   ██████░░░░ 6/10   — good fit minus floats
Pascal  ██████░░░░ 6/10   — sets and strings need work
Forth   █████░░░░░ 5/10   — stack vs register mismatch
Ada     ████░░░░░░ 4/10   — runtime requirements too heavy
```

Each language bumps into a different wall. The walls are:

1. **Execution model** — register machine vs stack machine
2. **Type richness** — how much the language demands beyond u8/u16/struct
3. **Runtime requirements** — what the language expects to exist at runtime
4. **Error model** — how errors propagate
5. **Memory model** — static vs dynamic allocation patterns

---

## PL/M-80: 9/10 — The Twin

PL/M-80 was designed for Intel 8080. Z80 is 8080-superset. MIR was designed for Z80. So PL/M and MIR share the same universe:

| PL/M concept | MIR equivalent | Gap? |
|---|---|---|
| `BYTE` | `TypeU8` | None |
| `ADDRESS` | `TypeU16` | None |
| `DECLARE x BYTE` | `OpLoadConst` + `OpStoreVar` | None |
| `DECLARE a(10) BYTE` | `ArrayType{U8, 10}` | None |
| `DECLARE s STRUCTURE (x BYTE, y BYTE)` | `StructType` | None |
| `BASED p` (pointer dereference) | `OpLoadPtr`/`OpStorePtr` | None |
| `DO WHILE cond` | `OpLabel` + `OpJumpIf` | None |
| `DO CASE expr` | Multiple `OpJumpIf` | None |
| Procedures | `Function` + `OpCall` | None |
| `INTERRUPT 7` | `Function.IsInterrupt` | None |
| `AT (0FF00H)` | `OpLoadDirect`/`OpStoreDirect` | None |
| `LITERALLY` (macro) | Preprocessor layer | N/A |
| `EXTERNAL`/`PUBLIC` | Weak — `Module` has no visibility | **-0.5** |
| `REENTRANT` procedures | Stack alloc exists but SMC is default | **-0.5** |

**Why 9 and not 10**: PL/M's module system has `EXTERNAL`/`PUBLIC` linkage that MIR's `Module` doesn't model. And PL/M's `REENTRANT` keyword maps awkwardly to MIR where SMC (non-reentrant) is the default. Both are trivial fixes.

**What to fix for 10/10**:
- Add `IsPublic bool` field to `Function` and `Global` → 5 minutes
- Add `REENTRANT` → disable SMC for that function (`IsSMCEnabled = false`) → already supported

---

## BASIC (Integer): 6/10 — Print is Finally Useful

Integer BASIC (no floating point) maps surprisingly well:

| BASIC concept | MIR equivalent | Gap? |
|---|---|---|
| `LET A = 42` | `OpLoadConst` + `OpStoreVar` | None |
| `A + B * C` | `OpAdd`, `OpMul` | None |
| `DIM A(100)` | `ArrayType` | None |
| `PRINT A` | `OpPrintU8`/`OpPrintU16` | **Perfect** |
| `PRINT "Hello"` | `OpPrintString` | **Perfect** |
| `IF A > 5 THEN` | `OpGt` + `OpJumpIf` | None |
| `FOR I = 1 TO 10` | `OpLabel` + `OpInc` + `OpLt` + `OpJumpIf` | None |
| `GOSUB 1000` / `RETURN` | `OpCall` + `OpReturn` | None |
| `GOTO 500` | `OpJump` | None |
| `PEEK(addr)` / `POKE addr,val` | `OpLoadDirect`/`OpStoreDirect` | None |
| `INPUT A` | None — need I/O read | **-1** |
| `A$ = "hello"` (strings) | `StringType` exists but limited | **-1** |
| Floating point (`3.14`) | None | **-2** |
| Line number dispatch | Needs label table | **-0** (trivial) |

**Why 6 and not 8**: Two significant gaps:

1. **No floating point** (-2 points). BASIC lives on float arithmetic. `10 PRINT SIN(X)` — impossible without float types. MIR has fixed-point (`f8.8`) which covers *some* use cases (graphics, simple math) but not general `PRINT 3.14159`. Adding IEEE float to an 8-bit IR is a major undertaking (software float library + new types + new ops).

2. **No INPUT** (-1 point). BASIC's `INPUT` needs blocking character read. MIR has `OpPortIn` but no high-level "read line from console" concept. Solvable with a runtime library.

3. **String operations** (-1 point). BASIC strings are dynamic (`A$ = A$ + B$`, `MID$(A$,3,5)`). MIR's `StringType` is a pointer to length-prefixed data — concatenation/slicing would need runtime heap management.

**The irony**: `OpPrintU8`, `OpPrintString` — the opcodes that are "wrong" for a portable IR — are *exactly* what BASIC needs. `10 PRINT 42` maps directly to `OpPrintU8`.

**What to fix for 8/10**:
- Add `OpReadChar` / `OpReadLine` — simple I/O input opcodes → 1 day
- String runtime library (concat, mid$, left$, right$, len) → 1 week
- *Not* adding float — that's a 3/10 → 8/10 jump on its own and would double the IR complexity

---

## Pascal: 6/10 — Close But Missing Sets and Strings

Pascal and MIR share a structured-programming worldview, but Pascal demands more from its type system:

| Pascal concept | MIR equivalent | Gap? |
|---|---|---|
| `integer` (16-bit) | `TypeI16` | None |
| `byte` | `TypeU8` | None |
| `boolean` | `TypeBool` | None |
| `char` | `TypeU8` | None |
| `array[1..10] of byte` | `ArrayType` | None |
| `record` | `StructType` | None |
| `pointer` / `^T` | `PointerType` | None |
| `procedure` / `function` | `Function` | None |
| Nested procedures | `Function.ParentFunction` + `CapturedVars` | **Works** |
| `if/then/else`, `while`, `for` | Control flow ops | None |
| `case` | Multiple `OpJumpIf` | None |
| `set of 0..7` | None | **-1.5** |
| `string[255]` (length-prefixed) | `StringType` (same model!) | Partial |
| `type Color = (Red, Green, Blue)` | `EnumType` | None |
| `type Range = 1..100` | None — no subrange types | **-1** |
| `with record do` | Syntactic sugar | N/A |
| `new`/`dispose` | `OpAlloc`/`OpFree` | None |
| `file of T` | None — no file I/O model | **-0.5** |
| Runtime range checking | None — no trap/check opcodes | **-1** |

**Why 6 and not 8**:

1. **No SET type** (-1.5). Pascal's `set of` is fundamental — `if color in [Red, Green]`. On Z80, sets of 0..7 fit in a byte (bit operations), sets of 0..15 in a word. MIR has bitwise ops (`OpAnd`/`OpOr`/`OpXor`) but no set abstraction. The semantic layer would need to lower sets to bitmasks.

2. **No subrange types** (-1). `type Month = 1..12` with runtime bounds checking is core Pascal. MIR has no runtime check-and-trap mechanism. Would need `OpCheckRange(reg, min, max, trap_label)`.

3. **No runtime range checking** (-1). Related to above. Pascal expects array bounds checking, assignment range checking, etc. MIR assumes all accesses are valid (C-style).

**What's surprisingly good**: Pascal's string model (length byte + data) is *exactly* MIR's `StringType`. Turbo Pascal compatibility would be high.

**What to fix for 8/10**:
- Add `OpCheckRange(src, imm_lo, imm_hi, label)` — runtime bounds check → 1 day
- Lower `set of` to bitmask operations in the frontend (no IR changes) → 1 week
- File I/O via `OpSyscall` abstraction → already exists

---

## Forth: 5/10 — The Stack Problem

Forth is a stack-based language. MIR is a register-based IR. This is a fundamental mismatch:

```forth
: SQUARE ( n -- n*n )  DUP * ;
3 4 + SQUARE .
```

Forth operates on an implicit data stack. Every word pushes/pops. MIR operates on named virtual registers with explicit operands.

| Forth concept | MIR equivalent | Gap? |
|---|---|---|
| Data stack (TOS, NOS) | `OpPush`/`OpPop` exist but... | **-2** |
| Return stack (>R, R>) | No separate return stack | **-1** |
| `+ - * /` | `OpAdd`/`OpSub`/`OpMul`/`OpDiv` | None (after stack→reg) |
| `@ !` (fetch/store) | `OpLoad`/`OpStore` | None |
| `IF THEN ELSE` | `OpJumpIf` + labels | None |
| `DO LOOP` | `OpDJNZ` or label loop | None |
| `: word ;` (colon def) | `Function` | None |
| `VARIABLE` / `CONSTANT` | `Global` | None |
| `CREATE DOES>` | None — runtime word creation | **-1** |
| `IMMEDIATE` | None — compile-time execution | **-0.5** |
| `' word EXECUTE` | `OpCallIndirect` | **Works** |
| `ALLOT` | `OpAlloc` | None |
| `C@ C!` (byte fetch/store) | `OpLoad`/`OpStore` with u8 type | None |

**Why 5 and not 7**:

1. **Stack-to-register translation** (-2). The core problem. Forth's `DUP SWAP OVER ROT` manipulate an implicit stack. In register IR:

   ```forth
   : example  DUP + ;    ( n -- 2n )
   ```
   ```
   ; Forth native:              ; MIR translation:
   DUP    ( n -- n n )          r1 = load_param "n"
   +      ( n n -- 2n )         r2 = move r1         ; DUP
                                 r3 = add r1, r2      ; +
                                 return r3
   ```

   Every Forth program needs a **stack scheduler** that maps the implicit stack to virtual registers. This is solvable (the C backend already proves register-based IRs can compile stack languages) but adds a complex translation layer.

2. **No separate return stack** (-1). Forth's return stack (`>R`, `R>`, `R@`) is distinct from the data stack. MIR has one stack (`OpPush`/`OpPop`). Would need either two stack pointers or a register-allocated return stack emulation.

3. **No CREATE DOES>** (-1). Forth's metacircular features require runtime word creation — defining new functions at runtime. MIR is a static, ahead-of-time IR. No JIT, no runtime code generation (ironic, given SMC support — but SMC patches existing code, doesn't create new words).

**What to fix for 7/10**:
- Stack scheduler in the Forth frontend (map TOS to register, NOS to register, spill deeper items) → standard technique, 2 weeks
- Model return stack as a second `OpPush`/`OpPop` with a flag → 1 day
- Accept that CREATE DOES> won't work in a static compiler (same limitation as most Forth-to-native compilers like GForth's ahead-of-time mode)

---

## Ada: 4/10 — The Runtime Gap

Ada requires things that don't exist on Z80 and that MIR doesn't model:

| Ada concept | MIR equivalent | Gap? |
|---|---|---|
| `Integer`, `Natural`, `Positive` | `TypeI16`/`TypeU16` | Partial (no constraint) |
| `type Temp is range -40..125` | None | **-1.5** |
| `type Day is (Mon, Tue, ... Sun)` | `EnumType` | None |
| `type Vector is array(1..3) of Float` | `ArrayType` | Partial (no bounds) |
| `record` | `StructType` | None |
| `access T` (pointers) | `PointerType` | None |
| `procedure`/`function` | `Function` | None |
| `package` | `Module` | Weak |
| `generic` packages | `GenericType` (monomorphization) | **Works** |
| `if/elsif/else`, `loop`, `for`, `while` | Control flow ops | None |
| `case` with full coverage | Multiple `OpJumpIf` | None |
| `exception` / `raise` / `when` | `OpSetError`/`OpCheckError` | **-2** |
| `task` / `entry` / `accept` | None | **-2** |
| `pragma Restrictions` | N/A (compile-time) | N/A |
| `delta 0.01 range 0.0..1.0` (fixed-point) | `f8.8` etc. | **Excellent** |
| Runtime checks (overflow, range, null) | None | **-1** |
| Elaboration order | None | **-0.5** |
| `'First`, `'Last`, `'Range` attributes | None (compile-time computable) | **-0** |

**Why 4 and not 7**:

1. **Exception handling** (-2). Ada's exceptions are non-negotiable — `raise Constraint_Error` must propagate up the call stack, unwinding as it goes. MIR has `OpSetError` (set carry flag) and `OpCheckError` (test carry flag) — this is a **single-level, value-based** error model. Ada needs **stack unwinding**: `raise` in function F must find the nearest `when` handler, potentially many frames up.

   On Z80 with 64KB RAM, a full exception runtime (exception tables, stack unwinding, handler dispatch) costs ~500-1000 bytes of code + runtime overhead on every function entry/exit. Heavy, but Ada expects it.

   MIR would need:
   ```
   OpThrow       symbol          — raise named exception
   OpCatchBegin  label           — begin try block
   OpCatchEnd                    — end try block
   OpHandler     symbol, label   — when Exception => goto label
   ```

2. **Tasking (concurrency)** (-2). Ada tasks are a core language feature, not a library:
   ```ada
   task Sensor_Reader is
       entry Read(Value : out Integer);
   end Sensor_Reader;
   ```
   On Z80, this means cooperative multitasking with stack switching. MIR has no concept of multiple execution contexts, task priorities, rendezvous, or protected objects. This is arguably out of scope for any 8-bit IR — even GNAT doesn't support tasking on all platforms.

3. **Runtime constraint checks** (-1). Every assignment in Ada can trigger a `Constraint_Error`:
   ```ada
   X : Integer range 1..10 := 15;  -- Constraint_Error!
   ```
   MIR has no check-and-trap mechanism. Needs `OpCheckRange` (same as Pascal).

4. **Elaboration** (-0.5). Ada packages have elaboration code that runs at startup in dependency order. MIR modules don't model initialization ordering.

**The bright spot**: Ada's fixed-point types map beautifully to MIR's `f8.8`/`f16.8`:
```ada
type Fraction is delta 0.01 range 0.0 .. 1.0;  -- maps to f.8 or f.16
type Angle is delta 0.1 range 0.0 .. 359.9;     -- maps to f8.8
```
This is MIR's **unique advantage** over every other 8-bit IR. SDCC, cc65, z88dk — none have native fixed-point. Ada on Z80 with native fixed-point math would be genuinely novel.

**What to fix for 6/10**:
- Add exception opcodes (Throw/CatchBegin/CatchEnd/Handler) + small runtime → 2-3 weeks
- Add `OpCheckRange` → 1 day
- Accept no tasking (target "Ravenscar profile" — restricted Ada subset for embedded, no dynamic tasks)
- Package elaboration via init function ordering → 1 week

---

## Comparison with Other 8-bit IRs

### SDCC iCode

**What it is**: SDCC (Small Device C Compiler) uses iCode as its IR. It targets Z80, 8051, STM8, HC08, PIC, and others.

| Aspect | SDCC iCode | MinZ MIR |
|--------|------------|----------|
| **Form** | Tree-based with linked list | Flat instruction list |
| **SSA** | No (partial, symbol-based) | No (register-based) |
| **Opcodes** | ~80 (estimated from source) | 118 |
| **Types** | C types (char, int, long, float) | 14 basic + fixed-point |
| **Multi-target** | Yes (6+ backends) | Yes (7 backends) |
| **Register model** | Abstract, backend maps | Virtual + Z80 hints |
| **Optimization** | ~10 passes (CSE, loop inv, DCE) | 13+ passes + assembly |
| **SMC** | No | 14 opcodes |
| **Fixed-point** | No | 5 native types |
| **VM/Interpreter** | No | Yes (4.6K LOC) |
| **Self-modifying code** | No | First-class |

**Key difference**: iCode is **language-agnostic** by design — it models C semantics without hardware coupling. This makes multi-target easy but Z80 codegen generic. MinZ MIR is **Z80-optimized** — register hints, DJNZ, SMC make Z80 code excellent but portability harder.

**iCode's advantage**: True multi-target. Same C code compiles to Z80, 8051, STM8 without source changes. The IR doesn't assume any target.

**MIR's advantage**: Better Z80 code. Register hints, SMC, DJNZ mean the Z80 backend can emit code that iCode can't — self-modifying functions, fused iterator loops, register-hinted allocation.

**What MIR can learn from iCode**: Separate target definitions cleanly. iCode has a `PORT` abstraction — each target defines its registers, sizes, and constraints independently. MIR could adopt this pattern (Layer 1 of the refactoring plan).

### cc65

**What it is**: C compiler for 6502 (Commodore 64, NES, Atari, Apple II).

| Aspect | cc65 | MinZ MIR |
|--------|------|----------|
| **IR** | Pseudo-ops in .s files | Proper IR struct |
| **Optimization** | Basic peephole (~40 patterns) | 13+ passes + 67 asm patterns |
| **Register model** | A/X/Y explicit | Virtual registers |
| **Stack** | Software stack (ZP pointer) | Hardware stack + spill |
| **Types** | C types only | 24 types incl. fixed-point |

**Key difference**: cc65 barely has an IR — it translates C almost directly to 6502 assembly pseudo-ops, then applies peephole optimization. There's no optimizer pipeline, no SSA, no abstract passes. This makes cc65 simple but the output code quality is mediocre (lots of unnecessary stack operations).

**What MIR can learn**: Nothing architecturally. But cc65's **zero-page allocation** strategy is clever — 6502's zero page (256 bytes of fast-access RAM) is a scarce resource, and cc65 has heuristics for which variables get zero-page slots. MIR's memory allocator could adopt similar "fast region" heuristics for Z80's high-RAM.

### z88dk

**What it is**: Z80 development kit with two C compilers: sccz80 (custom) and zsdcc (SDCC fork).

| Aspect | z88dk sccz80 | z88dk zsdcc | MinZ MIR |
|--------|-------------|-------------|----------|
| **IR** | Direct codegen | iCode (SDCC) | MIR |
| **Optimization** | Minimal | SDCC passes | 13+ passes |
| **Libraries** | Huge (200+ targets) | Huge | stdlib (10 modules) |
| **Z80-specific** | Manual optimizations | Generic | SMC, DJNZ, hints |

**Key difference**: z88dk's strength is **library coverage** (200+ target platforms — every Z80 machine ever made), not IR quality. sccz80 has almost no optimization; zsdcc inherits SDCC's. Neither has anything like MIR's optimization pipeline.

**What MIR can learn**: z88dk's **target configuration** system. Each platform (ZX Spectrum, CP/M, MSX, Amstrad CPC, ...) has a configuration file defining memory map, I/O ports, and available routines. MIR's target system (`--target spectrum/cpm/agon`) could adopt this pattern for broader platform support.

### QBE

**What it is**: A lightweight compiler backend (alternative to LLVM) by Quentin Carbonneaux.

| Aspect | QBE | MinZ MIR |
|--------|-----|----------|
| **Targets** | x86-64, aarch64, riscv64 | Z80, 6502, i8080, GB |
| **Form** | SSA with basic blocks | Flat list |
| **Opcodes** | ~40 | 118 |
| **Types** | word, long, single, double | 24 types |
| **Optimization** | ~5 passes | 13+ passes |
| **Philosophy** | 70% of LLVM quality in 10% LOC | Z80-optimal quality |
| **LOC** | ~12K (total) | ~34K (IR+optimizer+codegen) |

**Key difference**: QBE targets modern 64-bit CPUs; MinZ MIR targets vintage 8-bit CPUs. Completely different worlds, but same philosophy: **good enough code without LLVM complexity**.

QBE proves that a small IR can produce good code. Its ~40 opcodes handle all of C. MinZ MIR's 118 opcodes reflect domain complexity (SMC, fixed-point, iterators) rather than bloat.

**What MIR can learn**: QBE's SSA construction is elegant and simple. If MIR ever moves to SSA form (for better optimization), QBE's `mem2reg` approach is the model to follow.

### ACK (Amsterdam Compiler Kit)

**What it is**: Multi-language compiler framework from Vrije Universiteit (1980s). Compiled C, Pascal, Modula-2, Occam, and BASIC to Z80, 6502, 8086, 68000, and others.

| Aspect | ACK EM | MinZ MIR |
|--------|--------|----------|
| **Form** | Stack machine | Register machine |
| **Multi-language** | Yes (5+ frontends) | No (MinZ only) |
| **Multi-target** | Yes (10+ backends) | Yes (7 backends) |
| **Philosophy** | Universal stack IL | Z80-optimized register IR |

**Key insight**: ACK solved the multi-language problem with a **stack-based IL** (EM — "abstract stack machine"). Stack-based IRs are language-neutral because every language can push/pop results. The cost is performance — stack IRs generate more memory traffic than register IRs.

MIR chose the opposite: register-based for performance, at the cost of language coupling. This is the right choice for Z80 (7 registers, 64KB RAM, every cycle counts) but means a PL/M or Pascal frontend needs more translation work.

---

## The Improvement Matrix

What changes would improve scores for which languages:

| Change | Effort | PL/M | BASIC | Pascal | Forth | Ada |
|--------|--------|------|-------|--------|-------|-----|
| **Add `IsPublic` to Function/Global** | 5 min | +1 | — | — | — | +0.5 |
| **Add `OpReadChar`/`OpReadLine`** | 1 day | — | +1 | — | — | — |
| **Add `OpCheckRange(src,lo,hi,trap)`** | 1 day | — | — | +1 | — | +1 |
| **Promote print to function calls** | 1 day | — | -1* | — | — | — |
| **Add exception opcodes** | 2 weeks | — | — | — | — | +2 |
| **Stack scheduler (frontend)** | 2 weeks | — | — | — | +2 | — |
| **Return stack model** | 1 day | — | — | — | +0.5 | — |
| **String runtime library** | 1 week | — | +1 | +0.5 | — | — |
| **Separate target definition** | 2 weeks | — | — | — | — | +0.5 |
| **Make OpDJNZ generic** | 3 days | — | — | — | — | — |

*Promoting print to calls would *hurt* BASIC compatibility (BASIC loves built-in print).

### Priority Order (best ROI)

1. **`OpCheckRange`** (1 day) → +1 Pascal, +1 Ada = +2 total
2. **`IsPublic` on Function** (5 min) → +1 PL/M, +0.5 Ada = +1.5 total
3. **`OpReadChar`/`OpReadLine`** (1 day) → +1 BASIC
4. **String runtime** (1 week) → +1 BASIC, +0.5 Pascal = +1.5 total
5. **Exception opcodes** (2 weeks) → +2 Ada (but expensive)

With items 1-4 (< 2 weeks of work):

```
PL/M   9 → 10/10  ████████████
BASIC  6 →  8/10  ████████░░░░
Pascal 6 →  8/10  ████████░░░░
Forth  5 →  5/10  █████░░░░░░░  (needs frontend work, not IR changes)
Ada    4 →  5/10  █████░░░░░░░  (needs exceptions for real improvement)
```

---

## The Philosophical Divide

Why is it **fundamentally** hard to make one IR serve all languages? Because languages disagree on five axes, and no IR can be neutral on all of them:

### 1. Evaluation Model
```
Register (C, PL/M, Ada, MinZ)  ←→  Stack (Forth, PostScript, JVM)
```
MIR chose register. This makes stack languages harder. ACK chose stack — makes register languages slower. No middle ground.

### 2. Memory Safety
```
Unsafe (C, PL/M, Forth)  ←→  Checked (Ada, Pascal)  ←→  Managed (BASIC, Java)
```
MIR has no runtime checks (unsafe). Adding `OpCheckRange` moves it toward checked. Managed requires a garbage collector (way beyond scope).

### 3. Error Propagation
```
None (C, Forth)  ←→  Flag (MinZ, asm)  ←→  Exception (Ada, Pascal)  ←→  Algebraic (Rust)
```
MIR's carry-flag model is one step above "none." Exceptions need stack unwinding. Algebraic types (Result<T,E>) need sum types and pattern matching.

### 4. Control Flow
```
Structured (Pascal, Ada)  ←→  Unstructured (BASIC GOTO, Forth)  ←→  Continuation (Scheme)
```
MIR handles both structured and unstructured (labels + jumps). Continuations are out of scope.

### 5. Data Model
```
Static (C, PL/M, Forth)  ←→  Dynamic strings (BASIC, Pascal)  ←→  Dynamic everything (Python)
```
MIR is firmly static. Dynamic strings need a heap allocator. Dynamic everything needs a runtime.

### The Sweet Spot

MIR's sweet spot is **statically-typed, register-model, unsafe-or-lightly-checked languages targeting 8-bit CPUs**:

```
Perfect:   PL/M, C subset, MinZ
Good:      Pascal (add range checks), BASIC (add strings/input)
Possible:  Forth (with stack scheduler), Ada subset (Ravenscar)
Wrong fit: Python, Ruby, Scheme, Java
```

---

## Conclusion

The score differences aren't random — they reflect how well each language's fundamental assumptions match MIR's design choices. PL/M scores 9/10 because it was designed for the same hardware with the same philosophy. Ada scores 4/10 because it demands runtime infrastructure (exceptions, tasking) that an 8-bit IR deliberately omits.

The cheapest improvements (`OpCheckRange`, `IsPublic`, `OpReadChar`, string library — under 2 weeks) would raise 4 of 5 languages by 1-2 points each. Exception handling (needed for Ada 6+/10) is the most expensive single improvement.

MIR doesn't need to become LLVM IR. Its Z80-specific features (SMC, DJNZ, register hints) are what make MinZ code fast. The goal is surgical additions that unlock new language frontends without compromising the Z80 optimization pipeline.
