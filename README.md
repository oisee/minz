# MinZ Programming Language

### ★ NEW: Frill — ML on Z80 | [Language Guide (book)](docs/Frill_Language_Guide.md) | [PDF](docs/book/Frill_Language_Guide.pdf)

**8th frontend:** An ML-style functional language that compiles to Z80. Algebraic data types, pattern matching, parametric polymorphism, effects, property testing — all zero-cost. 13 examples, 3351 compile-time checks.

```frill
type Option = None | Some of u8

let unwrap (opt : u16) (def : u8) : u8 =
  match opt with
  | Some x -> x
  | None   -> def
```

### ★ NEW: Nanz ADT + Match Expressions

Nanz (the primary language) now has **algebraic data types with payload** and **Rust-style match expressions**:

```nanz
enum Result { Ok(u8), Err(u8) }     // u16: tag<<8 | payload

fun safe_add(a: u8, b: u8) -> u16 {
    if (u16(a) + u16(b) > 255) { return Err(1) }
    return Ok(a + b)
}

fun color_name(c: Color) -> u8 {
    return match c {
        Red   => 1,
        Green => 2,
        Blue  => 3,
    }
}
```

Exhaustive check, payload binding, `_` wildcard. [Showcases: Option](examples/nanz/10_adt_option.nanz) | [Match](examples/nanz/11_match_expression.nanz) | [State machine](examples/nanz/12_state_machine.nanz) | [Result/Error](examples/nanz/13_result_error.nanz)

---

### ★ [VIR Solver: Z3 Joint Isel+Regalloc — 61% smaller than SDCC](research/abi-paper/vir-e2e-report.md)

**Z3 SMT solver for joint instruction selection + register allocation.** ISLE fusion + Grace PIR dead-register elimination. Now the default backend.

| Program | SDCC | VIR | Delta |
|---------|------|-----|-------|
| abs_diff | 12 | 12 | tie |
| gcd | 17 | 15 | −11% |
| minmax | 60 | 11 | −81% |
| fib | 22 | 11 | −50% |
| swap | 20 | 2 | −90% |
| **TOTAL** | **131** | **51** | **−61%** |

FatFS `ld_word`: **5 instructions** (SDCC: 29 bytes). **520/520 corpus coverage (100%)** — zero PBQP fallback. 10/10 leaf functions provably optimal (identical to hand-written Z80). 55 Z80-verified asserts.

→ **[Full E2E Report](research/abi-paper/vir-e2e-report.md)** | **[ADR-0039: Unified VIR Solver](docs/adr/0039-unified-vir-solver.md)**

---

### ★ [Week In Review: The Everything Sprint](reports/2026-03-18-094-Week_In_Review_Sprint_0311_0318.md) | [Report Guide](docs/Report_Guide.md) | [Nanz Language Book v5](docs/Nanz_Language_Book_v5.md)

253 commits, 8 frontends, 3 backends, FAT12/16 filesystem, TUI framework with compile-time metafunctions, Mini Norton Commander, `@target()` platform detection — in one week.

---

### ★ [LIR Backend: 948/948 = 100% All Corpora](reports/2026-03-18-094-LIR-100-Percent-C89-Corpus.md)

**14 commits in one session: from wrong code to 100% across all 4 frontends.** PBQP (global strategist) + WFC (local tactician) register allocation, with Z80-specific innovations no existing compiler has.

| Corpus | Functions (×3 machines) | Pass Rate |
|--------|------------------------|-----------|
| C89 | 720 | **100.0%** |
| Nanz | 162 | **100.0%** |
| Lizp | 57 | **100.0%** |
| Lanz | 9 | **100.0%** |
| **Total** | **948** | **100.0%** |

| Innovation | What | Savings |
|------------|------|---------|
| IXH/IXL call-safe spill | Undocumented IX halves survive CALL | 8T vs 11T PUSH/POP |
| PBQP→WFC hint bridge | Global + local allocators cooperate | = production output |
| Tail call optimization | CALL+RET → JP | -17T per tail call |
| `__mul8`/`__mul16` runtime | Shared multiply routines | code size vs inline |
| Save-before-overwrite | Destructive Z80 ALU handling | correctness |

→ **[Architecture & Innovations](docs/LIR_Backend_Architecture.md)** | **[Quick Reference](docs/LIR_Backend_Reference.md)** | **[Journey Report](reports/2026-03-18-094-LIR-100-Percent-C89-Corpus.md)**

---

### Latest

