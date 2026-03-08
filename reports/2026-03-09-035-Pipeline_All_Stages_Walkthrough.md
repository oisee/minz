# Report #035 — Full Pipeline Walk-Through: PL/M → Nanz → HIR → MIR2 → Z80

**Date:** 2026-03-09
**Subject:** All intermediate stages of the HIR-based compilation pipeline, with disassembled binary output
**Source file:** `report_demo.plm` — two functions: `abs_diff` and `fib`

> **Scope note:** All compilation paths in this report go through the MinZ `mz` compiler.
> A comparison against a *native* PL/M-80 compiler (e.g. `plm80c` by Mark Ogden, or the
> original Intel PL/M-80 running under CP/M emulation) is not yet included — no native
> PL/M compiler is currently installed. That comparison (native codegen quality vs our
> MIR2 backend) is left as future work.

---

## Source: PL/M-80

The starting point is a PL/M-80 program. Two idiomatic functions chosen for their different structural properties:
- `abs_diff` — pure arithmetic, two returns, no loops
- `fib` — iterative loop with three mutable variables and a body-local temp

```plm
/* report_demo.plm */

abs_diff: procedure (a, b) byte;
  declare (a, b) byte;
  if a > b then
    return a - b;
  else
    return b - a;
end abs_diff;

fib: procedure (n) byte;
  declare n byte;
  declare (a, b) byte;
  a = 0;
  b = 1;
  do while n > 0;
    declare t byte;
    t = b;
    b = a + b;
    a = t;
    n = n - 1;
  end;
  return a;
end fib;
```

---

## Stage 1 — `--emit=nanz`: PL/M → Nanz surface syntax

`mz --emit=nanz report_demo.plm`

Nanz is the MinZ surface language. `--emit=nanz` transpiles PL/M to Nanz source syntax — it
round-trips through the HIR and prints it back as readable Nanz. This is a **transpiler view**,
not a compiler dump: it shows the same semantics in a different surface language.

```nanz
fun ABS_DIFF(A: u8, B: u8) -> u8 {
    if A > B {
        return A - B
    } else {
        return B - A
    }
}

fun FIB(N: u8) -> u8 {
    var A: u8
    var B: u8
    A = 0
    B = 1
    while N > 0 {
        T = B
        B = A + B
        A = T
        N = N - 1
    }
    return A
}
```

**Observations:**
- PL/M `PROCEDURE … END` → Nanz `fun … {}` (braces, no `END`)
- PL/M `DECLARE (a,b) BYTE` → Nanz named `A: u8, B: u8` parameters
- PL/M `DO WHILE … END` → Nanz `while … {}`
- PL/M is case-insensitive; identifiers are uppercased in the HIR (PL/M convention)
- Body-local `DECLARE T BYTE` inside the loop disappears as a declaration — `T` appears as
  an assignment `T = B` with no explicit `var T:` line (the PL/M compiler currently
  infers it as a variable, not yet emitting an explicit HIR VarDeclStmt for body-locals)

**Nanz is nicer than PL/M when:**
- You're reading control flow — braces + indentation are immediately clear
- You want type annotations visible at the call site (params show `A: u8`)
- You're targeting MinZ tooling (LSP, syntax highlighting, formatter)

**PL/M is still appropriate when:**
- Porting historical CP/M software
- The original source is PL/M and you want minimal translation noise

---

## Stage 2 — `--emit=hir`: Typed HIR tree

`mz --emit=hir report_demo.plm`

HIR (High-level IR) is the internal Go struct representation (`pkg/hir/`). Unlike `--emit=nanz`
(which prints surface syntax), `--emit=hir` shows the **typed tree structure**: every
expression node has its resolved type, variable references are explicit, and the difference
between a declaration and an assignment is visible.

```
; HIR module:
fun @ABS_DIFF(A: u8, B: u8) -> u8
  if ((A:u8) > (B:u8)):bool
    return ((A:u8) - (B:u8)):u8
  else
    return ((B:u8) - (A:u8)):u8

fun @FIB(N: u8) -> u8
  var A: u8
  var B: u8
  (A:u8) = 0:u8
  (B:u8) = 1:u8
  while ((N:u8) > 0:u8):bool
    (T:u16) = (B:u8)
    (B:u8) = ((A:u8) + (B:u8)):u8
    (A:u8) = (T:u16)
    (N:u8) = ((N:u8) - 1:u8):u8
  return (A:u8)
```

