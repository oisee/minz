# Fix `asm fun` Parsing + VDP Buffer Stdlib + Vivid Vibes Demo

**Date**: 2026-02-12
**Status**: Complete
**Scope**: Parser, Semantic Analyzer, Stdlib (Agon), Examples

---

## Summary

Fixed the core blocker preventing Agon Light 2 stdlib from working: `pub asm fun` bodies containing raw assembly instructions and `;` comments could not be parsed by the Participle parser. Added VDP buffer command functions to the stdlib. Wrote a complete Vivid Vibes demo showcasing LZSS decompression, VDP buffered rendering, and 60fps playback.

---

## Phase 1: Fix `asm fun` Parsing (BLOCKER)

### Problem

`pub asm fun foo() { ... }` uses `Body *StmtBlock` in the parser grammar, which expects MinZ statements. Assembly code (`;` comments, `RST 0x08`, `LD C, A`) cannot parse as valid MinZ, causing all 12+ `asm fun` functions in `stdlib/agon/mos.minz` to silently fail.

### Solution: Source Preprocessing

Added `preprocessAsmFunctions()` in `minzc/pkg/parser/parser.go` that transforms:

```
// Before:
pub asm fun fopen(filename: *u8, mode: u8) -> u8 {
    ; filename in HL, mode in A
    LD C, A
    RST 0x00
    RET
}

// After:
pub asm fun fopen(filename: *u8, mode: u8) -> u8 {
    asm { ; filename in HL, mode in A
    LD C, A
    RST 0x00
    RET
    }
}
```

The `AsmBlock` lexer token (`asm\s*\{([^{}]|\{[^}]*\})*\}`) captures everything between `asm {` and `}` as raw text — including `;` comments and assembly instructions.

### Files Modified

- **`minzc/pkg/parser/parser.go`** — Added `preprocessAsmFunctions()`, `isIdentChar()`, `findAsmFunBodyBrace()`, `findMatchingBrace()`. Updated `ParseFile` and `ParseString` to preprocess before parsing.
- **`minzc/pkg/semantic/analyzer.go`** — Added `FunctionKindAsm` handling in `analyzeFunctionDecl()`: walks body for `AsmStmt` nodes, emits `ir.OpAsm` for each, skips normal body analysis.
- **`minzc/pkg/ir/ir.go`** — Added `IsAsm bool` field to `Function` struct.

### Module Loading Fixes (discovered during testing)

Several fixes to `processLoadedModule()` in `analyzer.go`:

| Fix | Why |
|-----|-----|
| Process module's own imports (Pass -1) | `vdp.minz` imports `mos.minz` — those symbols must be available during vdp's function analysis |
| Register struct names before function sigs (Pass 0) | `get_rtc(time: *RTCTime)` needs `RTCTime` to resolve during signature registration |
| Register ALL function signatures (not just exported) | Internal helpers like `vdu()` are called by exported functions |
| Register bare (unqualified) names | Examples call `mos_puts()` not `stdlib.agon.mos.mos_puts()` |
| Nil body guard on `fn.Body` | Declaration-only functions (extern with `;`) have nil bodies |

---

## Phase 2: VDP Buffer Commands

### File: `stdlib/agon/vdp.minz`

Added 6 new functions for VDP PSRAM buffered drawing:

| Function | VDU Sequence | Purpose |
|----------|-------------|---------|
| `vdu_buffer_write(id, data, len)` | `23, 0, 0xA0, id, 0, len, <data>` | Upload VDU commands to buffer |
| `vdu_buffer_call(id)` | `23, 0, 0xA0, id, 1` | Execute buffered commands |
| `vdu_buffer_clear_all()` | `23, 0, 0xA0, 0xFFFF, 2` | Clear all buffers |
| `vdu_swap()` | `23, 0, 0xC3` | Swap double buffers |
| `vdu_pixel_mode()` | `23, 0, 0xC0, 0` | Set pixel coordinate mode |
| `set_mode_buffered(mode)` | `22, mode+128` | Screen mode with double buffering |

---

## Phase 3: Vivid Vibes Demo

### File: `examples/agon/vivid_vibes.minz`

Complete demo structure:

1. **LZSS Decompressor** (~30 lines) — Flag byte + 8 tokens, 12-bit offset + 4-bit length matches
2. **Columnar Frame Data Parser** — Reads header (num_frames, max_tris), computes column offsets for 10 data columns (tri_counts, colors, x1_lo, x1_hi, y1, x2_lo, x2_hi, y2, x3_lo, x3_hi, y3)
3. **Frame Upload** — Builds VDU command sequence per frame (CLG + GCOL + MOVE + MOVE + PLOT85 per triangle), uploads via `vdu_buffer_write`
4. **Playback Loop** — `vdu_buffer_call(id)` + `vdu_swap()` + `wait_vblank()` = 9 bytes/frame over UART
5. **Main** — Loads compressed data from SD card, decompresses, uploads all frames, plays animation loop

### Workarounds for Current MinZ Limitations

| Limitation | Workaround |
|-----------|-----------|
| Array-to-pointer cast (`&arr as *u8`) | Use `&arr[0]` (address of first element) |
| Struct literal returns | Assign fields individually via globals |
| Pointer arithmetic (`ptr + offset`) | Use array indexing with offset |

---

## Phase 4: Tests

### New Test File: `minzc/pkg/parser/parser_test.go`

| Test | Description |
|------|-------------|
| `TestPreprocessAsmFunctions` | 6 subtests: simple asm fun, semicolon comments, regular fun unchanged, asm block unchanged, multiple asm funs, no false positive on "assembly" |
| `TestPreprocessPreservesRegularCode` | Verifies no modification of code without `asm fun` |
| `TestAsmFunParsing` | End-to-end: parse `asm fun`, verify FunctionKindAsm, check AsmStmt in body |
| `TestAsmFunWithSemicolonComments` | Verifies `;` assembly comments survive preprocessing |
| `TestMultipleAsmFuns` | Mixed asm and regular functions parse correctly |

---

## Verification Results

### Tests
| Suite | Result |
|-------|--------|
| Parser tests (5 new) | PASS |
| Agon tests (5 existing) | PASS |
| z80asm assembler tests | Pre-existing failures (unrelated) |
| Corpus test | Pre-existing failure (missing directory) |

### Compilation
| File | Result |
|------|--------|
| `examples/agon/hello_world.minz` | Compiles successfully |
| `examples/agon/graphics_demo.minz` | Compiles successfully |
| `examples/agon/vivid_vibes.minz` | Compiles successfully |
| `go build ./cmd/... ./pkg/...` | Clean build, no errors |

### Known Remaining Warnings
- `SysVars`/`RTCTime` struct cast failures — pre-existing issue where module-prefixed struct names (`stdlib.agon.mos.SysVars`) don't match unqualified cast targets (`SysVars`). Non-blocking for the functions used by these examples.
- `get_rtc`/`set_rtc` mangled name mismatch — `p_RTCTime` vs `p_stdlib.agon.mos.RTCTime`. Same root cause as above.
- `fseek` uses `u32` parameter type which may not be fully registered.

---

## TODO: Issues Discovered

These should be tracked for future work:

- [ ] Struct type prefix in casts: `*u8 as *SysVars` fails when SysVars is registered as `stdlib.agon.mos.SysVars`
- [ ] `u32` type support in module parameter resolution
- [ ] `@incbin` in global array initializer syntax
- [ ] Pointer arithmetic (`ptr + offset`) as first-class syntax
- [ ] Large array (>64KB) support verification for eZ80/ADL mode
- [ ] VDP audio API functions (for bytebeat audio stretch goal)
