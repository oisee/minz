# MinZ Programming Language

<div align="center">

![MinZ Logo](/media/minz-logo-shamrock-mint.png)

### Modern Programming Language for Vintage Hardware

[![Version](https://img.shields.io/badge/version-0.18.0-blue)](https://github.com/oisee/minz/releases)
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
mz program.minz -b crystal -o prog.cr                  # Crystal (testing)
mz program.minz -b c -o prog.c                         # C99
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
| **CTIE** | Compile-time function execution |
| **True SMC** | Self-modifying code optimization |
| **@extern FFI** | `extern fun putchar(c: u8) at 0x10;` with RST optimization |
| **Operator overloading** | `v1 + v2` via `impl` blocks |
| **Error propagation** | `@error(code)` with CY flag ABI |
| **Enums** | `enum State { IDLE, RUNNING }` with values |
| **Module system** | `import stdlib.cpm.bdos;` |
| **Lambdas** | Closure syntax, zero-cost transform |

### Partial / In Progress

| Feature | Status |
|---------|--------|
| Pattern matching | Syntax parses, codegen partial |
| Iterator chains | Compiles, optimization in progress |
| MIR interpreter | Arrays/structs working, not complete |

### Known Limitations

- Register allocator has bugs with overlapping lifetimes in complex loops
- Some loop/arithmetic combinations produce incorrect code
- `loadToHL` can use stale values in multi-expression contexts
- Loop rerolling can be too aggressive across function call boundaries

These are documented and being worked on. Simple programs (hello world, fibonacci, demos) work correctly. Complex programs with nested loops and heavy arithmetic may hit edge cases.

---

## Code Examples

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

**Status:** Lambda-to-function transform works. DJNZ optimization works for `for i in 0..N`. Full method-chain syntax (`.map().filter()`) and fusion optimizer are in active development.

**Design documents:**
- [Zero-Cost Iterators Revolution](docs/Zero_Cost_Iterators_Revolution.md) — complete vision
- [DJNZ Iterator Optimization](docs/2026-01-03-301-DJNZ_Iterator_Optimization.md) — loop optimization details
- [Generator Vision](docs/2026-01-03-302-Generator_Vision_Zero_Cost_Iteration.md) — `gen`/`yield` design
- [Z80 Optimal Iteration Design](docs/Z80_Optimal_Iteration_Design.md) — hardware-level patterns
- [Iterator Implementation Status](docs/Iterator_Implementation_Status.md) — progress tracker

---

## Platform Targets

### Z80 Targets (Primary)

| Target | Status | Binary | Notes |
|--------|--------|--------|-------|
| **ZX Spectrum** | Working | `.tap` | Main development target, tested via mze + ZXSpeculator |
| **CP/M** | Working | `.com` | BDOS stdlib, tested via mze with CP/M mode |
| **Agon Light 2** | Working | `.bin` | eZ80/ADL mode, MOS + VDP stdlib, structural testing only |
| **MSX** | Compiles | varies | Target config exists, limited testing |

### Other Backends

| Backend | Status | Notes |
|---------|--------|-------|
| **Z80** | Production | Full-featured, optimized, 5500+ lines |
| **6502** | Basic | Generates assembly, limited testing |
| **C99** | Working | Useful for algorithm verification |
| **Crystal** | Working | Good for rapid testing |

The Z80 backend is production-quality. C99 and Crystal are useful for testing and verification. 6502 generates code but needs more work.

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
| **mze** | Z80 emulator (1335/1335 FUSE tests) | `mze program.com -t cpm` |
| **mzx** | ZX Spectrum emulator (T-state accurate, AY sound, profiler, .sna/.tap/.trd/.scl) | `mzx --snapshot game.sna` |
| **mzd** | Z80 disassembler (IDA-like analysis, xrefs, ROM tables) | `mzd program.bin --org 0x8000` |
| **mzrun** | Remote runner (DZRP protocol) | `mzrun program.minz --reset` |
| **mzr** | Interactive REPL | `mzr` |

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

# Headless screenshots (for CI, automated testing, book illustrations)
mzx --snapshot game.sna --screenshot shot.png --frames 100
mzx --tap game.tap --screenshot shot.png --screenshot-on-stable 3

# Execution profiling and tracing
mzx --snapshot demo.sna --profile heatmap.json --frames 500
mzx --snapshot demo.sna --trace trace.jsonl --trace-frames 100:200

# Debugging
mzx --warn-on-halt --verbose --diag --snapshot game.sna
```

Features: FrameMap ULA rendering, beeper + AY-3-8912 audio (AYumi), ULA contention, .sna/.tap/.trd/.scl format support, full TR-DOS function dispatch, execution profiler/tracer, conditional screenshots, DI+HALT detection, 48K ROM included.

### Live Testing with DZRP

For ZX Spectrum development, `mzrun` compiles, assembles, and uploads to a running emulator in one command:

```bash
# Start ZXSpeculator with DZRP enabled, then:
export DZRP_HOST=localhost DZRP_PORT=11000
mzrun game.minz --reset -v
```

### Debug Flags

```bash
mz program.minz --dump-mir    # Show MIR intermediate representation
mz program.minz --dump-ast    # AST in JSON format
mz program.minz --viz out.dot # MIR visualization (Graphviz)
mz program.minz -d            # Verbose compilation details
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
      codegen/           Z80, 6502, C, Crystal backends
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

## Current Status (February 2026)

MinZ is under active development. The Z80 backend is mature and produces working binaries for ZX Spectrum, CP/M, and Agon Light 2. The toolchain is now a complete end-to-end ecosystem: write code, compile, assemble, emulate, disassemble, screenshot — all with zero external dependencies.

**What works well:**
- Complete self-contained toolchain: compile -> assemble -> emulate -> screenshot
- T-state accurate ZX Spectrum emulation with display, audio, and tape/disk support
- Execution profiler with memory/IO heatmaps and basic-block trace export
- Raw binary loading (`--load`/`--run`) for testing compiled code without ROM
- Multi-target compilation (same source for Spectrum, CP/M, Agon)
- Compile-time execution (CTIE) for constant expressions
- Inline assembly for performance-critical code
- Z80 CPU emulation verified against FUSE test suite (gold standard)
- DI+HALT lockup detection (`--warn-on-halt`)

**What needs work:**
- Complex programs with nested loops and heavy register pressure
- Register allocator edge cases
- Some optimizer passes can be too aggressive
- Non-Z80 backends are basic
- No IDE integration yet (LSP planned)

**Metrics:**
- ~73 core examples, ~272 total (including experimental)
- ~90K lines of Go in the compiler + toolchain
- 4 active backends: Z80 (production), 6502, C99, Crystal
- 3 validated Z80 targets: Spectrum, CP/M, Agon Light 2
- **1335/1335 FUSE Z80 tests pass** — gold-standard CPU verification including all undocumented opcodes
- 9 toolchain binaries, all pure Go, zero external dependencies

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