**Observations vs Nanz:**
- `fun @ABS_DIFF` — `@` prefix marks this as a compiler-internal name, not a source identifier
- Every expression carries `:type` — `((A:u8) > (B:u8)):bool` shows the comparison produces `bool`
- `(A:u8) = 0:u8` — a VarRefExpr on the left: this is an assignment to an existing var, not
  a new declaration. In Nanz this looked like a bare `A = 0`.
- `(T:u16)` — a type inference bug from the PL/M frontend: `T` is a body-local `DECLARE T BYTE`
  but arrives in HIR as `u16`. This is a pre-existing issue in `plm.Compile()`.
- Control flow is still **structured** (if/while, not basic blocks) — HIR sits above MIR2

**HIR is the diagnostic layer between frontend and MIR2 lowering.** If a type looks wrong
here, the bug is in `plm.Compile()` or `nanz.Parse()`, not in the lowerer.

---

## Stage 3 — `--emit=mir2-raw`: MIR2 before optimization

`mz --emit=mir2-raw report_demo.plm`

HIR is lowered to MIR2 by `hir.LowerModule()`. MIR2 is the SSA-like IR with:
- **Virtual registers** (`%r1`, `%r2`, …) globally numbered across the module
- **Basic blocks** with explicit control-flow edges
- **Block parameters** (instead of phi-nodes): `block @loop_head1(%r11, %r12, %r13)`
- **Register classes** on every virtual: `[acc]`=A, `[general]`=C/D/E/H/L, `[counter]`=B,
  `[pointer]`=HL, `[flag]`=flags register

```
; MIR2 module:
fun @ABS_DIFF(%r1: u8 [acc], %r2: u8 [general]) -> u8 [acc]
  block @entry:
    %r3 = cmp.gt %r1, %r2 : bool [flag]
    br_if %r3, @if_then1(), @if_else3()
  block @if_then1:
    %r4 = sub %r1, %r2 : u8 [general]
    ret %r4
  block @if_else3:
    %r5 = sub %r2, %r1 : u8 [general]
    ret %r5

fun @FIB(%r6: u8 [acc]) -> u8 [acc]
  block @entry:
    %r7 = const 0 : u8 [general]
    %r8 = const 0 : u8 [general]
    %r9 = const 0 : u8 [general]
    %r10 = const 1 : u8 [general]
    jmp @loop_head1(%r9, %r10, %r6)
  block @loop_head1(%r11: u8 [general], %r12: u8 [general], %r13: u8 [general]):
    %r14 = const 0 : u8 [general]
    %r15 = cmp.gt %r13, %r14 : bool [flag]
    br_if %r15, @loop_body2(), @loop_exit3(%r11, %r12, %r13)
  block @loop_body2:
    %r16 = add %r11, %r12 : u8 [general]
    %r17 = const 1 : u8 [general]
    %r18 = sub %r13, %r17 : u8 [general]
    jmp @loop_head1(%r12, %r16, %r18)
  block @loop_exit3(%r19: u8 [general], %r20: u8 [general], %r21: u8 [general]):
    ret %r19
```

**Observations:**
- `ABS_DIFF` lowers cleanly: 3 blocks, no loops, 5 instructions total
- `FIB` produces 4 dead `const 0` values (`%r7`, `%r8`, `%r14`) — these are initialization
  artifacts from the HIR lowering of `var A: u8` (zero-inits) and the `while (n > 0)`
  comparison materializing `0` each iteration
- Loop variables `A`, `B`, `N` become block params `%r11`, `%r12`, `%r13` on `@loop_head1` —
  SSA threading without phi-node syntax
- All loop variables are `[general]` class here: the register allocator will assign physical
  registers later

---

## Stage 4 — `--emit=mir2`: MIR2 after optimization (DSE + ReorderBlocks)

`mz --emit=mir2 report_demo.plm`

Two passes run after lowering: `ReorderBlocks` (moves cold/exit blocks to the end) and
`DeadStoreElim` (removes pure instructions with zero uses).

