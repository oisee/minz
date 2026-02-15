# Agon & CP/M Codegen Fixes + Z80 Address Widening

**Date**: 2026-02-14
**Status**: Complete
**Scope**: Z80 Assembler, Code Generator, Targets, Stdlib, Examples

---

## Summary

Two sessions of work fixing Agon Light 2 (eZ80) and CP/M code generation. The core change is widening all assembler address types from `uint16` to `int` to support eZ80's 24-bit ADL mode addressing. Along the way, fixed several codegen bugs: loop rerolling string address loading, TRUE SMC label sanitization, LF-to-CR newline conversion applied to wrong targets, and Agon memory layout limits.

All 3 Agon examples compile and assemble. All 3 CP/M hello world programs compile, assemble, and run correctly in the emulator.

---

## Phase 1: uint16 to int Address Widening

### Problem

The Z80 assembler (`minzc/pkg/z80asm/`) used `uint16` for all addresses, symbols, and instruction targets. The Agon Light 2 uses eZ80 in ADL mode with 24-bit addresses (e.g., `$040000`). A `uint16` can only hold values up to `$FFFF`, silently truncating Agon addresses.

### Solution

Widened all address-related types from `uint16` to `int` across the entire z80asm package. Added `emitWord()` helper that emits 2 bytes in Z80 mode or 3 bytes in ADL mode based on `CPUMode`.

### Files Modified (12 files)

| File | Changes |
|------|---------|
| `z80asm/assembler.go` | `Result`, `ListingLine`, `AssembledInstruction` fields: `uint16` -> `int`; `Symbols` map: `map[string]uint16` -> `map[string]int`; `EmitWord` method parameter widened |
| `z80asm/instruction_table.go` | All ~30 `v.(uint16)` type assertions -> `v.(int)`; all JP/CALL/LD encoding uses `a.emitWord(target)` instead of manual byte splitting |
| `z80asm/targets.go` | `MemoryLayout` fields all `int`; `CommonSymbols` map `map[string]int`; Agon `DefaultOrigin` = `0x040000`, `RAMSize` = `0x07E000` |
| `z80asm/instructions.go` | `parseOperandValue` return type; `currentAddr` arithmetic |
| `z80asm/directives.go` | `currentAddr` arithmetic |
| `z80asm/expression.go` | `uint16(val)` casts -> `int(val)` |
| `z80asm/minz_integration.go` | `MinZAssembler.baseAddress`, `NewMinZAssembler`, `AssembleInlineBlock`, `AssembledFunction` fields, `FixupSMCReferences` |
| `z80asm/local_labels.go` | `processLabelForContext` addr parameter |
| `z80asm/instruction_sets.go` | RST `validVectors` and IM `opcodeMap` maps |
| `z80asm/pattern_matcher.go` | Removed unnecessary `int()` casts |
| `z80asm/assembler_test.go` | `expectedSymbols` map type |
| `codegen/z80.go` | Agon ORG changed from `$0000` to `$040000` |

### Verification

- Z80 mode: 2-byte addresses, CP/M programs run correctly at `$0100`
- ADL mode: 3-byte addresses, Agon programs assemble at `$040000`
- No regressions in ZX Spectrum examples

---

## Phase 2: Loop Rerolling Bug Fix

### Problem

The loop reroller optimization (`optimizer/loop_reroll.go`) converts repeated `putchar('X')` calls into a single `print_string` call referencing a string constant. However, the `OpPrintString` handler in `z80.go` always called `g.loadToHL(inst.Src1)`, and the reroller never sets `Src1` (leaving it as register 0). Register 0 resolved to memory address `$F000` (a virtual register), generating:

```asm
LD HL, ($F000)    ; WRONG: loads from address $F000
CALL print_string
```

Instead of:

```asm
LD HL, _str_reroll_0    ; CORRECT: loads string address directly
CALL print_string
```

This caused all CP/M programs using function-call `putchar()` patterns to print nothing.

### Fix

Modified `OpPrintString` handler in `z80.go` (line ~3036): when `inst.Symbol` is set (as it always is from the reroller), emit `LD HL, <label>` directly instead of going through `loadToHL()`.

