# Report #024: MinZ Comprehensive Status Review

**Date**: 2026-03-04
**Version**: v0.19.5
**Scope**: Full ecosystem review — compiler, language, toolchain, IR, testing, roadmap

---

## Executive Summary

MinZ is a modern programming language targeting Z80 and retro hardware, with a self-contained Go toolchain of 10+ tools and ~125K LOC. The core compiler works reliably for straightforward programs. Advanced features (iterators, SMC, CTIE) are functional but constrained by register allocator bugs in loops. The toolchain (assembler, emulators, disassembler, LSP, debugger integration) is remarkably complete for a project of this scope.

**Headline numbers**: 284 examples, 55 stdlib modules, 12 backends, 116 IR opcodes, 1335/1335 FUSE tests, 133 corpus tests, 11 MIR pipeline tests, 61 Go test files.

---

## 1. Toolchain Feature Matrix

### Core Tools

| Tool | Purpose | Status | LOC | Key Capabilities |
|------|---------|--------|-----|------------------|
| **MZ** (minzc) | Compiler | Production | ~60K | Participle parser, semantic analysis, 12 backends, 13+ optimizer passes |
| **MZA** | Z80 Assembler | Production | 9,611 | Table-driven encoder, macros, 6 output formats, undocumented opcodes |
| **MZE** | Z80 Emulator | Production | 3,619 | 100% FUSE-tested, BDOS/MOS handlers, profiler, DAP/DZRP servers |
| **MZX** | ZX Spectrum Emulator | Production | 7,222 | T-state accurate ULA, AY-3-8912 sound, Ebitengine GUI, beeper SFX |
| **MZD** | Z80 Disassembler | Production | 6,713 | IDA-like analysis, recursive descent, XREF, ABI annotation, .mzp projects |
| **MZLSP** | Language Server | Working | 983 | Diagnostics, hover, goto-def, completion, VSCode extension v0.5.0 |
| **MZR** | Interactive REPL | WIP | 427 | History, MIR inspection, multi-backend, module loading |
| **MZV** | MIR Virtual Machine | WIP | 4,604 | Register-based VM, platform abstraction, breakpoints, framebuffer export |
| **MZRUN** | Remote Runner | Production | 1,136 | DZRP protocol, multi-emulator (ZEsarUX/CSpect/ZXSpeculator), headless CI |
| **MZTAP** | TAP File Loader | Production | 645 | TAP parsing, DZRP memory loading, CODE block extraction |

### Pipeline

```
.minz → [MZ] → .a80 → [MZA] → .bin/.com/.sna/.tap
                                    ↓
                        [MZE] local emulation
                        [MZX] ZX Spectrum GUI
                        [MZRUN] remote DZRP
                        [MZV] MIR VM (direct)

.mir  → [MZ] → .a80 → [MZA] → .bin → [MZE] emulation   (NEW: validated by test suite)

Development:  [MZLSP] IDE support  |  [MZR] REPL  |  [MZD] reverse engineering
```

---

## 2. Language Feature Matrix

### Types

| Feature | Status | Notes |
|---------|--------|-------|
| u8, u16, i8, i16 | Done | Full arithmetic |
| bool | Done | |
| void | Done | |
| Structs | Done | Declaration + field access |
| Enums | Done | Values + `State::IDLE` syntax |
| Arrays | Done | Indexing works; literal optimization pending |
| Strings | Done | Ruby interpolation `"Hello #{name}!"` |
| Pointers | Partial | Address-of works, dereferencing limited |
| u24/i24 (eZ80) | Planned | For Agon Light 2 |
| Fixed-point (f8.8) | Planned | |
| Generics `<T>` | Rejected | Use function overloading |
| Option/Result | Rejected | Use `@error` pattern |

### Control Flow

| Feature | Status | Notes |
|---------|--------|-------|
| if/else | Done | |
| while | Done | Register allocator bugs in complex cases |
| for i in 0..n | Done | |
| match/case | WIP | Syntax parses, codegen partial |
| gen/yield | Planned | Lazy iteration |

