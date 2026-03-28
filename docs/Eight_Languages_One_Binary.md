---
title: "Eight Languages, One Binary"
subtitle: "Whole-Program Compilation: 8 Frontends × 5 Backends on a 1976 CPU"
author: "MinZ Project"
date: "2026-03-28"
---

# Eight Languages, One Binary

## The Idea

What if you could write in any language and get the same optimal binary?

MinZ compiles **eight programming languages** to the same intermediate
representation (MIR2), then optimizes them with the same Z3 SMT solver,
and emits the same provably optimal Z80 machine code. No linker. No runtime.
No overhead.

```
 Nanz ─────┐
 C17 ──────┤
 Frill ────┤
 Pascal ───┤
 ObjC ─────┼──→ HIR ──→ MIR2 ──→ VIR (Z3) ──→ Z80 / CUDA / OpenCL / Vulkan / Metal
 PL/M ─────┤
 Lizp ─────┤
 ABAP ─────┘
```

This is **whole-program compilation**: every function from every language is
visible to the optimizer simultaneously. The Z3 solver picks registers across
all functions at once. A Nanz function can call an ObjC method can call a C
utility — and PFCCO (Per-Function Calling Convention Optimization) ensures zero
overhead at every boundary.

No headers. No `.o` files. No link step. One source tree → one binary.

## The Same Function in Eight Languages

Here is `double(x) = x + x` in every frontend MinZ supports:

**Nanz** (Swift-like, primary language):
```nanz
fun double(x: u8) -> u8 { return x + x }
```

**C17** (freestanding, C23 extensions):
```c
unsigned char double_c(unsigned char x) { return x + x; }
```

**Frill** (ML-style functional):
```frill
let double (x : u8) : u8 = x + x
```

**Pascal** (Turbo Pascal 3.0):
```pascal
function Double(X: Byte): Byte;
begin Double := X + X; end;
```

**Objective-C** (message passing, zero-cost):
```objc
@implementation Math
-(int)double:(int)x { return x + x; }
@end
```

**PL/M** (Intel 8080 heritage):
```plm
DOUBLE: PROCEDURE (X) BYTE;
    DECLARE X BYTE;
    RETURN X + X;
END DOUBLE;
```

**Lizp** (Lisp S-expressions):
```lizp
(defun double ((x u8)) -> u8
  (return (+ x x)))
```

**ABAP** (SAP enterprise):
```abap
FORM double USING x TYPE i CHANGING result TYPE i.
  result = x + x.
ENDFORM.
```

**All eight produce identical Z80 assembly:**

```z80
double:           ; params: A = x, return: A
    ADD A, A      ; x + x (2 bytes, 4 T-states)
    RET           ; return
```

Two instructions. Two bytes. The abstraction is free.

## Whole-Program Compilation

Traditional compilation: each file → object → linker → binary.
Every function boundary has a fixed ABI (cdecl, stdcall). The caller
must follow the convention even when it's wasteful.

MinZ: all files → one IR → one optimizer → one binary.
The Z3 solver sees **every call site** and picks the optimal register
for each parameter. This is PFCCO: Per-Function Calling Convention
Optimization.

```
Traditional (SDCC):          MinZ (PFCCO):
  swap(a, b):                  swap(a, b):
    PUSH HL     ; save           RET     ; 1 instruction!
    LD A, (SP+4); load a         ; Z3 places params so
    LD L, A     ; temp           ; swap(DE) = return(DE)
    LD A, (SP+2); load b         ; The calling convention
    LD (SP+4), A; store          ; IS the optimization.
    LD A, L     ;
    LD (SP+2), A;
    POP HL      ;
    RET         ; 20 instructions
```

**swap() = RET**. Zero instructions. The Z3 solver realized that if
the caller passes `(a, b)` in `(D, E)` and the return convention is
`(D, E)`, then swap is a no-op. SDCC needs 20 instructions because
it uses a fixed stack-based ABI.

## Visual Gallery

All images rendered by MinZ-compiled programs on Z80 hardware or MZV emulator.

### Graphics: Nanz + Frill

| L-System Forest | Windblown Trees | House Scene |
|:---:|:---:|:---:|
| ![Forest](reports/nanz_lsystem_forest.png) | ![Wind](reports/nanz_lsystem_windblown.png) | ![House](reports/nanz_impl_showcase.png) |
| *Recursive fractal trees* | *Asymmetric branching* | *impl dispatch: zero-cost OOP* |

### Shaders: ObjC