```
; MIR2 module:
fun @ABS_DIFF(%r1: u8 [acc], %r2: u8 [general]) -> u8 [acc]
  block @entry:
    %r3 = cmp.gt %r1, %r2 : bool [flag]
    br_if %r3, @if_then1(), @if_else3()
  block @if_then1:
    %r4 = sub %r1, %r2 : u8 [general]
    ret %r4
  block @if_else3:
    %r5 = sub %r2, %r1 : u8 [general]
    ret %r5

fun @FIB(%r6: u8 [acc]) -> u8 [acc]
  block @entry:
    %r9 = const 0 : u8 [general]
    %r10 = const 1 : u8 [general]
    jmp @loop_head1(%r9, %r10, %r6)
  block @loop_head1(%r11: u8 [general], %r12: u8 [general], %r13: u8 [general]):
    %r14 = const 0 : u8 [general]
    %r15 = cmp.gt %r13, %r14 : bool [flag]
    br_if %r15, @loop_body2(), @loop_exit3(%r11, %r12, %r13)
  block @loop_body2:
    %r16 = add %r11, %r12 : u8 [general]
    %r17 = const 1 : u8 [general]
    %r18 = sub %r13, %r17 : u8 [general]
    jmp @loop_head1(%r12, %r16, %r18)
  block @loop_exit3(%r19: u8 [general], %r20: u8 [general], %r21: u8 [general]):
    ret %r19
```

**What DSE removed:**
- `%r7 = const 0`, `%r8 = const 0` — zero-init artifacts for `var A: u8` and `var B: u8`
  that were never used (the actual initial values `A=0`, `B=1` are passed as block params)

**What DSE did NOT remove:**
- `%r14 = const 0` inside `@loop_head1` — this is the `0` in `while n > 0`. It IS used by
  `cmp.gt`, so it stays. (Future optimization: `CP 0` → `AND A` peephole, already implemented
  in the Z80 codegen layer)

**ABS_DIFF is identical** — no dead stores were present.

---

## Stage 5 — Default output: `.a80` Z80 assembly

`mz report_demo.plm`  (writes `report_demo.a80`)

Register allocation (linear scan with Z80CostTable) assigns physical registers to all virtuals,
then Z80 codegen emits the final assembly.

```asm
; generated by MIR2 Z80 codegen

ABS_DIFF:
    CP C
    JP C, .ABS_DIFF_if_else3
.ABS_DIFF_if_then1:
    SUB C
    LD C, A
    RET
.ABS_DIFF_if_else3:
    NEG
    ADD A, C
    LD C, A
    RET

FIB:
    LD C, 0
    LD D, 1
    LD E, C
    LD C, D
    LD D, A
.FIB_loop_head1:
    LD A, D
    AND A
    JP Z, .FIB_trmp0
.FIB_loop_body2:
    LD A, E
    ADD A, C
    LD E, A
    DEC D
    LD A, C
    LD C, E
    LD E, A
    JP .FIB_loop_head1
.FIB_loop_exit3:
    LD A, C
    RET
.FIB_trmp0:
    LD A, E
    LD E, D
    LD D, C
    LD C, A
    JP .FIB_loop_exit3
```

**ABS_DIFF register allocation:**
- `%r1` (param A, `[acc]`) → A  (calling convention: first param in A)
- `%r2` (param B, `[general]`) → C

`CP C` compares A (a) against C (b). For the `a > b` case the CP sets CF=0 if A≥C, so
`JP C` jumps to the else branch (when carry set = A < C). The `SUB C` path computes `A-B`.
The else path uses `NEG; ADD A,C` which computes `C-A` without clobbering C first (the NEG
trick: `-(A) + C = B - A` since A holds `a` after CP).

**FIB register allocation:**
- `%r6` (param N, `[acc]`) → A  → then moved to D (loop variable N)
- `%r9` (const 0, init of A) → materialised directly as `LD C,0`
- `%r10` (const 1, init of B) → materialised as `LD D,1`
- Loop block params: `%r11` (A) → C, `%r12` (B) → D, `%r13` (N) → E — but see below

Block param assignments at loop entry (`jmp @loop_head1(%r9, %r10, %r6)`):
```asm
LD E, C    ; pass A(=0, in C) as N slot... wait
LD C, D    ;
LD D, A    ;
```
This is the parallel copy for `(C,D,A) → (C,D,E)` → (A→C, B→D, N→E). The allocator
chose C/D/E for the three loop variables.

**Trampoline:** The `@loop_exit3` block receives the final values via a trampoline
`.FIB_trmp0` because the false edge of `br_if` needs to pass copies of `(%r11,%r12,%r13)`.
The trampoline shuffles registers and jumps to the real exit block.

**Known codegen issues visible here:**
1. `LD C, A` before `RET` in ABS_DIFF — result is in A after `SUB C`, but codegen
   emits an extra `LD C,A` (return value copy to a general reg before ret). This is a dead
   store: result is in A, RET expects A, the LD C is wasted (+4T).
