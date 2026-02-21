# MinZ Programming Language

<div align="center">

![MinZ Logo](/media/minz-logo-shamrock-mint.png)

### Modern Programming Language for Vintage Hardware

[![Version](https://img.shields.io/badge/version-0.18.0--dev-blue)](https://github.com/oisee/minz/releases)
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
go build -o mz cmd/minzc/main.go     # Compiler
go build -o mza cmd/mza/main.go      # Assembler
go build -o mze cmd/mze/main.go      # Emulator
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
| **Inline assembly** | `asm { LD A, 42 }` blocks |
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

## Toolchain

All tools are self-contained Go binaries with zero external dependencies.

| Tool | Purpose | Usage |
|------|---------|-------|
| **mz** | Compiler (MinZ to assembly) | `mz program.minz -o program.a80` |
| **mza** | Z80 assembler | `mza program.a80 -o program.com` |
| **mze** | Z80 emulator | `mze program.com -t cpm` |
| **mzrun** | Remote runner (DZRP) | `mzrun program.minz --reset` |
| **mztap** | TAP file loader | `mztap game.tap` |
| **mzr** | Interactive REPL | `mzr` |

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
  minzc/           Compiler (Go)
    cmd/             CLI tools (mz, mza, mze, mzr, mzrun)
    pkg/
      parser/        Participle-based parser
      semantic/      Type checking, analysis (~11K lines)
      ir/            Intermediate representation
      codegen/       Code generators (Z80, 6502, C, Crystal, etc.)
      optimizer/     MIR + peephole optimizers
      z80asm/        Built-in Z80 assembler
  stdlib/          Standard library (.minz)
    agon/            Agon Light 2 (MOS, VDP)
    cpm/             CP/M (BDOS)
    graphics/        Screen drawing
    math/            Fast math, PRNG
    text/            String, formatting
    ...
  examples/        Example programs
    agon/            Agon Light 2 demos
    cpm/             CP/M programs
    games/           Game demos
  docs/            300+ documentation files
```

---

## Current Status (February 2026)

MinZ is under active development. The Z80 backend is mature and produces working binaries for ZX Spectrum, CP/M, and Agon Light 2. Other backends exist but are less developed.

**What works well:**
- Simple to medium programs (hello world, fibonacci, demos, string output)
- Multi-target compilation (same source for Spectrum, CP/M, Agon)
- Self-contained toolchain with zero dependencies
- Compile-time execution (CTIE) for constant expressions
- Inline assembly for performance-critical code

**What needs work:**
- Complex programs with nested loops and heavy register pressure
- Register allocator edge cases
- Some optimizer passes can be too aggressive
- Non-Z80 backends are basic
- No IDE integration yet (LSP planned)

**Metrics:**
- ~73 core examples, ~272 total (including experimental)
- ~90K lines of Go in the compiler
- 4 active backends: Z80 (production), 6502, C99, Crystal
- 3 validated Z80 targets: Spectrum, CP/M, Agon Light 2
- Zero external dependencies

---

## Contributing

```bash
# Build everything
cd minzc
go build -o mz cmd/minzc/main.go
go build -o mza cmd/mza/main.go
go build -o mze cmd/mze/main.go

# Test an example
./mz ../examples/hello_print.minz -o /tmp/hello.a80
./mza /tmp/hello.a80 -o /tmp/hello.tap
./mze /tmp/hello.tap

# Run compiler tests
go test ./pkg/...
```

Report issues at [github.com/oisee/minz/issues](https://github.com/oisee/minz/issues).

---

## License

MIT. See [LICENSE](LICENSE) for details.

---

<div align="center">

**MinZ: Modern syntax for vintage hardware.**

</div>