- **[VIR Solver: Z3 isel+regalloc](research/abi-paper/vir-e2e-report.md)** — Z3 SMT joint instruction selection + register allocation. ISLE load16_le fusion (FatFS `ld_word` 5x over SDCC), Grace dead-register elimination, inline div8/mod8/mul8 runtime. −61% vs SDCC on benchmarks. Default backend.
- **Nanz ADT + Match** — `enum Option { None, Some(u8) }` with payload (u16 encoding), `match` expression with exhaustive check, 11 new tests. 4 showcase examples.
- **[Frill Language: ML on Z80](docs/Frill_Language_Guide.md)** — 8th frontend. ADTs, pattern matching, polymorphism, effects, property testing. 38 features, 13 examples, 3351 compile-time checks. [Book (PDF)](docs/book/Frill_Language_Guide.pdf)
- **[ABAP Frontend + SQLite + Zork](reports/2026-03-16-088-ABAP_Frontend_SQLite_Zork.md)** — 7th frontend (ABAP via abaplint), SQLite host functions in MIR2 VM, CP/M file I/O fixed (ROM protection root cause), **Zork I (1983) runs in MZE**.
- **[Nanz Z80 Showcase v2](reports/2026-03-15-084-Nanz_Z80_Showcase_Definitive_v2.md)** — 12 verified examples: `abs_diff` 6B (optimal), `swap` 1B (bare RET), `smaller` 0B (EQU), `popcount` 3-inst LUT, `@smc` compiled sprites, value pipes constant-folded, iterator DJNZ fusion. Plus `elimJrToRet` peephole: `JR cc → RET` → `RET cc`.
- **[C89 Frontend vs SDCC](reports/2026-03-15-081-MinZ_C89_vs_SDCC_Codegen_Comparison.md)** — 6th frontend: C89/C99 via `modernc.org/cc/v4`. Identical C source, **MinZ 81B vs SDCC 179B (−55%)**. Pair-return byte-identical to Nanz. See table below. [Progress report](reports/2026-03-15-082-C89_Frontend_Progress.md): 9 corpus files, 51 functions, 68 MIR2 asserts pass.
- **[Six Frontends, Universal Assert](reports/2026-03-15-080-Five_Frontends_Universal_Assert.md)** — Nanz, Lanz, Lizp, PL/M-80, Pascal, C89 — all compile through one HIR → MIR2 → Z80 pipeline. Compile-time assert works in all 6. Pascal → CP/M hello world runs in MZE.
- **[Nanz Language Book v5.3](docs/Nanz_Language_Book_v5.md)** — 21 chapters + 8 appendices. New: five-frontend architecture, universal assert syntax, Pascal/Lizp imports, transpilation via `--emit`.
- **[ZX Spectrum Tetris](examples/zx/tetris.nanz)** — 853 LOC, 7 tetrominoes, SRS wall kicks, hold/next/ghost piece, T-spin scoring. Attribute-based rendering for fast frame updates.
- **[Nanz Language Sprint: 6 features](reports/2026-03-15-073-Nanz_Language_Sprint_Six_Features.md)** — enums, type aliases, module imports, three string types, pipe/trans named pipelines with DJNZ fusion.
- **[Arena allocator + sandbox + sizeof](reports/2026-03-14-069-Arena_Allocator_Sandbox_Sizeof.md)** — struct-based bump allocator with `^Arena` pointer receiver, `arena_split` chaining, `sizeof(Type)` compile-time constant.
- **[PreallocCoalesce delivers](reports/2026-03-13-068-Showcase_ForEachEdge_SignedCmp_PreallocImpact.md)** — `mapInPlace` loop: 5 instructions → **1 DJNZ**. `factorial_fold`: entire mul16 routine eliminated.
- **[MOS 6502 backend alive](reports/2026-03-13-067-6502_Backend_E2E_Harness.md)** — 35/35 tests, dual-VM oracle (MIR2 VM vs sim6502), console I/O for Apple II/C64/BBC Micro.

[Full changelog →](CHANGELOG.md)

### MinZ C89 vs SDCC 4.2.0 — Z80 Codegen Comparison

Identical C source compiled through both toolchains. Binary sizes (code only):

| Function | MinZ C89 | SDCC 4.2.0 | Delta | Notes |
|----------|-------:|-------:|------:|-------|
| `twice(i16)→i16` | 2B | 3B | −1B | SDCC: `EX DE,HL` return tax |
| `add(i16,i16)→i16` | 2B | 3B | −1B | SDCC: `EX DE,HL` return tax |
| `max(i16,i16)→i16` | 12B | 12B | TIE | Both clever compare tricks |
| `abs_diff(u8,u8)→u8` | 9B | 11B | −2B | MinZ: `RET Z/RET C` conditional return |
| `sum_to(i16)→i16` | 21B | 25B | −4B | MinZ: no trampoline |
| `clamp8(u8,u8,u8)→u8` | 10B | 30B | −20B | MinZ: 3-reg ABI + `RET Z/C` |
| `minmax(u16,u16)→(u16,u16)` | 19B | 61B | −42B | MinZ: tuple return + `RET C/Z` |
| `smaller` (uses lo) | 0B | 34B | −34B | MinZ: `EQU minmax` (degenerate!) |
| `larger` (uses hi) | 6B | — | — | |
| **TOTAL** | **81B** | **179B** | **−55%** | [Full report →](reports/2026-03-15-081-MinZ_C89_vs_SDCC_Codegen_Comparison.md) |

<div align="center">

![MinZ Logo](/media/minz-logo-shamrock-mint.png)

### Modern Programming Language for Vintage Hardware