| Plasma | Rainbow Diamond | XOR Texture |
|:---:|:---:|:---:|
| ![Plasma](examples/objc/output/plasma.png) | ![Diamond](examples/objc/output/diamond.png) | ![XOR](examples/objc/output/xor.png) |
| *5-distance plasma* | *Diagonal gradient* | *Demoscene classic* |

### 3D Rendering: Nanz

| Shaded Sphere |
|:---:|
| ![Sphere](examples/mzv_sphere_shaded.png) |
| *Raymarched sphere on MZV — pure Nanz, no stdlib* |

### Enterprise: ABAP on ZX Spectrum

| SAP Report | ALV Grid | SQL on Z80 |
|:---:|:---:|:---:|
| ![SAP](media/abap_zx_spectrum.png) | ![ALV](media/mara_alv_zx_spectrum.png) | ![SQL](media/zsql_mara_zx_spectrum.png) |
| *Material Master report* | *ALV with INNER JOIN* | *SQLite on ZX Spectrum!* |

### TUI: Interactive Forms

| Selection Screen |
|:---:|
| ![TUI](media/tui_zx_spectrum.png) |
| *ABAP selection screen on ZX Spectrum — PARAMETERS, input fields* |

---

## Language Showcase

### Nanz: The Primary Language

Swift-like syntax, zero-cost abstractions, `@error` propagation.

```nanz
fun gcd(a: u8, b: u8) -> u8 {
    while b != 0 {
        let t = b
        b = a % b
        a = t
    }
    return a
}

assert gcd(48, 18) == 6
assert gcd(100, 75) == 25
```

**44 example files, 341/341 VIR-compiled functions.**

### C17: The Benchmark Language

First Z80 compiler at C17 conformance level (freestanding) with C23 extensions.
Used for head-to-head comparison with SDCC.

```c
// MinZ: abs_diff compiles to 4 bytes (SUB/RET NC/NEG/RET)
// SDCC: same function = 10 instructions, stack manipulation
unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b;
    return b - a;
}

// assert abs_diff(10, 3) == 7
// assert abs_diff(3, 10) == 7
```

```z80
; MinZ Z3-optimal:          ; SDCC output:
abs_diff:                    _abs_diff:
    SUB B      ; a - b          LD HL, 2
    RET NC     ; a >= b?        ADD HL, SP
    NEG        ; -(a-b)         LD A, (HL)
    RET        ; 4 bytes        INC HL
                                LD C, (HL)
                                SUB C
                                JR NC, .skip
                                LD A, C
                                ...  ; 10 instructions
```

**51 test files, 524/524 asserts.**

### Frill: ML on Z80

Algebraic data types, pattern matching, pipe operators, property testing.
All zero-cost — ADTs compile to integer tags, match compiles to CP/JR chains.

```frill
type Light = Red | Yellow | Green

let next_light (s : u8) : u8 =
  match s with
  | Red    -> 2
  | Yellow -> 0
  | Green  -> 1
  end

(* Property: test ALL 256 values at compile time *)
prop |x| double (double x) == x * 4

(* Pipe operator: data flows left to right *)
let transform (x : u8) = x |> double |> inc
```

```z80
next_light:            ; 12 bytes, Z3-optimal
    OR A               ; s == Red(0)?
    JR NZ, .else
    LD A, 2            ; Red → Green
    RET
.else:
    CP 2               ; s == Green(2)?
    LD A, 0            ; Yellow → Red
    RET Z
    LD A, 1            ; Green → Yellow
    RET
```

**15 example files, 436 asserts, 3351 compile-time checks.**

### Pascal: Turbo Pascal on Z80

Full Turbo Pascal 3.0 subset: procedures, functions, records, arrays,
case statements, nested scopes.

```pascal
function Max(A, B: Byte): Byte;
begin
  if A > B then Max := A
  else Max := B;
end;

assert Max(10, 2) = 10;
assert Max(3, 7) = 7;
```

```z80
MAX:                   ; Z3-PFCCO: A=first, B=second, ret=A
    CP B               ; compare A with B
    JR Z, .ret_b       ; equal → return B
    JR C, .ret_b       ; A < B → return B
    RET                 ; A > B → return A (already in A!)
.ret_b:
    LD A, B
    RET
```

**9 example files, 54 asserts.**

### Objective-C: Messages on Z80

ObjC syntax with zero-cost method dispatch. `[obj method]` compiles to
direct `CALL` — no vtables, no message lookup, no objc_msgSend runtime.

