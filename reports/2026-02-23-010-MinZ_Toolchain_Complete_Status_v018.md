# MinZ Toolchain: Complete Status Report (v0.18.0)

## The Self-Contained Universe

MinZ is no longer just a compiler — it's a **complete retro development ecosystem**. Every tool in the pipeline is written in Go, ships as a single binary, and has zero external runtime dependencies.

```
Source Code → MZC (compile) → MZA (assemble) → Binary
                                                  ↓
                                    MZX (run on Spectrum display)
                                    MZE (CPU-level emulate)
                                    MZD (disassemble + analyze)
                                    MZRUN (remote debug via DZRP)
```

## Tool Status Matrix

| Tool | Binary | Purpose | LOC | Status |
|------|--------|---------|-----|--------|
| **MZC** | `mz` | MinZ → Z80 compiler | ~90K | Production |
| **MZA** | `mz` | Z80 assembler (built-in) | ~2K | Production |
| **MZE** | `mze` | Z80 CPU emulator | ~1K | Production (1335/1335 FUSE) |
| **MZD** | `mzd` | Disassembler + analysis | ~3K | Production |
| **MZX** | `mzx` | ZX Spectrum emulator | ~2.5K | Phase 1 Complete |
| **MZRUN** | `mzrun` | DZRP remote runner | ~1K | Production |

**Total toolchain:** ~100K lines of Go

## Recent Milestones (v0.17 → v0.18)

### MZA: Table-Driven Encoder
- **Old:** Hand-coded switch/case chains across 4 files (~2000 lines)
- **New:** Declarative instruction table + pattern matcher (2 files, ~1750 lines)
- **Impact:** Adding instructions is now a one-line table entry, not a multi-file edit
- **Coverage:** All Z80 opcodes including undocumented (IXH, IXL, IYH, IYL operations)

### MZD: IDA Pro-Like Analysis
- **Recursive descent disassembly** — follows control flow, not just linear sweep
- **Cross-references** — bidirectional xrefs for calls, jumps, and data access
- **ABI annotation** — backward parameter scanning detects syscall arguments
- **4 platform profiles** — CP/M, ZX Spectrum, MSX, Agon Light 2 (extensible `.abi` format)
- **Project persistence** — save/load analysis sessions as `.mzp` JSON
- **T-state counting** — exact cycle counts for every Z80 instruction

### MZE: FUSE Test Suite
- **1335/1335 tests pass** — every Z80 opcode, every prefix, every undocumented instruction
- **Gold standard** — FUSE is the reference test suite used by all major Z80 emulators
- **Critical timing semantics** proven: ReadByte=3T, ContendRead=+time, Port=4T

### MZX: ZX Spectrum Emulator
- **T-state accurate** — FrameMap-based ULA, per-instruction screen sync
- **Two models** — 48K (69,888 T/frame, ULA contention) + Pentagon 128K (71,680 T/frame)
- **Full pipeline** — CPU + ULA + memory banking + keyboard + beeper + Ebitengine display
- **26 unit tests** covering all subsystems

## Architecture Highlights

### Shared CPU Core
MZE and MZX use the **same** remogatto/z80 library with the **same** timing guarantees:
- MZE validates correctness via FUSE
- MZX wraps it in a CPUCore interface for future eZ80/Z80Next swaps
- Same memory interface semantics (ReadByte=3T, etc.)

### The CompileAssembleEmulateVisualize Loop
```bash
# Full pipeline example
mz plasma.minz -o plasma.a80 -t spectrum   # compile
mza plasma.a80 -o plasma.sna               # assemble to snapshot
mzx --rom 48.rom --snapshot plasma.sna      # see it on screen
mzd plasma.sna --analyze -t spectrum        # analyze the binary
```

## MZX Phase 2 Roadmap

### Immediate (in progress)

| Feature | Approach | Lines |
|---------|----------|-------|
| **.tap tape loading** | ROM trap at `$0556` — intercept LD-BYTES, inject block data | ~100 |
| **.trd disk images** | WD1793 FDC port traps + Beta 128 ROM paging | ~300 |
| **AY-3-8912 sound** | Go port of AYumi (MIT) — stereo PSG with FIR decimation | ~400 |
| **Screenshots** | `--screenshot` flag for headless PNG capture | ~80 |

### Phase 3: Debugger Integration
- DeZog/DZRP protocol support (breakpoints, step, memory view)
- Integration with mzd analysis engine
- Variable/symbol overlay from mzc debug info

### Phase 4: Multi-Platform
- eZ80 core for Agon Light 2 (via CPUCore interface swap)
- Z80Next core for ZX Next
- .z80 and .tzx format support

## Platform Target Matrix

| Platform | Compiler | Assembler | Emulator | Disassembler | Status |
|----------|----------|-----------|----------|------------|--------|
| ZX Spectrum 48K | MZC | MZA | MZX | MZD (Spectrum ABI) | Full |
| ZX Spectrum 128K | MZC | MZA | MZX (Pentagon) | MZD | Partial |
| CP/M | MZC | MZA | MZE | MZD (CP/M ABI) | Full |
| Agon Light 2 | MZC | MZA | — | MZD (Agon ABI) | Compile only |
| MSX | — | MZA | — | MZD (MSX ABI) | Disasm only |

## Quality Metrics

| Metric | Value |
|--------|-------|
| FUSE Z80 tests | 1335/1335 (100%) |
| MZX unit tests | 26/26 |
| Core example compilation | 72/72 (100%) |
| All examples | ~81% |
| Stdlib modules | 10 |
| Peephole patterns | 35+ |
| Platform ABI profiles | 4 |
| Z80 instruction coverage | 100% (including undocumented) |