2. `JP .FIB_loop_head1` could be `JR .FIB_loop_head1` (relative, saves 1 byte if within ±127)
3. No DJNZ — the loop decrements D with `DEC D` and checks separately. A counter hint on N
   (ClassCounter → B) would enable DJNZ and save ~3T/iteration.

---

## Stage 6 — Binary + `mzd` disassembly

`mz -o report_demo.com -t cpm report_demo.plm`
`mzd report_demo.com --entry 0x0100 --entry 0x010c -o 0x0100 -l -a --cycles`

The assembler produces a 43-byte CP/M `.COM` binary. `mzd` reconstructs the instruction
stream from the binary bytes, annotating T-states:

```
; ---- sub_0100 ($0100) ---- [ABS_DIFF]
0100: B9          CP C                 ; 4T
0101: DA 07 01    JP C,loc_0107        ; 10T
0104: 91          SUB C                ; 4T
0105: 4F          LD C,A               ; 4T   ← dead store before RET
0106: C9          RET                  ; 10T
loc_0107:
0107: ED 44       NEG                  ; 8T
0109: 81          ADD A, C             ; 4T
010A: 4F          LD C,A               ; 4T   ← dead store before RET
010B: C9          RET                  ; 10T

; ---- sub_010C ($010C) ---- [FIB]
010C: 0E 00       LD C,$00             ; 7T
010E: 16 01       LD D,$01             ; 7T
0110: 59          LD E,C               ; 4T   ┐
0111: 4A          LD C,D               ; 4T   ├ parallel copy: N→E, A→C, B→D
0112: 57          LD D,A               ; 4T   ┘
loc_0113:                              ; ← loop_head
0113: 7A          LD A,D               ; 4T
0114: A7          AND A                ; 4T   (CP 0 peephole → AND A)
0115: CA 24 01    JP Z,loc_0124        ; 10T  (to trampoline)
0118: 7B          LD A,E               ; 4T   ┐
0119: 81          ADD A, C             ; 4T   │ B = A + B  (new B into E)
011A: 5F          LD E,A               ; 4T   ┘
011B: 15          DEC D                ; 4T   N = N - 1
011C: 79          LD A,C               ; 4T   ┐
011D: 4B          LD C,E               ; 4T   ├ swap A ↔ B (was A in C, new B in E)
011E: 5F          LD E,A               ; 4T   ┘
011F: C3 13 01    JP loc_0113          ; 10T
loc_0122:                              ; ← loop_exit
0122: 79          LD A,C               ; 4T   return A
0123: C9          RET                  ; 10T
loc_0124:                              ; ← trampoline (pass params to loop_exit)
0124: 7B          LD A,E               ; 4T   ┐
0125: 5A          LD E,D               ; 4T   │ shuffle C,D,E → new C,D,E for exit params
0126: 51          LD D,C               ; 4T   │
0127: 4F          LD C,A               ; 4T   ┘
0128: C3 22 01    JP loc_0122          ; 10T
```

---

## Summary: all stages at a glance

```
PL/M source
    │
    │  plm.Compile()
    ▼
HIR (pkg/hir/)              ← structured CF, named vars, types on every node
    │  --emit=hir            dump as typed tree
    │  --emit=nanz           round-trip as Nanz surface syntax
    │
    │  hir.LowerModule()
    ▼
MIR2 raw                    ← basic blocks, virtual regs, block params (SSA)
    │  --emit=mir2-raw       dump before passes
    │
    │  ReorderBlocks + DSE
    ▼
MIR2 opt                    ← dead stores removed (3 dead consts in FIB)
    │  --emit=mir2           dump after passes
    │
    │  ComputeLiveness + Allocate
    │  Z80Codegen
    ▼
.a80 assembly               ← physical registers, trampolines, peepholes applied
    │  (default output)
    │
    │  MZA assemble
    ▼
.com binary  (43 bytes)
    │
    │  mzd disassemble
    ▼
annotated disassembly       ← T-states, XREF labels, reconstructed CFG
```

---

## Three-path comparison: PL/M direct vs PL/M→Nanz→HIR vs native Nanz

The pipeline supports three entry paths to the same backend. Here we compare all three for
the same two functions.

### The three paths

```
Path A:  report_demo.plm  ──plm.Compile()──▶  HIR  ──▶  MIR2  ──▶  .a80
Path B:  report_demo.plm  ──emit=nanz──▶  .nanz  ──nanz.Parse()──▶  HIR  ──▶  MIR2  ──▶  .a80
Path C:  report_demo.nanz  ──nanz.Parse()──────────────────────────▶  HIR  ──▶  MIR2  ──▶  .a80
                           (written by hand or edited from path B output)
```