```objc
@interface Entity {
    int hp; int maxHp; int x; int y;
}
-(int)hp;
-(int)isAlive;
-(int)hpPercent;
-(int)distanceTo:(int)tx y:(int)ty;
@end

@implementation Entity
-(int)hp { return self->hp; }
-(int)isAlive {
    if (self->hp > 0) return 1;
    return 0;
}
-(int)hpPercent {
    return self->hp * 100 / self->maxHp;
}
@end
```

Method chaining, builder patterns, inheritance — all compile to
direct struct access + CALL. The ObjC syntax is pure sugar.

```objc
// Vec3 fluent pipeline:
// [[self scale:2] sum]  →  CALL Vec3_scale, CALL Vec3_sum
-(int)scaleAndSum:(int)factor {
    return [[self scale:factor] sum];
}

// Builder pattern:
// [[[self width:8] height:10] area]  →  3 CALLs, zero overhead
```

**11 example files, 98 asserts.**

### PL/M: Intel Heritage (1973)

The original 8080/8085 programming language. Digital Research used it for CP/M.
MinZ brings it back — same syntax, modern optimizer.

```plm
DOUBLE: PROCEDURE (X) BYTE;
    DECLARE X BYTE;
    RETURN X + X;
END DOUBLE;

ABS$DIFF: PROCEDURE (A, B) BYTE;
    DECLARE (A, B) BYTE;
    IF A > B THEN RETURN A - B;
    RETURN B - A;
END ABS$DIFF;

MAX$BYTE: PROCEDURE (A, B) BYTE;
    DECLARE (A, B) BYTE;
    IF A > B THEN RETURN A;
    RETURN B;
END MAX$BYTE;

ASSERT DOUBLE(5) = 10;
ASSERT ABS$DIFF(10, 3) = 7;
ASSERT MAX$BYTE(99, 1) = 99;
```

The `$` in identifiers is original PL/M syntax. `ABS$DIFF` reads as "abs-diff".

### Lizp: Lisp That Compiles to Z80

Lizp is Scheme for Z80 — a minimal Lisp with S-expression syntax that
compiles to the same optimal Z80 code as every other frontend. Not an
interpreter. Not a bytecode VM. A real Lisp-to-machine-code compiler.

The name: **Li**sp + Min**z** = **Lizp**. Pronounced "lisp" with a Z80 accent.

Why? Because if you can compile Lisp to 2-byte Z80 functions, you can
compile anything. Lizp proves the MIR2 pipeline is truly language-agnostic.

```lizp
;; S-expressions → Z80 machine code
(defun double ((x u8)) -> u8
  (return (+ x x)))
;; → ADD A, A / RET  (2 bytes)

(defun max_byte ((a u8) (b u8)) -> u8
  (if (> a b) (return a) (return b)))

(defun add ((a u8) (b u8)) -> u8
  (return (+ a b)))

;; Compile-time verification — in S-expression syntax
(assert double 5 == 10)
(assert max_byte 10 20 == 20)
(assert add 3 4 == 7)
```

The parentheses are real. The Z80 code is optimal. John McCarthy meets
Zilog, 1958 meets 1976.

### ABAP: Enterprise on Z80

SAP's enterprise language — SELECT statements, ALV grid reports, classes,
and forms. Running on a 3.5MHz CPU with 64KB RAM. Complete with SQLite
bridge via I/O ports.

```abap
SELECT matnr mtart meins maktx
  FROM mara
  INNER JOIN makt
  ON mara~matnr = makt~matnr
  INTO TABLE @DATA(lt_mara).

cl_salv_table=>factory( lt_mara ).
```

This renders an ALV grid on ZX Spectrum with 5 entries, column headers,
and formatted output — from actual ABAP SELECT syntax.

```abap
FORM gcd USING a TYPE i b TYPE i CHANGING result TYPE i.
  WHILE b > 0.
    DATA: temp TYPE i.
    temp = b.
    b = a MOD b.
    a = temp.
  ENDWHILE.
  result = a.
ENDFORM.

* assert gcd(48, 18) == 6 via mir2
```

## Cross-Language Calling

Because everything compiles to the same MIR2, functions from different
languages can call each other with zero overhead:

```nanz
// Nanz calls C calls Frill calls Pascal
import "utils.c"      // C utility functions
import "transform.frl" // Frill pipeline
import "math.pas"      // Pascal math library

fun pipeline(x: u8) -> u8 {
    let y = c_normalize(x)     // C function
    let z = y |> frl_transform // Frill pipe
    return pas_clamp(z, 0, 255) // Pascal function
}
```

All three functions are compiled together. Z3-PFCCO picks registers
across all of them — no ABI boundary, no calling convention mismatch,
no adapter stubs. The inter-language call is a single `CALL` instruction.

## Compilation Pipeline