[![Version](https://img.shields.io/badge/version-0.23.0-blue)](https://github.com/oisee/minz/releases)
[![License](https://img.shields.io/badge/license-MIT-purple)]()

**Write modern code. Run it on Z80, eZ80, 6502, and more.**

[Quick Start](#quick-start) | [Features](#features) | [Examples](#code-examples) | [Targets](#platform-targets) | [Toolchain](#toolchain)

</div>

---

## What is MinZ?

MinZ is a compiler toolchain for retro hardware — primarily Z80 and eZ80, with an experimental MOS 6502 backend.

The **primary frontend** is **Nanz** (`.nanz`) — a type-safe language with ADT enums, pattern matching, lambdas, and zero-cost abstractions. Seven additional frontends — **Frill** (ML-style functional), **Lanz** (S-expressions), **Lizp** (Lisp), **PL/M-80**, **Pascal**, **C89**, and **ABAP** — compile through the same HIR → MIR2 → Z80 pipeline. Cross-language imports are first-class.

Self-contained toolchain: compiler, assembler, emulator, disassembler, and remote runner. No external dependencies — pure Go.

```minz
import stdlib.cpm.bdos;

fun main() -> void {
    @print("Hello from MinZ!");
    let fib_a: u16 = 0;
    let fib_b: u16 = 1;
    for i in 0..10 {
        print_u16(fib_a);
        putchar(32);  // space
        let next = fib_a + fib_b;
        fib_a = fib_b;
        fib_b = next;
    }
}
```

This compiles to Z80 assembly, assembles to a `.com` binary, and runs on CP/M:

```
$ mz fibonacci_cpm.minz -b z80 --target cpm -o fib.a80 && mza fib.a80 -o fib.com
$ mze fib.com -t cpm
Fibonacci:
0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55
```

---

## Quick Start

### Build from Source

```bash
git clone https://github.com/oisee/minz.git
cd minz/minzc
make all            # Build all 9 tools
make install-user   # Install to ~/.local/bin/
```

No external dependencies. Pure Go.

### Compile and Run

```bash
# Compile MinZ to Z80 assembly
./mz ../examples/hello_print.minz -o hello.a80

# Assemble to binary
./mza hello.a80 -o hello.tap

# Run in emulator
./mze hello.tap
```

### Multi-Target

```bash
mz program.minz -b z80 --target spectrum -o prog.a80   # ZX Spectrum
mz program.minz -b z80 --target cpm -o prog.a80        # CP/M
mz program.minz -b z80 --target agon -o prog.a80       # Agon Light 2
mz program.minz -b c -o prog.c                         # C99 (partial — simple programs only)
mz program.minz -b crystal -o prog.cr                  # Crystal (stub — not functional)
```

---

## Features

### Working

| Feature | Description |
|---------|-------------|
| **Types** | `u8`, `u16`, `i8`, `i16`, `bool`, `void`, pointers |
| **Functions** | `fun`/`fn` declaration, overloading, multiple returns |
| **Control flow** | `if`/`else`, `while`, `for i in 0..n`, `loop {}` |
| **Structs** | Declaration, field access, UFCS method syntax |
| **Arrays** | Declaration, indexing |
| **Globals** | `global counter: u8 = 0;` |
| **String interpolation** | `"Hello #{name}!"` (Ruby-style) |
| **Inline assembly** | `asm { LD A, 42 }` blocks, `[addr]` bracket indirection |
| **CTIE** | Compile-Time Interface Execution (trait monomorphization) |
| **True SMC** | Self-modifying code optimization |
| **@extern FFI** | `extern fun putchar(c: u8) at 0x10;` with RST optimization |
| **Operator overloading** | `v1 + v2` via `impl` blocks |
| **Error propagation** | `@error(code)` with CY flag ABI |
| **Enums** | `enum State { IDLE, RUNNING }` with values |
| **ADT Enums** | `enum Option { None, Some(u8) }` — payload in u16, auto `__tag`/`__payload` |
| **Match expressions** | `match c { Red => 1, Green => 2, _ => 0 }` — exhaustive check, payload binding |
| **Module system** | `import stdlib.cpm.bdos;` |
| **Lambdas** | Closure syntax, zero-cost transform |
| **PL/M-80 frontend** | Parse + HIR lowering for all 26 Intel 80 Tools corpus files (100%); 1338 functions, 11661 statements |
| **Nanz frontend** | New active source language for the MIR2 backend; arithmetic, control flow, loops, function calls |
| **LUTGen** | `u8<lo..hi>` ranged type annotation → compile-time table generation; popcount loop → 3-instruction LUT at runtime |
| **Flag-return ABI** | Functions returning `bool` from a comparison pass the result via carry flag — no `LD A, 0/1` materialization |
| **Interprocedural CC opt** | Register class chosen per call-site: params coerced to A/B/C/HL/DE based on callee contract |
| **JRS pseudo-instruction** | Codegen emits `JRS` for all branches; MZA picks `JR` (2B) or `JP` (3B) based on offset and condition |

### Partial / In Progress

| Feature | Status |
|---------|--------|
| Pattern matching | **Working** — `match` expression with ADT payloads, exhaustive check. LIR backend: nested CondExpr WIP |
| Iterator chains | 9 ops on Z80 + inline lambda filters + **fusion optimizer** (inlines callbacks in DJNZ loops). **87+ tests, 11/11 E2E hex-verified, all pass.** enumerate/reduce at MIR level (Z80 needs OpPush fix). See [Status](docs/Iterator_Implementation_Status.md) |
| MIR interpreter | Arrays/structs working, not complete |

### Known Limitations

- Register allocator has bugs with overlapping lifetimes in complex loops
- Some loop/arithmetic combinations produce incorrect code
- `loadToHL` can use stale values in multi-expression contexts
- Loop rerolling can be too aggressive across function call boundaries

These are documented and being worked on. Simple programs (hello world, fibonacci, demos) work correctly. Complex programs with nested loops and heavy arithmetic may hit edge cases.

---

## Code Examples

### Nanz: New Active Frontend for MIR2

Nanz is the primary language for the HIR→MIR2→Z80 pipeline. Real compiled output:

```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a > b { return a - b }
    return b - a
}

fun clamp(x: u8, lo: u8, hi: u8) -> u8 {
    if x < lo { return lo }
    if x > hi { return hi }
    return x
}
```

Generated Z80 (actual `mz` output):

```asm
abs_diff:
    CP C
    JR Z, .abs_diff_if_join2
    JR C, .abs_diff_if_join2
.abs_diff_if_then1:
    SUB C
    LD C, A
    RET
.abs_diff_if_join2:
    NEG
    ADD A, C
    LD C, A
    RET

clamp:
    CP D                    ; x vs lo
    JR NC, .clamp_if_join2
.clamp_if_then1:
    LD A, D
    RET
.clamp_if_join2:
    CP C                    ; x vs hi
    JR Z, .clamp_if_join4
    JR C, .clamp_if_join4
.clamp_if_then3:
    LD A, C
    RET
.clamp_if_join4:
    RET
```

### ADT Enums + Match Expressions (NEW)

```nanz
enum Option { None, Some(u8) }  // u16: tag<<8 | payload

fun unwrap_or(opt: u16, def: u8) -> u8 {
    if (__tag(opt) == 1) { return __payload(opt) }
    return def
}

enum State { Idle, Walking, Jumping, Dead }

fun state_speed(s: State) -> u8 {
    return match s {
        Idle    => 0,
        Walking => 2,
        Jumping => 4,
        Dead    => 0,
    }
}
```

Generated Z80 (production backend):

```asm
state_speed:              ; A = state tag
    AND A                 ; test A == 0 (Idle)
    JR NZ, .cret_else
    XOR A                 ; return 0
    RET
.cret_else:
    CP 1                  ; Walking?
    JR NZ, .cond_else
    LD A, 2               ; return 2
    RET
.cond_else:
    CP 2                  ; Jumping?
    LD A, 0               ; default: 0 (Dead)
    RET NZ
    LD A, 4               ; return 4
    RET
```

**11 bytes, 5 branches, zero overhead.** Match compiles to a chain of `CP` + conditional returns — the same code a hand-written assembly programmer would write.

### PBQP Allocator: Hot Registers in Cheap Slots

([`examples/nanz/05_four_pointers.nanz`](examples/nanz/05_four_pointers.nanz) · [`06_pbqp_weighted.nanz`](examples/nanz/06_pbqp_weighted.nanz) · [`07_ix_load_store.nanz`](examples/nanz/07_ix_load_store.nanz))

The PBQP allocator weights each virtual register's cost by its use count.
A register used 10× pays 10× the slot cost, so the solver puts it in the
cheapest location — even when that means displacing a low-use register.

**Four simultaneously-live pointer registers → HL / DE / BC / IX (no spill):**

```nanz
// examples/nanz/05_four_pointers.nanz
fun four_ptrs(p0: ptr, p1: ptr, p2: ptr, p3: ptr) -> u8 {
    var v0: u8 = p0[0]
    var v1: u8 = p1[0]
    var v2: u8 = p2[0]
    var v3: u8 = p3[0]    // p3 → IX under register pressure
    var s01: u8 = v0 + v1
    var s23: u8 = v2 + v3
    return s01 + s23
}
```

```asm
four_ptrs:
    LD C, (HL)      ; p0 → HL  (cost 0)
    LD D, (DE)      ; p1 → DE  (cost 4)
    LD E, (BC)      ; p2 → BC  (cost 6)
    LD H, (IX+0)    ; p3 → IX  (cost 8) ← (IX+0) not $F0xx memory!
    LD A, C
    ADD A, D
    LD C, A
    LD A, E
    ADD A, H
    ...
    RET
```

**High-use vs low-use — PBQP always puts the hot reg in the cheap slot:**

```nanz
// examples/nanz/06_pbqp_weighted.nanz
fun weighted(x: u8) -> u8 {
    var light: u8 = 1          // used 1×  — displaced to C
    var heavy: u8 = x          // used 10× — stays in A (0T per use)
    heavy = heavy + x          // ... repeated 9 more times
    ...
    return heavy + light
}
```

```asm
weighted:
    LD C, 1          ; light → C  (1× use, forced out of A)
    ADD A, A         ; heavy stays in A throughout (10× use, 0T/use)
    LD D, A
    ADD A, D
    ...              ; 8 more iterations — all in A, zero memory traffic
    ADD A, C         ; final: heavy(A) + light(C)
    RET
```

**IX store/load — undocumented HL→IX copy (16T vs 21T PUSH/POP):**

```nanz
// examples/nanz/07_ix_load_store.nanz
fun roundtrip_ix(hl_ptr: ptr, de_ptr: ptr, bc_ptr: ptr, val: u8) -> u8 {
    bc_ptr[0] = val           // bc_ptr overflows to IX under 4-reg pressure
    var a: u8 = hl_ptr[0]
    var b: u8 = de_ptr[0]
    var back: u8 = bc_ptr[0]
    return a + b + back
}
```

```asm
roundtrip_ix:
    LD IXH, H         ; undocumented DD 67 — copy HL→IX (16T, not PUSH/POP=21T)
    LD IXL, L         ; undocumented DD 6D
    LD (IX+0), C      ; store val through IX pointer
    LD C, (DE)
    LD D, (BC)
    LD E, (HL)
    ...
    RET
```

### LUTGen: Compile-Time Lookup Tables

Annotate with `u8<0..255>` — the compiler evaluates the function for all 256 values and emits a page-aligned table:

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

The loop above never runs at runtime. Generated Z80:

```asm
popcount:
    LD HL, popcount_lut
    LD L, C                 ; C = input (index into table)
    LD A, (HL)              ; table lookup — H unchanged = page base
    RET

    ALIGN 256
popcount_lut:
    DB 0, 1, 1, 2, 1, 2, 2, 3, ...   ; 256 bytes, evaluated at compile time
```

### Structs and Methods (UFCS)

```minz
struct Vec2 { x: i16, y: i16 }

impl Vec2 {
    fun add(self, other: Vec2) -> Vec2 {
        return Vec2 { x: self.x + other.x, y: self.y + other.y };
    }
    fun length_sq(self) -> i16 {
        return self.x * self.x + self.y * self.y;
    }
}

fun main() -> void {
    let v1 = Vec2 { x: 3, y: 4 };
    let v2 = Vec2 { x: 1, y: 2 };
    let v3 = v1 + v2;            // Zero-cost: CALL Vec2_add
    let len = v3.length_sq();    // Zero-cost: CALL Vec2_length_sq
}
```

### Compile-Time Execution (CTIE)

```minz
@ctie
fun fibonacci(n: u8) -> u8 {
    if n <= 1 { return n; }
    return fibonacci(n-1) + fibonacci(n-2);
}

let fib10 = fibonacci(10);  // Becomes: LD A, 55 (no runtime cost)
```

### Inline Assembly

```minz
asm fun fast_clear_screen() {
    LD HL, $4000
    LD DE, $4001
    LD BC, 6143
    LD (HL), 0
    LDIR
}
```

### CP/M Program

```minz
import stdlib.cpm.bdos;

fun main() -> void {
    @print("Hello, CP/M!");
    putchar(13);
    putchar(10);
    let ch = getchar();
    putchar(ch);
}
```

### Agon Light 2 Program

```minz
import stdlib.agon.mos;
import stdlib.agon.vdp;

fun main() -> void {
    mos_puts("Hello from Agon Light 2!");
    set_mode(3);
    fill_rect(10, 10, 100, 80, 4);
}
```

### Error Handling

```minz
enum FileError { None, NotFound, Permission }

fun read_file?(path: u8) -> u8 ? FileError {
    if path == 0 {
        @error(FileError.NotFound);
    }
    return path;
}
```

### Self-Modifying Code (True SMC)

```minz
@abi("smc")
fun draw_pixel(x: u8, y: u8) -> void {
    // Parameters patched directly into instruction immediates
    // Single-byte opcode changes: 7-20 T-states vs 44+ for memory reads
    let screen_addr = y * 32 + x;
    // ...
}
```

### Zero-Cost Iterator Chains & Lambda Fusion (In Development)

MinZ aims to bring functional-style iterator chains to Z80 — with zero runtime overhead. The compiler fuses chains like `.map().filter().forEach()` into a single tight loop, inlining all lambdas and using DJNZ where possible.

**Target syntax:**

```minz
// Functional iterator chain — compiles to ONE loop, zero allocations
scores.iter()
    .map(|x| x + 5)
    .filter(|x| x >= 90)
    .forEach(|x| print_u8(x));

// In-place mutation with ! variants
enemies.filter!(|e| e.health > 0);
particles.forEach!(|p| p.update());

// Generators (planned)
gen fibonacci() -> u16 {
    let a: u16 = 0;
    let b: u16 = 1;
    loop {
        yield a;
        let tmp = a + b;
        a = b;
        b = tmp;
    }
}
```

**What the compiler produces** — the entire chain fuses into ~25 T-states/element:

```asm
; scores.iter().map(|x| x + 5).filter(|x| x >= 90).forEach(|x| print_u8(x))
;
; No intermediate arrays. No function call overhead. Just one DJNZ loop.

    LD HL, scores            ; source pointer
    LD B, scores_len         ; counter in B for DJNZ
.loop:
    LD A, (HL)               ; load element         (7 T)
    ADD A, 5                 ; .map(|x| x + 5)      (4 T)
    CP 90                    ; .filter(|x| x >= 90) (7 T)
    JR C, .skip              ; skip if < 90
    CALL print_u8            ; .forEach(...)
.skip:
    INC HL                   ; next element          (6 T)
    DJNZ .loop               ; dec B, loop          (13 T)
```

Compare: a naive indexed loop with separate map/filter passes would cost 60-150+ T-states/element and allocate intermediate arrays. The fused version uses O(1) memory and runs 3-5x faster.

**Key optimizations:**
- **Lambda inlining** — closures compile to direct `CALL` or inline code, never heap-allocated
- **Iterator fusion** — multi-stage chains merge into a single loop at compile time
- **DJNZ loops** — arrays ≤255 elements use Z80's dedicated loop instruction (13 T-states vs 25+ for compare-jump)
- **Pointer arithmetic** — `HL` walks the array with `INC HL`, no index multiplication

**Testing (v0.19.5):** 87+ tests across 7 layers — every stage of the pipeline has dedicated coverage:

| Layer | Tests | Status |
|-------|------:|--------|
| E2E shell (hex-verified output) | 11 | **all pass** |
| Corpus (full compile to Z80) | 18 | all pass |
| Fusion optimizer (callback inlining) | 7 | all pass |
| MIR VM (DJNZ execution) | 8 | all pass |
| Codegen (Z80 patterns) | 7 | all pass |
| Semantic (IR generation) | 20 | all pass |
| Parser (chain conversion) | 18 | all pass |

**9 operations fully working on Z80:** forEach, map, filter, take, skip, peek, inspect, takeWhile, and inline lambda filters (`filter(|x| x > N)` compiles to `CP N+1` + `JR C` — no function call, ~27 T-states saved per iteration). **Fusion optimizer** inlines small callbacks directly into DJNZ loop bodies, eliminating CALL/RET overhead and enabling bare `DJNZ` instruction. enumerate and reduce work at MIR level, Z80 blocked by OpPush routing. See [Iterator Implementation Status](docs/Iterator_Implementation_Status.md) for details.

**Documentation:**
- [Iterator Implementation Status](docs/Iterator_Implementation_Status.md) — actual compiler output, known bugs, performance reality
- [Iterator Reality Check (Report #017)](reports/2026-03-02-017-Iterator_Reality_Check.md) — grounded analysis of T-state costs
- [ADR-0008: Flag-Based Boolean ABI](docs/adr/0008-flag-based-boolean-abi-for-iterators.md) — `CP` + flag returns for iterator predicates

---

## Platform Targets

### Z80 Targets (Primary)

| Target | Status | Binary | Notes |
|--------|--------|--------|-------|
| **ZX Spectrum** | Working | `.tap` | Main development target, tested via mze + ZXSpeculator |
| **CP/M** | Working | `.com` | BDOS stdlib, tested via mze with CP/M mode |
| **Agon Light 2** | Working | `.bin` | eZ80/ADL mode, MOS + VDP stdlib, structural testing only |
| **MSX** | Compiles | varies | Target config exists, limited testing |

### Backends

| Backend | Status | Notes |
|---------|--------|-------|
| **Z80** | ✅ Production | Full-featured, optimized, 5500+ lines, MIR2 active target |
| **QBE (native)** | ✅ Working | MIR2→QBE IL→arm64/x86_64. Correctness oracle: 4/4 E2E tests. `brew install qbe` |
| **C99** | ⚠️ Partial | Produced real binaries; variable redeclaration bug in scoped locals |
| **M68k** | 🧪 Untested | Most complete non-Z80 (28 opcodes, real register allocator); never assembled |
| **i8080** | 🧪 Untested | Structurally correct (all-memory approach); never assembled |
| **6502** | ⚠️ Working | **35/35 tests**, E2E via sim6502 emulator, dual-VM cross-check, console I/O (A2/C64/BBC). [Report #067](reports/2026-03-13-067-6502_Backend_E2E_Harness.md) |
| **LLVM** | ❌ Broken | JumpIf fallthrough hardcoded, type errors; llc fails |
| **WASM** | ❌ Broken | Label/jump emit as comments; WAT validation fails |
| **Crystal** | ❌ Stub | Control flow emits comments, function args always empty |
| **Game Boy** | ❌ Stub | Add, Sub, LoadVar, StoreVar all emit only comments |

Only Z80 is production-quality. **QBE is new (2026-03-09)** — `pkg/mir2qbe` translates MIR2 directly to QBE IL, which compiles to native arm64/x86_64 via `qbe` + `cc`. Used as a correctness oracle: same MIR2 module → Z80 emulator vs native binary; agreement means the pipeline is correct. See [Report #045](reports/2026-03-09-045-MIR2_To_QBE_Native_Backend_And_Correctness_Oracle.md).

### 6502 Backend — E2E Verified (NEW)

The MOS 6502 backend compiles MIR2 IR to valid NMOS 6502 assembly, assembled
and executed on an in-process emulator.  **35/35 tests pass.**

```
MIR2 IR ─┬─→ VM.Call()           → reference ─┐
          │                                     ├→ assert equal (dual-VM oracle)
          └─→ M6502Codegen → asm → sim6502 → A ┘
```

**What works (E2E verified):** `add`, `sub`, `neg`, `double`, `and`/`or`/`xor`,
function calls (`JSR`/`RTS`), constants, console output (4 platforms).

**Console I/O** — four OS vectors captured simultaneously (zero conflicts):

| Address | System | Convention |
|---------|--------|-----------|
| `$F001` | Bare metal | `STA $F001` (I/O port) |
| `$FDED` | Apple II | `JSR $FDED` (COUT) |
| `$FFD2` | C64 | `JSR $FFD2` (CHROUT) |
| `$FFEE` | BBC Micro | `JSR $FFEE` (OSWRCH) |

All share the same calling convention: char in A.  In MinZ:
`@extern("$FFEE") fun putchar(c: u8)`.

**Missing (roadmap):** loops, 16-bit math, memory access, SMC.
See [Report #067](reports/2026-03-13-067-6502_Backend_E2E_Harness.md) for
the full feature matrix and Z80 vs 6502 comparison.

### Language Frontends

Eight source languages compile through the same HIR → MIR2 → Z80 backend:

```
  .nanz ──→ nanz.Parse()     ──┐
  .frl  ──→ frill.Parse()    ──┤  ← NEW: ML-style functional
  .lanz ──→ lanz.Compile()   ──┤
  .lizp ──→ lizp.Compile()   ──┤
  .plm  ──→ plm.Compile()    ──┼──→ *hir.Module ──→ MIR2 ──→ Z80/6502/QBE
  .pas  ──→ pascal.Compile()  ──┤
  .c    ──→ c89.Compile()    ──┤
  .abap ──→ abap.Compile()   ──┘
```

| Frontend | Status | Purpose | Notes |
|----------|--------|---------|-------|
| **Nanz** | Primary | Modern systems language | Full-featured: structs, ADT enums, match expressions, iterators, lambdas, SMC, LUTGen, flag-return ABI |
| **Frill** | Working | ML-style functional | ADTs, pattern matching, polymorphism, effects, property testing, pipes. 38 features, 3351 checks. [Guide](docs/Frill_Language_Guide.md) |
| **Lanz** | Working | S-expression IR | 1:1 mapping to HIR. Round-trips perfectly. Used by `@derive_*` metafunctions |
| **Lizp** | Working | Lisp dialect | Macros, threading (`->`, `->>`), `defmacro`/`cond`/`when`/`dotimes`. Desugars to Lanz |
| **PL/M-80** | Working | Legacy Intel (1976) | 26/26 Intel 80 Tools corpus (100%); 1338 functions, 11661 statements |
| **Pascal** | Working | Turbo Pascal | `WriteLn` → CP/M BDOS via inline asm. `mz hello.pas -t cpm -o hello.com` |
| **C89** | WIP | C89/C99 | `modernc.org/cc/v4` parser. 9 corpus files, 68 asserts. [−55% vs SDCC](reports/2026-03-15-081-MinZ_C89_vs_SDCC_Codegen_Comparison.md) |
| **ABAP** | NEW | SAP ABAP on Z80! | [abaplint](https://github.com/abaplint/abaplint) parser (TS). DATA, WRITE, IF, WHILE, DO, FORM, CLASS. [Examples →](examples/abap/) |
| **MinZ** | Frozen on MIR1 | Legacy syntax | Old MIR1 path; will be rewired through HIR→MIR2 |

**Eight pipelines, one backend.** `.nanz`, `.frl`, `.lanz`, `.lizp`, `.plm`, `.pas`, `.c`, and `.abap` files all go through `compileViaHIR()` → HIR → MIR2 → Z80. A function `double(x) = x + x` written in any of the eight languages produces the same Z80: `ADD A, A / RET`.

### ABAP on Z80 — Yes, Really

```abap
REPORT zfibonacci.

DATA: lv_a TYPE i VALUE 0,
      lv_b TYPE i VALUE 1,
      lv_temp TYPE i,
      lv_i TYPE i VALUE 0.

WHILE lv_i < 10.
  WRITE lv_a.
  lv_temp = lv_a + lv_b.
  lv_a = lv_b.
  lv_b = lv_temp.
  lv_i = lv_i + 1.
ENDWHILE.
```

This compiles through: ABAP → [abaplint](https://github.com/abaplint/abaplint) (TypeScript parser by Lars Hvam Petersen) → JSON AST → Go lowerer → HIR → MIR2 → Z80 assembly. Your ZX Spectrum is now an enterprise-grade ABAP runtime. See [8 examples](examples/abap/) including FizzBuzz, bubble sort, OOP with interfaces, and a system info report.

**Cross-language imports** — Nanz can import from any frontend:

```nanz
import mathlib    // finds mathlib.nanz, .lanz, .lizp, .plm, or .pas
import legacy { PLM_ADD }           // PL/M-80 procedure
import macrolib { lizp_double }     // Lizp function
```

**Universal compile-time assert** — all 6 frontends produce the same `hir.Assert`, verified by dual-VM (MIR2 VM + Z80 binary):

| Frontend | Syntax |
|----------|--------|
| Nanz | `assert double(5) == 10` |
| Lanz | `(assert double 5 == 10)` |
| Lizp | `(assert double 5 == 10)` |
| PL/M-80 | `ASSERT DOUBLE(5) = 10;` |
| Pascal | `assert Double(5) = 10;` |
| C89 | `// assert double(5) == 10` |

**Pascal on CP/M** — hello world in one command:

```bash
mz hello.pas -t cpm -o hello.com && mze -t cpm hello.com
# Output: Hello from Pascal on Z80!
```

**PL/M-80 coverage** (Intel 80 Tools corpus): algolm compiler, BASIC-E compiler/parser/synthesizer, ML80 assembler (l81/l82/l83/m81), TeX, CP/M utilities, Kermit — 1338 functions / 943 globals / 11661 statements lowered to HIR from 26 source files. Handles LITERALLY macro chains, `$INCLUDE` with CP/M device designators, binary literals, record field access, EXTERNAL procedures, all PL/M-80 statement forms. See [ADR-0014](docs/adr/0014-plm80-frontend-strategy.md).

**Pipeline emit flags** (works with all frontends):

```bash
mz program.plm --emit=nanz       # Transpile PL/M → Nanz (round-trip)
mz program.pas --emit=lanz       # Transpile Pascal → Lanz
mz program.lanz --emit=nanz      # Transpile Lanz → Nanz
mz program.plm --emit=hir        # HIR typed-tree dump
mz program.plm --emit=mir2       # MIR2 after optimisation passes
mz program.plm -o prog.com -t cpm  # Assemble to CP/M binary
```

The Nanz transpiler is lossless: `mz prog.plm --emit=nanz | mz --stdin` produces
byte-identical assembly to compiling `.plm` directly.

See [Chapter 21 of the Nanz Language Book](docs/Nanz_Language_Book_v5.md) for the full cross-language import guide.

---

## Toolchain — End-to-End Development Ecosystem

MinZ provides a complete, self-contained development ecosystem. Every tool you need — from source code to running program to screenshot — is a single Go binary with zero external dependencies. No fragile toolchain of third-party assemblers, separate emulators, or external debuggers. One `make` builds everything.

```
Source Code                          Running Program
    |                                      |
    v                                      v
  [mz] compile ──> [mza] assemble ──> [mze] run (CP/M, headless)
    |                                  [mzx] run (ZX Spectrum, graphical)
    |                                  [mzrun] run (remote, DZRP)
    |                                      |
    v                                      v
  [mzd] disassemble <──────────────── [mzx --screenshot] capture
```

| Tool | Purpose | Usage |
|------|---------|-------|
| **mz** | MinZ compiler | `mz program.minz -o program.a80` |
| **mza** | Z80 assembler (table-driven, all Z80 ops including undocumented, `[addr]` bracket syntax) | `mza program.a80 -o program.com` |
| **mze** | Z80 emulator (1335/1335 FUSE tests, profiler, console I/O, stderr port) | `mze program.com -t cpm --console-io` |
| **mzx** | ZX Spectrum emulator (T-state accurate, AY, profiler, .sna/.tap/.trd/.scl, console I/O) | `mzx --snapshot game.sna` |
| **mzd** | Z80 disassembler (IDA-like analysis, xrefs, ROM tables) | `mzd program.bin --org 0x8000` |
| **mzrun** | Remote runner (DZRP protocol) | `mzrun program.minz --reset` |
| **mzv** | MIR VM runner (breakpoints, tracing, PNG export) | `mzv program.mir` |
| ~~**mzr**~~ | ~~Interactive REPL~~ | ❌ Broken — compilation pipeline not wired |
| **mzlsp** | LSP server (diagnostics, hover, goto-def, completion) | auto-started by VSCode extension |

### MZX — ZX Spectrum Emulator

T-state accurate emulation with real display output. Supports 48K and Pentagon 128K models.

```bash
# Interactive emulation
mzx --snapshot game.sna
mzx --tap game.tap
mzx --model pentagon --rom 128-0.rom --rom1 trdos.rom --trd game.trd

# Load raw binary and run (no ROM needed)
mzx --load code.bin@8000 --set PC=8000,SP=FFFF,DI
mzx --run code.bin@8000   # shortcut for --load + --set PC + SP + DI

# Bare-metal console I/O (no ROM needed)
mzx --run code.bin@8000 --frames DI:HALT --console-io
# OUT ($23),A → stdout | IN A,($23) → stdin | OUT ($25),A → stderr
# DI + HALT → exit with A register as process exit code

# Console I/O with custom port or AY serial
mzx --run code.bin@8000 --frames DI:HALT --console-to-port '$FF'
mzx --run code.bin@8000 --frames DI:HALT --console-to-port ay

# BASIC console (RST $10, needs ROM)
mzx --snapshot game.sna --console

# Headless screenshots (for CI, automated testing, book illustrations)
mzx --snapshot game.sna --screenshot shot.png --frames 100
mzx --tap game.tap --screenshot shot.png --screenshot-on-stable 3

# Execution profiling (7-channel heatmap + memory snapshot)
mzx --snapshot demo.sna --profile heatmap.json --frames 500
# Profile includes: exec, read, write, stack_push, stack_pop, io, mem_snapshot
mzx --snapshot demo.sna --trace trace.jsonl --trace-frames 100:200

# Debugging
mzx --warn-on-halt --verbose --diag --snapshot game.sna
```

Features: FrameMap ULA rendering, beeper + AY-3-8912 audio (AYumi), ULA contention, .sna/.tap/.trd/.scl format support, full TR-DOS function dispatch, 7-channel execution profiler (exec/read/write/stack push/pop/IO + memory snapshot), basic-block tracer, conditional screenshots, T-state snapshots, DI+HALT exit with A as exit code, bare-metal console I/O (port $23 stdout, $25 stderr, or AY serial), 48K ROM included.

### Live Testing with DZRP

For ZX Spectrum development, `mzrun` compiles, assembles, and uploads to a running emulator in one command:

```bash
# Start ZXSpeculator with DZRP enabled, then:
export DZRP_HOST=localhost DZRP_PORT=11000
mzrun game.minz --reset -v
```

### Debug Flags

```bash
mz program.minz --dump-mir       # Show MIR intermediate representation
mz program.minz --dump-ast       # AST in JSON format
mz program.minz --viz out.dot    # MIR visualization (Graphviz)
mz program.minz -d               # Verbose compilation details
mz program.minz --compile-trace  # Structured log of all optimization decisions
```

---

## Standard Library

Stdlib modules are organized by domain. Quality varies — some modules are well-tested, others are experimental.

### Tested and Working

| Module | Description |
|--------|-------------|
| `cpm/bdos` | CP/M BDOS calls: `putchar`, `getchar`, `print_string`, file I/O |
| `agon/mos` | Agon MOS API: `mos_putchar`, `mos_puts`, file I/O (eZ80 ADL mode) |
| `agon/vdp` | Agon VDP graphics: modes, shapes, sprites, buffer commands |
| `text/format` | Number formatting: `u8_to_str`, `u16_to_hex` |
| `mem/copy` | Fast memory ops: `memcpy`, `memset` (LDIR-based) |

### Available but Less Tested

| Module | Description |
|--------|-------------|
| `math/fast` | Sin/cos/sqrt lookup tables (256 entries) |
| `math/random` | LFSR PRNG, noise functions |
| `graphics/screen` | Pixel/line/circle drawing (ZX Spectrum) |
| `input/keyboard` | Keyboard matrix, debouncing |
| `text/string` | strlen, strcmp, strcpy, strcat |
| `sound/beep` | Beeper SFX |
| `time/delay` | Frame timing, delays |

### Experimental

| Module | Description |
|--------|-------------|
| `glsl/*` | GLSL-style shader library: fixed-point math, raymarching, SDFs |

---

## Optimization Pipeline

MinZ applies optimizations at multiple levels:

1. **CTIE** — Pure functions with constant args execute at compile time
2. **MIR optimizer** — Constant folding, strength reduction, dead code elimination
3. **True SMC** — Self-modifying code patches parameters into instruction immediates
4. **Loop rerolling** — Detects repeated call sequences, collapses to loops
5. **Peephole optimizer** — 35+ Z80-specific assembly patterns

Example: `fibonacci(10)` with CTIE generates `LD A, 55` — zero runtime cost.

---

## Project Structure

```
minz/
  minzc/             Compiler & toolchain (Go, ~90K LOC)
    cmd/               CLI tools
      minzc/             mz — MinZ compiler
      mza/               mza — Z80 assembler
      mze/               mze — Z80 emulator (headless)
      mzx/               mzx — ZX Spectrum emulator (graphical)
      mzd/               mzd — Z80 disassembler
      mzrun/             mzrun — DZRP remote runner
      mzr/               mzr — REPL
    pkg/               Core packages
      parser/            Participle-based parser
      semantic/          Type checking, analysis (~11K lines)
      ir/                Intermediate representation
      codegen/           Z80 (production), C (partial), + 8 experimental backends
      optimizer/         MIR + peephole optimizers
      z80asm/            Z80 assembler engine (table-driven)
      spectrum/          ZX Spectrum emulation (ULA, AY, memory, ports)
      emulator/          Z80 CPU emulation (remogatto/z80, FUSE-tested)
      disasm/            Disassembler with IDA-like analysis
  stdlib/            Standard library (.minz)
    agon/              Agon Light 2 (MOS, VDP)
    cpm/               CP/M (BDOS)
    graphics/          Screen drawing
    math/              Fast math, PRNG
    text/              String, formatting
    ...
  examples/          270+ example programs
  docs/              Technical documentation
  reports/           Progress reports (date-numbered)
```

---

## Current Status (March 2026)

**Active pipeline:** Nanz/Lanz/Lizp/PL/M-80/Pascal/ABAP → HIR → MIR2 → Z80 (production) / QBE (native) / 6502 (experimental).

**Metrics (verified 2026-03-15):**

| | |
|---|---|
| **Language frontends** | 7 (Nanz, Lanz, Lizp, PL/M-80, Pascal, C89, **ABAP**) |
| **Nanz showcase** | 34/34 compile + verify |
| **Compile-time asserts** | 113 across all 6 frontends (45 original + 68 C89) |
| **Go test packages** | 26/26 pass |
| **6502 backend** | 35/35 E2E tests |
| **Z80 emulator** | 1335/1335 FUSE tests (100%) |
| **PL/M-80 corpus** | 26/26 Intel 80 Tools files (100%) |
| **MIR2→QBE** | 4/4 E2E (correctness oracle) |
| **Toolchain** | 9 working binaries, pure Go, zero deps |

**Known limitations:** strict `>` codegen on Z80 (flag polarity), struct alloca encoding, `as` cast syntax not parsed. MinZ `.minz` frozen on MIR1. See [Open Bugs](docs/Open_Bugs_RCA.md).

---

## Development

See [docs/GenPlan.md](docs/GenPlan.md) for the development roadmap and current priorities.

---

## Contributing

```bash
# Build all tools
cd minzc
make all

# Run all tests (emulator, assembler, spectrum, parser, etc.)
make test-all

# Test an example end-to-end
./mz ../examples/hello_print.minz -o /tmp/hello.a80
./mza /tmp/hello.a80 -o /tmp/hello.tap
./mze /tmp/hello.tap

# Screenshot an example
./mzx --rom roms/48.rom --snapshot demo.sna --screenshot shot.png --frames 50
```

Report issues at [github.com/oisee/minz/issues](https://github.com/oisee/minz/issues).

---

## License

MIT. See [LICENSE](LICENSE) for details.

---

<div align="center">

**MinZ: Modern syntax for vintage hardware.**

</div>
