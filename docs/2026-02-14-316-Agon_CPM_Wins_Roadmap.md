# Agon & CP/M: Quick / Mid / Long Wins Roadmap

**Date**: 2026-02-14
**Context**: After completing eZ80 address widening and CP/M codegen fixes

---

## Quick Wins (1-2 hours each)

These are bugs or gaps that have known fixes and can be done in a single session.

### QW-1: Fix `newline()` inlining argument loading
**Effort**: ~1 hour | **Impact**: Medium | **Files**: `codegen/z80.go`

When `newline()` from `bdos.minz` is inlined into the caller, the `putchar(13)` and `putchar(10)` calls inside it get their asm bodies inlined but the `LD A, <value>` argument loads are missing. The semantic analyzer or inliner drops the `OpLoadConst` when flattening nested asm function calls.

**Fix approach**: In the asm function inliner, ensure argument setup instructions are emitted before the inlined asm body. Specifically, when a function calls an asm function with a constant argument, emit `LD A, <const>` (for u8) or `LD HL, <const>` (for u16) before the inlined asm block.

### QW-2: Sanitize `$imm0` in SMC anchor labels
**Effort**: ~30 min | **Impact**: Low | **Files**: `codegen/z80.go`

PATCH_TABLE entries reference labels like `stdlib_cpm_bdos_conio_u8_param_param$imm0`. The `$` is from the anchor naming convention. MZA handles it, but it's not clean assembly.

**Fix**: Change `$imm0` / `$immOP` suffixes to `_imm0` / `_immOP` in `generateParameterAnchor()`.

### ~~QW-3: Add `@extern(addr)` preprocessor~~ REJECTED
Not needed — `extern fun ... at addr` is the proper syntax and already works. The `@extern` decorator style is unnecessary.

### ~~QW-4: CP/M target auto-sets load address in mze~~ DONE
Already implemented in `cmd/mze/main.go` lines 49-51. Auto-sets `load=0x0100` when target is `cpm`.

### ~~QW-5: Fix `→` Unicode in generated assembly comments~~ NOT AN ISSUE
No Unicode arrows found in `codegen/z80.go`. Was a display artifact.

---

## Mid Wins (half day to 1-2 days each)

These require deeper investigation or touch multiple subsystems.

### MW-1: Fix `@mode("adl")` attribute preprocessing
**Effort**: ~4 hours | **Impact**: High | **Files**: `parser/parser.go`, `semantic/analyzer.go`

The `@mode("adl")` attribute on asm functions in `mos.minz` is silently dropped, just like `@extern`. Need a `preprocessModeAttributes()` to convert `@mode("adl")\npub asm fun foo() { ... }` to `pub asm fun foo() mode "adl" { ... }`.

More complex than `@extern` because the inline `mode` syntax may need parser grammar support if not already present. Currently worked around by manual conversion.

### MW-2: Agon binary testing in Fab Agon Emulator
**Effort**: ~1 day | **Impact**: High | **Files**: CI/testing scripts

The 3 Agon examples assemble but have only been tested structurally (correct addresses, ADL encoding). Need to:
1. Set up Fab Agon Emulator (or real hardware) testing
2. Copy `hellomz.bin`, `gfxdemo.bin`, `vibes.bin` to emulator SD card
3. Verify actual execution: text output, graphics, timing
4. Document the testing workflow

### MW-3: Complete CP/M BDOS stdlib testing
**Effort**: ~4 hours | **Impact**: Medium | **Files**: tests, examples

Currently only tested `putchar`, `getchar`, and `newline` (via print_string). Need mze-based tests for:
- `file_open` / `file_close` / `file_read` / `file_write` (requires mze FCB support)
- `get_drive` / `select_drive`
- `print_string` (BDOS function 9, `$`-terminated)
- `get_version`

### MW-4: Proper eZ80 instruction encoding validation
**Effort**: ~1 day | **Impact**: High | **Files**: `z80asm/instruction_table.go`, tests

The current ADL mode support emits 3-byte addresses via `emitWord()` but doesn't validate eZ80-specific instruction encodings:
- `.LIL` / `.SIS` / `.LIS` / `.SIL` suffixes for mixed-mode calls
- `RST.LIL` prefix byte (`$5B`) handling is ad-hoc
- No test coverage for ADL-mode assembled output

### MW-5: Virtual register addresses for Agon
**Effort**: ~4 hours | **Impact**: Medium | **Files**: `codegen/z80.go`

Virtual registers start at `$F000` (Z80 high memory). On Agon with `$040000` base, `$F000` is in MOS firmware space, not user RAM. Need to relocate virtual register base to Agon-safe address (e.g., `$04F000` or dynamic based on code end).

---

## Long Wins (multi-day to week+)