```
Source files (.nanz, .c, .frl, .pas, .m, .plm, .lizp, .abap)
  │
  ├─ 8 Frontends (parsers + lowerers)
  │
  ▼
HIR (High-level IR: typed, structured)
  │
  ▼
MIR2 (SSA form, ~30 opcodes, target-independent)
  │
  ├─ MIR2 VM (compile-time assert execution)
  │
  ▼
VIR (Z3 SMT solver: joint isel + regalloc)
  │
  ├─ Z3-PFCCO: module-level calling convention optimization
  ├─ Per-instruction variables: solver plans moves as part of solution
  ├─ PBQP fallback: for functions that exceed Z3 timeout
  │
  ▼
5 Backend targets:
  ├─ Z80 Assembly → Binary (ZX Spectrum, CP/M, Agon Light 2)
  ├─ CUDA kernel (NVIDIA GPU)
  ├─ OpenCL kernel (AMD/Intel GPU)
  ├─ Vulkan GLSL compute shader
  └─ Metal shader (Apple GPU)
```

## Assert Corpus: Trust But Verify

Every function is tested at compile time. Asserts run on both the MIR2 VM
and the Z80 emulator (MZE, 1335/1335 FUSE tests). If any assert fails,
the binary is not produced.

| Language | Files | Asserts | Z80 Verified |
|----------|-------|---------|-------------|
| Frill | 15 | 436 | ✅ 14/15 |
| C89 | 38 | 353 | ✅ |
| C99+ | 20 | 305 | ✅ |
| ObjC | 11 | 98 | ✅ 11/11 |
| Pascal | 9 | 54 | ✅ 9/9 |
| Nanz | 44 | 371 | ✅ |
| ABAP | 28 | 31 | ✅ (MIR2) |
| PL/M | 5 | 9 | ✅ |
| Lizp | 5 | 9 | ✅ |
| **Total** | **175** | **~1666** | |

## Bool Return Convention (GPU-Proven)

How should a predicate like `is_digit` return its result? GPU brute-force
search across 456,976 instruction sequences proved the optimal design:

**Z flag is write-only on Z80.** No instruction reads Z into A or CY.
The only way to use Z is an immediate conditional branch (JR Z/NZ).
This was proven exhaustive — not a heuristic, a mathematical fact.

**CY flag can be materialized branchless:**
```z80
SBC A, A    ; A = 0xFF if CY was set, 0x00 if clear. 1 instruction, 4T.
```

Z3 chooses the optimal return mode per-function:

| Comparison | Natural flag | Caller branch | Materialize to A |
|-----------|-------------|---------------|-----------------|
| `a == b` | Z | JR Z (0T) | branch (14T) |
| `a < b` | CY | JR C (0T) | SBC A,A (4T) |
| `a >= b` | NC | JR NC (0T) | SBC A,A;CPL (8T) |

**Branchless conditional move (CMOV) on Z80:**
```z80
; CY ? B : C  — 6 instructions, 24T, no branches
SBC A, A      ; A = mask (0xFF or 0x00)
LD  D, A      ; save mask
LD  A, B      ; load X
XOR C         ; A = X ^ Y
AND D         ; A = (X^Y) & mask
XOR C         ; A = Y ^ ((X^Y) & mask) = CY ? X : Y
```

The classic bitwise select trick, verified exhaustive on all 131,072
input combinations (2 × 256 × 256).

## Idiomatic Bool Asserts

Nanz supports natural bool assertion syntax:

```nanz
// Classic
assert is_digit(48) == 1

// Bool shorthand — implies == true
assert is_digit(48)

// Negated — implies == false
assert not is_digit(65)

// Bool literals
assert is_digit(48) == true
assert is_digit(65) == false
```

All forms compile to the same assertion check.

## Why This Matters

**For retro developers:** Write in your favorite language. Get optimal Z80 code.
No compromises.

**For language designers:** Add a frontend (parser + HIR lowerer). Get 5 backends,
Z3 optimization, and a full test harness for free.

**For compiler researchers:** Whole-program compilation with SMT-based
register allocation across 8 source languages. We believe this is the first
compiler that applies Z3 to multi-language whole-program optimization
on a real target.

**For everyone:** Eight languages from 1973 (PL/M) to 2026 (Frill).
Enterprise (ABAP), systems (C), functional (Frill/Lizp), OOP (ObjC),
structured (Pascal), and Nanz — all producing the same binary.
Fifty years of programming languages on a fifty-year-old CPU.

One compiler. Eight languages. Five targets. Zero overhead.

*MinZ: where every abstraction compiles away.*