### Functions & Abstractions

| Feature | Status | Notes |
|---------|--------|-------|
| fun/fn declaration | Done | |
| Multiple returns | Done | |
| Nested functions | Done | |
| Function overloading | Done | |
| Lambdas | Done | Zero-cost, direct CALL |
| UFCS | Done | `obj.method()` via zero-cost interfaces |
| CTIE | Done | Compile-time interface execution |
| @extern FFI | Done | ROM/BIOS calls |
| Operator overloading | Done | |
| Closures | Done | Full capture support |

### Metafunctions

| Feature | Status | Notes |
|---------|--------|-------|
| @define | Done | Text substitution with {0},{1} |
| @print | Done | Optimized string output |
| @if/@elif/@else | Done | Conditional compilation |
| @error | Done | CY flag error propagation |
| @minz[[[ ]]] | WIP | Limited compile-time execution |
| @lua[[[ ]]] | Planned | Lua scripting for metaprogramming |

### Advanced Features

| Feature | Status | Notes |
|---------|--------|-------|
| Iterator chains | Done (with caveats) | 11/11 E2E, ~5x perf overhead from memory regs |
| TRUE SMC | Done | Self-modifying code optimization |
| RST optimization | Done | Auto-convert to RST instructions |
| Inline assembly | Done | `asm { }` blocks |
| Peephole optimizer | Done | 67 asm patterns + MIR passes |
| Fusion optimizer | Done | Inlines callbacks into DJNZ loops |

---

## 3. IR / MIR Design Status

### IR (Intermediate Representation)

| Metric | Value |
|--------|-------|
| Opcodes | 116 (53 portable, 13 abstractable, 22 Z80-only, 28 advanced/SMC) |
| Types | 24 |
| Source | `pkg/ir/ir.go` (2,886 LOC) |
| Format | Register-based SSA-like |

**Opcode categories**: Control flow (11), Data movement (10), Arithmetic (6), Logic (9), Comparison (6), Memory (12), Stack (2), Z80-specific (8), I/O (3), Output (8), Assembly (3), SMC (14+), Advanced (24).

### MIR (MinZ IR)

| Metric | Value |
|--------|-------|
| Text format | Human-readable, hand-writable |
| Parser | `pkg/mir/parser.go` (fixed: SplitN, Locals transition) |
| VM | `pkg/mirvm/` — register-based, 75+ tests, 4,604 LOC |
| Optimizer | 13+ passes (constant folding, DCE, copy prop, loop rerolling, fusion) |
| Backend test suite | 11 programs, 9/11 pass (NEW) |

### Language Compatibility via MIR

Based on Reports #020/#021, MIR can serve as a multi-language frontend target:

| Frontend | Compatibility | Key Gaps |
|----------|--------------|----------|
| PL/M | 9/10 | Minor: typed procedures |
| BASIC | 6/10 | No float, no INPUT, no PRINT formatting |
| Pascal | 6/10 | No SET, no subrange, no WITH |
| Forth | 5/10 | Stack-based mismatch with register IR |
| Ada | 4/10 | No exceptions, no tasking, no generics |

**Cheapest wins**: OpCheckRange (+2 compat points, 1 day), IsPublic flag (+1.5, 5 min), OpReadChar (+1, 1 day).

---

## 4. Backend Matrix

| Backend | Status | LOC | Target Platforms |
|---------|--------|-----|------------------|
| **Z80** | Production | 5,837 | ZX Spectrum, CP/M, Agon Light 2 |
| **i8080** | Experimental | 663 | CP/M (8080 subset) |
| **6502** | Experimental | 527 | C64, NES, Apple II |
| **Game Boy** | Experimental | 330 | DMG/CGB |
| **m68k** | Experimental | 702 | Amiga, Atari ST |
| **8051** | Experimental | ~300 | Microcontrollers |
| **C** | Testing | 721 | Portable C99 output |
| **Crystal** | Testing | 388 | Crystal lang output |
| **WASM** | Experimental | 307 | Web browsers |
| **LLVM** | Experimental | 493 | Any LLVM target |

