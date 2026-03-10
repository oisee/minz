# MinZ Programming Language

### Hot off the press

- **[The Nanz Language: A Complete Guide](docs/Nanz_Language_Book.md)** — 1,370-line comprehensive book on the Nanz language: syntax reference, type system, compilation pipeline (source → HIR → MIR2 → Z80), iterator chains and fusion, struct methods/UFCS/zero-cost interfaces, LUTGen ranged types, register classes, Z80 assembly output with T-state analysis, and comparison with MinZ and PL/M-80. Grounded in actual parser and lowerer code with real compiler output throughout. Now covers **fused-lambda capture**: `buf.forEach(|x: u8| { s = s + x }, n)` with outer `s` correctly threaded as a Z80 register via block-param mechanism — zero spill, zero heap allocation.
- **Closure capture fix** — `forEach` lambdas that write to outer local variables (`s = s + x` where `s` is declared outside the lambda) no longer panic. The `hasFreeVars` pass in `LowerModule` detects free-variable references in lambda bodies and skips standalone lowering; the lambda is only lowered inline via `lowerFusedForEach` where outer vars are correctly threaded as SSA block params. `TestLambdaCapture` added.
- **[LUT Pointer Selection & PBQP Edge Costs](reports/2026-03-10-049-LUT_Pointer_Selection_And_PBQP_Edge_Costs.md)** — Page-aligned LUT fast path trimmed from **21T → 18T** (−14%, −1 byte) by replacing `LD HL, sym` with `LD H, sym^H` — the low byte was immediately overwritten by the index so only the page base (high byte) matters. Full pointer-register timing table: HL/DE/BC all achieve 18T for page-aligned LUT; **BC★ achieves 14T** when the index is already in C (planned codegen check); IX/IY cost **38T** due to the 12T `(IX+d)` DD-prefix penalty and are excluded from LUT access. Analysis documents where this belongs architecturally: instruction selection (current fix), post-allocation codegen check (BC★), and full **PBQP edge costs** (Phase 6e) — edge costs are the correct formulation for correlated allocation decisions (`idx→C` + `ptr→BC` cheaper than independent assignments). [ADR-0017](docs/adr/0017-lut-pointer-selection-and-pbqp-edge-costs.md) written.
- **[Phase 6: Register Allocator Revolution — PBQP, IX/IY, Copy Coalescing](reports/2026-03-10-048-Phase6_Register_Allocator_Revolution.md)** — Three phases shipped in one session: **(6a)** IX/IY indexed addressing in Z80 codegen — `(IX+0)` / `(IX+1)` displacement, undocumented `LD IXl,L / LD IXh,H` byte-copy (16T vs 21T for PUSH/POP), no more invalid bare `(IX)`; **(6b)** full PBQP allocator replacing greedy — weighted cost vectors (`useCount × slotCost`), R0/R1/RN reduction rules, **delta sort** (`2nd_best − best`) ensures hot registers claim zero-cost locations first: `r_heavy(10×) → A (0T), r_light(1×) → C (6T)` vs greedy's arbitrary ordering; **(6c)** post-allocation copy coalescing eliminates trampolines at block boundaries by matching block param and arg physical locations — single-pass with `recolored` lock to avoid rotation cycles in loop phi-webs. Combined result: **four simultaneously-live ClassPointer registers → HL/DE/BC/IX, zero $F0xx spills**, correct `(IX+0)` addressing, 10 new tests, 23/23 packages green.
- **[Nanz Week 1: Struct Methods, UFCS, Zero-Cost Interfaces + Phase 6 RCA](reports/2026-03-09-046-Nanz_Week1_RCA_And_Phase6_Plan.md)** — Go-style interfaces (`interface Animal { speak }`) compile to **direct `CALL`** — zero vtable, zero indirection. Three animals, three `speak` implementations, one `all_speak` caller: `CALL Dog_speak` / `CALL Cat_speak` / `CALL Bird_speak` — each 17T on Z80 vs ~55T for Go's interface dispatch. Struct methods (`fun Dog.speak(self: Dog)`), UFCS dispatch, operator overloading, struct-typed parameters, and `interface` declarations all wired into the HIR→MIR2→QBE pipeline. 3 new E2E tests verify the full chain (parse → HIR → MIR2 → QBE → native binary). Honest RCA of current Z80 codegen bugs (`LD A, HL` invalid instruction, self-param spill to `$F0xx`) with root causes traced to `VarRefExpr.Ty = TyU8` hardcoding and missing `PtrAdd(x, 0) → x` fold. Phase 6 plan: spill cost model + ClassIX/IY as overflow pool, full PBQP graph coloring, copy coalescing.
- **[MIR2→QBE: Native Backend & Correctness Oracle](reports/2026-03-09-045-MIR2_To_QBE_Native_Backend_And_Correctness_Oracle.md)** — `pkg/mir2qbe` compiles MIR2 modules to [QBE IL](https://c9x.me/compile/) and runs them natively on arm64/x86_64. Full pipeline: Nanz/PL/M → HIR → MIR2 → QBE → `cc` → native binary. Correctness oracle: if Z80 emulator and native binary agree, the bug (if any) is in Z80 codegen. 4/4 E2E tests: PL/M `abs_diff` + `fib`, Nanz `sum_array` (real `ptr[i]` loop with bidirectional pointer type inference), Nanz `abs_diff`. Side-by-side QBE IL vs hand-written + arm64 disasm comparison. `brew install qbe` is all you need.
- **[E2E Overview: Architecture, Frontends, MIR2, and PBQP Roadmap](reports/2026-03-09-043-E2E_Overview_Architecture_And_Roadmap.md)** — comprehensive deep-dive: what Nanz/MinZ/PL/M-80 each are and why, MIR1 vs MIR2 design philosophy, how register classes map to PBQP, interprocedural contract optimization with before/after assembly, LUTGen, flag-return ABI, JRS, and the full roadmap to graph-coloring PBQP. Honest gap list included.
- **[JRS pseudo-instruction in MZA](minzc/pkg/z80asm/fake_instructions.go)** — codegen now emits `JRS` for all local-label branches. MZA expands to `JR` (2 bytes) when offset fits and condition is JR-compatible (NZ/Z/NC/C), auto-promotes to `JP` (3 bytes) when offset > ±127 via existing multi-pass convergence, and emits `JP` directly for conditions JR doesn't support (PE/PO/P/M). Zero codegen complexity — MZA sorts it out.
- **[LUTGen: compile-time lookup tables from ranged types](reports/2026-03-09-038-Nanz_PLM_HIR_MIR2_Z80_E2E_Snapshot.md)** — annotate a parameter with `u8<0..255>` and the compiler evaluates the function body at compile time for all 256 inputs, emitting a page-aligned `DB` table. A `popcount` loop that runs 8 iterations becomes 3 instructions + RET at runtime (`LD HL, lut / LD L, C / LD A, (HL) / RET`).
- **[Nanz & PL/M: Factual Status, Real Examples](reports/2026-03-09-042-Nanz_PLM_Factual_Status_And_Examples.md)** — three separate pipelines, what each can do, real compiled Z80 output, honest gaps. Corrects earlier claim that "Nanz is what MinZ lowers to".
- **[Native PL/M-80 V4.0 vs MIR2 Z80 Backend](reports/2026-03-09-036-Native_PLM80_vs_MIR2_Codegen_Comparison.md)** — `plm80c` (Intel PL/M-80 V4.0, built from source) vs our MIR2 backend: **−46% code size** (80B→43B), zero memory traffic in register-allocated loop body vs 6 loads/4 stores per iteration. Full side-by-side listing with T-state analysis.
- **[Full Pipeline Walk-Through: PL/M → Nanz → HIR → MIR2 → Z80](reports/2026-03-09-035-Pipeline_All_Stages_Walkthrough.md)** — all intermediate stages with real output: `--emit=nanz`, `--emit=hir`, `--emit=mir2-raw`, `--emit=mir2`, `.a80`, and `mzd` disassembly. Three-path comparison (PLM direct / PLM→Nanz / native Nanz). HIR dump reveals a type-inference bug in the PL/M frontend fixed by the Nanz round-trip.
- **[PL/M-80 E2E Pipeline: `mz file.plm -o file.com`](reports/2026-03-09-034-PLM_To_Z80_Pipeline_Walk_Through.md)** — `compileViaHIR` function wired, binary verified via MZE emulator (`fib(10)=55`, `abs_diff(10,3)=7`, `max3(5,12,7)=12`). `--emit=mir2`, `--emit=mir2-raw`, `--emit=hir` flags added.
- **[PL/M-80 Frontend: 26/26 corpus, 1338 functions → HIR](reports/2026-03-08-032-PLM80_HIR_Coverage_And_Pipeline.md)** — full PL/M-80 parser (100% Intel 80 Tools corpus), preprocessor with `$INCLUDE` + LITERALLY alias chains, 1338 functions / 11661 statements lowered to HIR. ADR: [0014](docs/adr/0014-plm80-frontend-strategy.md). Pipeline: PL/M-80 → HIR → MIR2 → Z80 asm end-to-end wired.
- **[PL/M-80 Parser: 26/26 corpus coverage](reports/2026-03-08-031-PLM80_Parser_Corpus_100pct.md)** — LITERALLY macro chains, `$INCLUDE` CP/M resolution, `''` escaped quotes, binary literals, record field access. [ADR-0014](docs/adr/0014-plm80-frontend-strategy.md).
- **[MIR2 Architecture & Progress (~22%)](reports/2026-03-07-029-MIR2_Architecture_And_Progress.md)** — topology-aware `holdsPhys`, SoA256 layout (H=field/L=index), PBQP domain map, progress bar. ADRs: [0011](docs/adr/0011-mir2-codegen-dse-and-shadow-guard.md) [0012](docs/adr/0012-mir2-array-layout-soa256.md). [Roadmap](docs/MIR2_Roadmap.md).
- **[MIR2 Codegen Quality Sprint](reports/2026-03-07-028-MIR2_Codegen_Quality_Sprint.md)** — 42 tests, 9 verified Z80 functions (gcd, max3, popcount, min8 + prior), shadow register guard, DSE pass, AND/OR/XOR immediate peepholes. Real assembly quality: min8 = 5 instructions, max3 = 7 on hot path.
- **v0.20.1: Profiler + Emulator upgrades** — 7-channel profiler (exec/read/write/stack push/pop/IO + memory snapshot), stderr port $25, DI+HALT exit with A register as process exit code. Stack depth tracking via SP-delta detection.
- **[Honest Assessment — Code-Verified Status](reports/2026-03-04-025-Honest_Assessment_Code_Verified.md)** — Every claim verified by live test runs: 75% compile rate, 1 production backend, what actually works vs. what doesn't
- **[MIR Backend Test Suite](reports/2026-03-04-023-MIR_Backend_Test_Suite.md)** — 11 handcrafted .mir programs, full MIR→Z80→binary→emulate pipeline validation (9/11 pass)
- **[VSCode: Edit, Compile & Run in One Click](reports/2026-03-04-022-VSCode_Tooling_And_Codegen_Fixes.md)** — Cmd+Alt+R compiles and runs MinZ in the terminal. 3 SMC codegen fixes, loop rerolling in action, 25% binary size savings. Try it: `examples/cpm/playground.minz`
- **[MIR Language Compatibility Deep Dive](reports/2026-03-03-021-MIR_Language_Compatibility_Deep_Dive.md)** — why PL/M scores 9/10 and Ada 4/10, what to fix, MIR vs SDCC/cc65/z88dk/QBE/ACK ([comparison](docs/MIR_vs_Other_8bit_IRs.md))
- **[MIR Analysis: Multi-Language IR?](reports/2026-03-02-020-MIR_Analysis_Multi_Language_IR_Feasibility.md)** — 118 opcodes, 24 types, 13+ optimizer passes. Can it compile PL/M and Ada? ([architecture guide](docs/MIR_Architecture_Guide.md))
- **[VSCode Tooling Sprint](reports/2026-03-02-019-VSCode_Tooling_Sprint_Report.md)** — LSP server, full syntax highlighting, SLD source maps, DeZog debugging, 10 compile commands ([guide](docs/VSCode_Tooling_Guide.md))
- **[Register Allocator Overhaul](reports/2026-03-02-018-Register_Allocator_Overhaul_Results.md)** — 7.8x iterator speedup (207T → 26T per element), full MinZ→asm pipeline walkthrough
- **[Iterator Reality Check](reports/2026-03-02-017-Iterator_Reality_Check.md)** — honest status: 11/11 E2E correct, before/after the overhaul
- **[Iterator Status](docs/Iterator_Implementation_Status.md)** — 11/11 E2E, 26T/element post-overhaul, operation matrix, known bugs
- **[Project Status](reports/2026-03-01-015-Project_Status_And_Next_Steps.md)** — v0.19 roadmap and priorities

<div align="center">

![MinZ Logo](/media/minz-logo-shamrock-mint.png)

### Modern Programming Language for Vintage Hardware

[![Version](https://img.shields.io/badge/version-0.20.1-blue)](https://github.com/oisee/minz/releases)
[![License](https://img.shields.io/badge/license-MIT-purple)]()

**Write modern code. Run it on Z80, eZ80, 6502, and more.**

[Quick Start](#quick-start) | [Features](#features) | [Examples](#code-examples) | [Targets](#platform-targets) | [Toolchain](#toolchain)

</div>

---

## What is MinZ?

MinZ is a programming language that compiles modern, readable code to efficient assembly for retro hardware — primarily Z80 and eZ80 systems. It includes a self-contained toolchain: compiler, assembler, emulator, and remote runner. No external dependencies.

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
| Pattern matching | Syntax parses, codegen partial |
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
| **6502** | ❌ Broken | Arithmetic uses `$00` placeholder; never assembled |
| **LLVM** | ❌ Broken | JumpIf fallthrough hardcoded, type errors; llc fails |
| **WASM** | ❌ Broken | Label/jump emit as comments; WAT validation fails |
| **Crystal** | ❌ Stub | Control flow emits comments, function args always empty |
| **Game Boy** | ❌ Stub | Add, Sub, LoadVar, StoreVar all emit only comments |

Only Z80 is production-quality. **QBE is new (2026-03-09)** — `pkg/mir2qbe` translates MIR2 directly to QBE IL, which compiles to native arm64/x86_64 via `qbe` + `cc`. Used as a correctness oracle: same MIR2 module → Z80 emulator vs native binary; agreement means the pipeline is correct. See [Report #045](reports/2026-03-09-045-MIR2_To_QBE_Native_Backend_And_Correctness_Oracle.md).

### Language Frontends

Three separate source languages compile through the same HIR → MIR2 → Z80 backend:

| Frontend | Status | Pipeline | Notes |
|----------|--------|----------|-------|
| **Nanz** | Active — primary MIR2 frontend | Nanz → HIR → MIR2 → Z80 | The new surface language; arithmetic, control flow, loops, LUTGen, flag-return ABI, interprocedural CC opt |
| **PL/M-80** | Working | PL/M-80 → HIR → MIR2 → Z80 | 26/26 Intel 80 Tools corpus (100%); 1338 functions, 11661 statements |
| **MinZ** | Frozen on MIR1 | MinZ → MIR1 → old Z80 codegen | Not being developed; will eventually route through HIR→MIR2 once the Participle parser → HIR wiring is done |

**Three pipelines, one backend.** `.nanz` and `.plm` files go through `compileViaHIR()` → HIR → MIR2 → Z80. `.minz` files use the old MIR1 path (`pkg/codegen/z80.go`, 5,800 LOC). MIR1 is frozen; all new work goes into MIR2.

**Nanz** is a minimal, type-safe language designed as a clean target for the MIR2 backend. Working features: arithmetic/bitwise ops, if/else, while, for-range, function calls, u8/u16/i8/i16/bool types, `u8<lo..hi>` ranged types for LUT generation, interprocedural calling-convention optimization, flag-return ABI (comparison results passed via carry flag — no bool materialization).

**PL/M-80 coverage** (Intel 80 Tools corpus): algolm compiler, BASIC-E compiler/parser/synthesizer, ML80 assembler (l81/l82/l83/m81), TeX, CP/M utilities, Kermit — 1338 functions / 943 globals / 11661 statements lowered to HIR from 26 source files. Handles LITERALLY macro chains, `$INCLUDE` with CP/M device designators, binary literals, record field access, EXTERNAL procedures, all PL/M-80 statement forms. See [ADR-0014](docs/adr/0014-plm80-frontend-strategy.md).

**Pipeline emit flags** (works with `.plm` and `.nanz` input):

```bash
mz program.plm --emit=nanz       # Transpile to Nanz surface syntax (round-trip)
mz program.plm --emit=hir        # HIR typed-tree dump (types on every node)
mz program.plm --emit=mir2-raw   # MIR2 before optimisation (DSE/ReorderBlocks)
mz program.plm --emit=mir2       # MIR2 after optimisation passes
mz program.plm                   # .a80 assembly (default)
mz program.plm -o prog.com -t cpm  # Assemble to CP/M binary
```

The Nanz transpiler is lossless: `mz prog.plm --emit=nanz | mz --stdin` produces
byte-identical assembly to compiling `.plm` directly.

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

MinZ is under active development. The Z80 backend is mature and produces working binaries for ZX Spectrum, CP/M, and Agon Light 2. A new compiler backend — **MIR2** — is now the active development target, fed by the **Nanz** and **PL/M-80** frontends via a typed HIR layer.

**What works well:**
- **MIR2 pipeline**: Nanz/PL/M-80 → HIR → MIR2 → Z80, fully wired end-to-end
- **LUTGen**: `u8<lo..hi>` ranged types → compile-time table generation, verified via emulator
- **Flag-return ABI + interprocedural CC optimization**: comparison results travel via flags, no bool materialization
- **JRS pseudo-instruction**: codegen emits `JRS`, MZA picks JR vs JP based on offset — saves 1B per short branch
- Complete self-contained toolchain: compile → assemble → emulate → screenshot
- T-state accurate ZX Spectrum emulation with display, audio, tape/disk support
- Execution profiler with memory/IO heatmaps and basic-block trace export
- Multi-target compilation (same source for Spectrum, CP/M, Agon)
- Compile-time execution (CTIE) for constant expressions
- Z80 CPU emulation verified against FUSE test suite (gold standard)

**What needs work:**
- MIR2: pointer-indexed array access (`ptr[i]` in loops) — broken due to HL conflict between base pointer and index arithmetic; use `ForEachStmt` (sequential scan) instead
- MIR2: non-zero-lo LUT (e.g. `u8<10..20>`) — contract opt changes param class after LUTGen builds body; unit tests pass, pipeline broken
- MinZ (`.minz`): register allocator stale HL tracking in loops — blocks complex programs (ADR-0006)
- MinZ: 9/11 advanced feature tests fail
- Non-Z80 backends: only C produces any working binaries; rest are stubs/broken
- MZR REPL: broken — compilation pipeline not wired through semantic analysis

**Metrics (verified 2026-03-09):**
- 71/73 core examples compile (97%), 131/173 all examples (75%)
- ~125K lines of Go in the compiler + toolchain
- **MIR2**: 53/53 unit tests pass; E2E fib(1..10), clamp, abs_diff, max3 verified via MZE
- **MIR2→QBE**: 4/4 E2E tests — PL/M + Nanz → native arm64 binary via QBE (correctness oracle)
- **PL/M-80**: 26/26 Intel 80 Tools corpus files parse + compile → Z80 (100%)
- **1335/1335 FUSE Z80 tests pass** — gold-standard CPU verification including all undocumented opcodes
- **87+ iterator tests** across 7 layers — 11/11 E2E hex-verified (MinZ/MIR1 pipeline)
- 24/24 Go test packages pass, 0 fail
- 9 working toolchain binaries (mzr REPL is broken), all pure Go, zero external dependencies

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