```go
// Before:
g.loadToHL(inst.Src1)

// After:
if inst.Symbol != "" {
    g.emit("    LD HL, %s", inst.Symbol)
} else {
    g.loadToHL(inst.Src1)
}
```

---

## Phase 3: Label Sanitization

### Problem

Function labels in Z80 assembly must not contain dots (`.`) or dollar signs (`$`) — these are either namespace separators or hex prefixes in most assemblers. The `sanitizeFunctionName()` function correctly replaces these, but two code paths bypassed it:

1. `generateTrueSMCFunction()` emitted `fn.Name` directly as the label
2. `generateSMCRecursiveCall()` used `fn.Name` in parameter label construction

This produced labels like `stdlib.cpm.bdos.conio$u8:` instead of `stdlib_cpm_bdos_conio_u8:`.

### Fix

Both paths now route through `sanitizeFunctionName()` before emitting labels.

---

## Phase 4: LF-to-CR Newline Conversion

### Problem

Two places in `z80.go` unconditionally converted LF (10) to CR (13):

1. String data emission (line ~602) — affects rerolled strings
2. `OpPrintStringDirect` character loop (line ~3074) — affects inline string output

This is correct for ZX Spectrum (which uses CR for newlines) but wrong for CP/M (which uses CR+LF) and other platforms.

### Fix

Gated both conversions with `g.targetPlatform == "zxspectrum"`:

```go
// Before:
if ch == 10 { ch = 13 }

// After:
if ch == 10 && g.targetPlatform == "zxspectrum" { ch = 13 }
```

---

## Phase 5: Agon Memory Layout

### Problem

Agon `RAMSize` was `0xFFFF` (64K), a leftover from when all fields were `uint16`. With code starting at `$040000`, any program larger than ~64K bytes would fail validation with "code exceeds available RAM".

### Fix

Set `RAMSize` to `0x07E000` (~504K), matching the actual user memory range `$040000`-`$0BDFFF`. Also added "agon" to MZA's target list help text.

---

## Test Results

### CP/M Programs (via mze emulator)

| Program | Method | Output | Status |
|---------|--------|--------|--------|
| `examples/cpm_hello.minz` | Inline asm | `Hi!\r\nPress key...\r\n` | PASS |
| `examples/cpm_hello_calls.minz` | Function calls | `Hello!\r\nCP/M` | PASS |
| `examples/cpm/hello_cpm.minz` | BDOS stdlib import | `Hello, CP/M!\r\n` | PASS |

### Agon Programs (assembly only — no eZ80 emulator)

| Program | Size | Status |
|---------|------|--------|
| `examples/agon/hello_world.minz` | 2,040 bytes | PASS |
| `examples/agon/graphics_demo.minz` | 5,210 bytes | PASS |
| `examples/agon/vivid_vibes.minz` | 44,656 bytes | PASS |

### ZX Spectrum Regression

| Program | Status |
|---------|--------|
| `examples/fibonacci.minz` | PASS (runs in mze) |

---

## Known Remaining Issues

| Issue | Severity | Description |
|-------|----------|-------------|
| `newline()` inlining bug | Medium | When `newline()` (which calls `putchar(13); putchar(10)`) is inlined, the `LD A, <value>` argument loads are missing before the inlined asm bodies. Workaround: use explicit `putchar(13); putchar(10)`. |
| `$` in SMC anchor labels | Low | PATCH_TABLE entries have `$imm0` suffixes in labels. MZA handles them, but not all assemblers would. |
| `@extern(addr)` syntax silently dropped | Medium | Participle parser expects `@[extern(addr)]` but mos.minz uses `@extern(addr)`. Current workaround: use `extern fun ... at addr` syntax. |
| fseek u32 warning | Low | `fseek(u8, u32)` skipped because u32 type not in symbol table. Pre-existing. |
| MOS header/RST.LIL | Done | Fixed in this session — JP target correct, RST.LIL `$5B` prefix emitted. |

---

## Architecture Decisions Made

See:
- [ADR-0006](../docs/adr/0006-ez80-adl-address-widening.md) — eZ80 ADL Address Widening Strategy
- [ADR-0007](../docs/adr/0007-platform-specific-newline-handling.md) — Platform-Specific Newline Handling