Path A and B differ only in frontend: one goes through `plm.Compile()`, the other round-trips
through Nanz text and `nanz.Parse()`. Path C is the "native Nanz" path — source written
directly in Nanz from the start.

### Assembly output: paths A and B are byte-for-byte identical

```
$ diff demo_via_plm.a80 demo_via_nanz.a80
(no output — identical)
```

The Nanz round-trip is **lossless at the codegen level**. The register allocator, parallel
copy resolver, and Z80 codegen produce exactly the same instructions regardless of which
frontend was used. The MIR2 representation is also identical (diff shows only the module
name string).

### HIR diff: two meaningful differences

```diff
< ; HIR module:
> ; HIR module: /tmp/report_demo_roundtrip.nanz

< (T:u16) = (B:u8)
> (T:u8) = (B:u8)

< (A:u8) = (T:u16)
> (A:u8) = (T:u8)
```

**1. Module name:** `plm.Compile()` doesn't propagate a module name to HIR; `nanz.Parse()`
uses the filename. Cosmetic only.

**2. `T:u16` vs `T:u8`** — this is a real bug in the PL/M frontend, and the Nanz round-trip
actually *fixes it*:

- In the PL/M source: `DECLARE T BYTE;` — T is declared as BYTE (u8)
- `plm.Compile()` processes body-local declarations and infers T's type as `u16` (bug in
  the PL/M type inference for body-local vars inside DO/WHILE)
- When `--emit=nanz` prints the HIR back to Nanz, it writes `T = B` — a plain assignment
  with no explicit type (T is untyped in the Nanz surface syntax here)
- When `nanz.Parse()` re-reads this, it infers T's type from the RHS: `B: u8` → `T: u8`

The type fix doesn't affect codegen here (the HIR→MIR2 lowerer coerces mismatched types
via Cast nodes before they reach the allocator), but it means the HIR is **semantically
more correct** after the round-trip than before it. The proper fix is in `plm.Compile()`.

### What this tells us about the pipeline architecture

| Property | Path A (PLM direct) | Path B (PLM→Nanz→HIR) |
|----------|--------------------|-----------------------|
| Assembly output | ✅ correct | ✅ correct (identical to A) |
| HIR types | ⚠️ T:u16 bug | ✅ T:u8 (round-trip fixes it) |
| Module name | empty | filename |
| MIR2 structure | identical | identical |
| Codegen output | identical | identical |

**The Nanz surface language is a faithful intermediate representation.** PL/M→Nanz is a
semantics-preserving transpilation, and Nanz→HIR reinterprets it with correct type inference.
For debugging type issues in PL/M input, `--emit=nanz | mz --emit=hir -` can serve as a
type-correction oracle.

### Nanz vs PL/M: readability comparison

Same semantics, side by side:

```plm
fib: procedure (n) byte;          fun FIB(N: u8) -> u8 {
  declare n byte;
  declare (a, b) byte;               var A: u8
  a = 0;                             var B: u8
  b = 1;                             A = 0
  do while n > 0;                    B = 1
    declare t byte;                  while N > 0 {
    t = b;                             T = B
    b = a + b;                         B = A + B
    a = t;                             A = T
    n = n - 1;                         N = N - 1
  end;                               }
  return a;                          return A
end fib;                           }
```

Nanz wins on:
- **No redundant `DECLARE` block** — types inline with params
- **Braces** instead of `DO … END` / `PROCEDURE … END PROCNAME`
- **`->` return type** visible at the function signature
- **Case-sensitive** — you can write `fib` not `FIB`

PL/M retains value for:
- Historical CP/M porting — source stays authentic
- Developers already fluent in PL/M-80
- `AT` attributes and `BASED` pointers (native PL/M features, mapped to HIR `@ptr()` nodes)

---

## Known quality gaps (future work)

| Issue | Location | Cost | Fix |
|-------|----------|------|-----|
| Dead `LD C,A` before RET | ABS_DIFF (×2) | +8T, +2B | Copy coalescing (Phase 4) |
| `JP` instead of `JR` | FIB loop back-edge | +1B/iter | JP→JR peephole (short branch) |
| No DJNZ | FIB | ~3T/iter | ClassCounter hint on loop counter N |
| Trampoline for loop_exit | FIB | +5 instr | Copy coalescing collapses trampoline |
| `T:u16` type inferred wrong | HIR (PL/M frontend) | — | Fix body-local type in `plm.Compile()` |