Strategic improvements that establish proper multi-platform infrastructure.

### LW-1: Platform abstraction layer in codegen
**Effort**: ~1 week | **Impact**: Very High | **Files**: `codegen/z80.go` (refactor)

The Z80 codegen currently has platform-specific code scattered throughout via `switch g.targetPlatform` blocks (for character output, newline handling, system calls, etc.). This grows linearly with each new platform.

**Proposal**: Extract a `PlatformCodegen` interface:
```go
type PlatformCodegen interface {
    EmitPutchar(reg string)      // Emit char output (RST 16, BDOS 2, RST $10, etc.)
    EmitNewline()                // Platform-appropriate newline
    EmitSystemCall(fn, param)    // Platform system call convention
    StringNeedsLFtoCR() bool     // Newline conversion flag
    PrintStringHelper() string   // Runtime helper name
}
```

Each platform implements the interface. The main codegen calls interface methods instead of switch blocks.

### LW-2: Unified attribute system (`@extern`, `@mode`, `@inline`, etc.)
**Effort**: ~1 week | **Impact**: Very High | **Files**: parser, semantic analyzer

Currently, attributes like `@extern(addr)` and `@mode("adl")` are handled via source preprocessing hacks. Long-term, the parser grammar should support a proper attribute syntax that the semantic analyzer interprets.

**Options**:
- A: `@[name(args)]` bracket syntax (Participle already partially supports)
- B: `@name(args)` direct decorator syntax (requires grammar extension)
- C: Preprocessor transforms (current approach, fragile)

Should be resolved alongside the hand-written parser migration (already planned).

### LW-3: eZ80 emulator or Agon MOS emulator in mze
**Effort**: ~2 weeks | **Impact**: Very High | **Files**: `cmd/mze/`, new package

Currently mze only emulates Z80. For Agon testing, need:
- eZ80 instruction support (24-bit addressing, ADL mode, `RST.LIL`)
- MOS API emulation (at minimum: `mos_puts`, `mos_putchar`, `mos_sysvars`)
- VDP stub (acknowledge VDU commands, maybe capture output)

This would enable automated CI testing of Agon binaries without hardware.

### LW-4: Cross-platform test matrix
**Effort**: ~1 week | **Impact**: High | **Files**: CI, test infrastructure

Establish a test matrix that compiles each example for every supported target and validates the output:

| Example | ZX Spectrum | CP/M | Agon | MSX |
|---------|-------------|------|------|-----|
| hello_world | mze | mze | fab-agon-emu | openmsx |
| fibonacci | mze | mze | - | - |
| graphics_demo | mze | - | fab-agon-emu | - |

Each cell tracks: compiles, assembles, runs, correct output.

### LW-5: Stdlib module dependency resolution
**Effort**: ~3 days | **Impact**: High | **Files**: `semantic/analyzer.go`

Currently, module loading order in `processLoadedModule()` is fragile — struct names must register before function signatures, imports must process before the module's own functions, etc. A dependency graph resolver would:
1. Parse all modules to extract dependencies
2. Topological-sort the load order
3. Process in correct order automatically
4. Detect circular dependencies with clear error messages

---

## Priority Matrix

| Win | Effort | Impact | Priority |
|-----|--------|--------|----------|
| ~~QW-4: mze auto-load CP/M~~ | — | — | DONE |
| ~~QW-2: Sanitize $ in assembly comments~~ | — | — | DONE (sanitizeComment) |
| ~~QW-5: Fix Unicode in comments~~ | 15 min | Low | DONE (→ to ->) |
| QW-1: Fix newline() inlining | 1 hour | Medium | Do soon |
| ~~QW-3: @extern preprocessor~~ | — | — | REJECTED |
| **QW-6: .a80 assembly pass-through** | 30 min | Medium | **DONE** |
| **QW-7: Silent error warnings** | 1 hour | High | **DONE** |
| MW-2: Agon emulator testing | 1 day | High | Next sprint |
| MW-4: eZ80 instruction validation | 1 day | High | Next sprint |
| ~~MW-5: Virtual register relocation~~ | — | — | Needs 24-bit codegen (deferred to LW-3) |
| MW-1: @mode preprocessing | 4 hours | High | Next sprint |
| MW-3: CP/M BDOS test suite | 4 hours | Medium | Backlog |
| LW-1: Platform abstraction | 1 week | Very High | Plan for v0.19 |
| LW-2: Unified attributes | 1 week | Very High | Align with parser rewrite |
| LW-5: Module dependency graph | 3 days | High | Plan for v0.19 |
| LW-3: eZ80 emulator in mze | 2 weeks | Very High | Plan for v0.20 |
| LW-4: Cross-platform test matrix | 1 week | High | Plan for v0.20 |