Z80 is the only production-quality backend. Others generate syntactically valid output but lack full instruction selection and optimization.

---

## 5. Testing Infrastructure

| Suite | Tests | Status | Coverage |
|-------|-------|--------|----------|
| FUSE Z80 emulator | 1,335 | 100% pass | All Z80 instructions including undocumented |
| Corpus manifest | 133 | Varies | basic, advanced, optimization, integration, real-world |
| MIR backend pipeline | 11 | 9 pass, 2 skip | Arithmetic, logic, compare, branch, loop, memory, integration |
| Iterator E2E | 11 | 100% pass | forEach, map, filter, take, skip, lambdas, multi-stage |
| Iterator corpus | 18 | 100% compile | Parser → semantic → codegen path |
| Go unit tests | 61 files | 19/19 packages pass | Parser, semantic, codegen, optimizer, assembler, emulator |
| Example programs | 284 | ~81% compile | Language feature coverage |

### Regression Commands

```bash
make test-all     # Full suite: emulator, assembler, parser, MIR, all packages
make test-mir     # MIR backend pipeline (11 programs)
make bench-mir    # MIR T-state benchmarks

scripts/run_mir_tests.sh --summary    # MIR summary table
scripts/run_e2e_tests.sh              # E2E TSMC tests
scripts/run_corpus_tests.sh           # Corpus manifest tests
```

---

## 6. Standard Library

| Module | Files | Description |
|--------|-------|-------------|
| `std/` | 7 | Core: iterators, I/O, print, error, collections, memory |
| `math/` | 2 | Fast sin/cos/sqrt lookup tables (256 entries) |
| `mathlib/` | 4 | Plasma, multiply, random, compact math |
| `graphics/` | 1 | Pixel/line/circle/rectangle (ZX Spectrum optimized) |
| `input/` | 1 | Keyboard matrix reading, debouncing |
| `text/` | 2 | strlen, strcmp, strcpy, strcat, trim, format |
| `sound/` | 1 | Beeper SFX (click, buzz, jump, explosion) |
| `time/` | 1 | Frame timing, delays, animation helpers |
| `mem/` | 1 | Fast memcpy/memset/memcmp using LDIR |
| `cpm/` | 1 | CP/M BDOS system calls |
| `agon/` | 2 | Agon Light 2 MOS/VDP APIs |
| `zx/` | 2 | ZX Spectrum screen I/O, input |
| `msx/` | 1 | MSX-2 (experimental) |
| `glsl/` | 5 | GLSL graphics (raymarching, SDF, dithering) |
| `glsl2/` | 4 | Extended GLSL (CSG, distance functions) |
| `core/` | 1 | Core I/O primitives |
| `metafunctions/` | 1 | Metaprogramming utilities |

**55 stdlib files** across 17 module directories.

---

## 7. Developer Experience

### VSCode Extension (v0.5.0)

| Feature | Status |
|---------|--------|
| Syntax highlighting | Done — full TextMate grammar (lambdas, asm, iterators, interpolation) |
| LSP diagnostics | Done — parse + semantic errors |
| Goto definition | Done — functions, structs, enums, variables |
| Hover info | Done — types and function signatures |
| Auto-completion | Done — keywords, types, metafunctions, symbols, iterator methods |
| SLD source maps | Done — `--emit-sld` flag, `@src:file:line` annotations |
| DeZog debugging | Done — F5 one-click debug build + launch |
| Compile & Run | Done — target selection (Spectrum/CP/M/Agon) |
| Problem matcher | Done — click-to-error from compiler output |

### Missing Developer Experience

| Feature | Status | Priority |
|---------|--------|----------|
| Incremental document sync | Planned | Medium |
| Workspace symbol search | Planned | Medium |
| Signature help | Planned | Medium |
| Native DAP server | Planned | Low |
| WASM playground | Planned | Low |
| Source context in errors | Planned | Medium |
| Fix suggestions | Planned | Low |

---

## 8. Known Blockers & Gaps

### Critical (blocks complex programs)

| Issue | Impact | ADR |
|-------|--------|-----|
| Register allocator: same phys reg for two live virtuals in loops | While/for loops produce wrong results | ADR-0006 |
| loadToHL stale values | Multi-expression codegen uses wrong values | ADR-0006 |
| Loop rerolling across function call boundaries | Incorrect code when loop contains calls | — |

### Moderate (workarounds exist)

| Issue | Impact | Workaround |
|-------|--------|------------|
| Iterator enumerate: B counter conflict | enumerate broken on Z80 | Use manual counter |
| Iterator reduce: A overwritten | reduce broken on Z80 | Use accumulator pattern |
| Memory-backed virtual regs (~5x overhead) | Iterator perf bottleneck | Direct register allocation (future) |
| T-state counting returns 0 in MIR tests | Benchmark data unavailable | Under investigation |
| LD IX,SP invalid instruction | Functions with Locals in MIR | Avoid Locals section |

### Low (cosmetic / non-blocking)

| Issue | Impact |
|-------|--------|
| codegen package needs `-vet=off` | Pre-existing format string warnings in non-Z80 backends |
| Pattern matching codegen incomplete | Syntax parses but doesn't fully generate |
| Array literal optimization incomplete | IR skeleton exists, codegen not wired |

---

## 9. Roadmap Summary

### Phase 1: Stability (Q1 2026) — 85% Complete
- [x] Liveness analysis, dead register freeing, loadToHL tracking
- [x] Iterator chain fusion optimizer
- [x] MIR backend test suite (NEW: 11 programs, 9/11 pass)
- [x] MIR parser bug fixes
- [x] 19/19 test packages passing
- [ ] Fix remaining register conflicts in complex loops (ADR-0006)
- [ ] Verify loop rerolling across call boundaries

### Phase 2: Infrastructure (Q2 2026) — 10% Complete
- [ ] MZE/MZX shared packages (profiler, console, diagnostics)
- [ ] Backend harmonization across 12 backends
- [ ] Superoptimizer-proven peephole rules (602K candidates)

### Phase 3: Language Features (Q2-Q3 2026) — 5% Complete
- [ ] Pattern matching codegen completion
- [ ] Generator syntax (gen/yield)
- [ ] Array literal optimization
- [ ] MIR interpreter array/struct support

### Phase 4: Platform Expansion (Q3 2026) — 70% (Agon)
- [x] Agon target config, eZ80 instructions, MOS/VDP stdlib
- [ ] 24-bit type codegen, fixed-point math, audio stdlib
- [ ] Real hardware testing

### Phase 5: Developer Experience (Q4 2026) — 60% Complete
- [x] LSP server (basic), SLD source maps, DeZog debugging, VSCode extension
- [ ] Incremental sync, workspace search, signature help
- [ ] Native DAP server, WASM playground

---

## 10. Project Metrics Summary

| Metric | Value |
|--------|-------|
| **Total Go LOC** | ~125,000 |
| **Total Go files** | ~293 |
| **CLI tools** | 10+ (mz, mza, mze, mzx, mzd, mzlsp, mzr, mzv, mzrun, mztap) |
| **Packages** | 28 in minzc/pkg/ |
| **Backends** | 12 (1 production, 11 experimental) |
| **IR opcodes** | 116 |
| **Optimizer passes** | 13+ |
| **Stdlib modules** | 55 files across 17 directories |
| **Example programs** | 284 |
| **Test files** | 61 Go test files |
| **FUSE Z80 tests** | 1,335/1,335 (100%) |
| **Corpus tests** | 133 |
| **MIR pipeline tests** | 11 (9 pass, 2 known bugs) |
| **Iterator E2E tests** | 11/11 hex-verified |
| **Documentation** | 93 doc files + 24 reports |
| **Compilation rate** | ~81% of 284 examples |
| **Platforms** | ZX Spectrum, CP/M, Agon Light 2 |

---

*MinZ v0.19.5: A remarkably complete retro-computing toolchain. The core is solid — the register allocator is the last major barrier to reliable complex program compilation.*
