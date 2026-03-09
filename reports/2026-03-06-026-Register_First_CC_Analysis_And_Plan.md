# Register-First Calling Convention: Analysis & Implementation Plan

**Date:** 2026-03-06
**Status:** Plan — ready to implement
**Context:** docs/_in/claude-deep-01.md, claude-deep-01-b.md, Krause arXiv 2112.01397

---

## Executive Summary

MinZ already has a "register" calling convention implemented (caller + callee codegen both exist).
The default is SMC. Changing the default is a 2-line change in `ir/ir.go` + a few lines in `analyzer.go`.

The full interprocedural optimization (PBQP, FunctionContract struct, IXH/IXL support) is a
multi-week project. But **step 1 alone likely delivers 10-30% improvement**, similar to SDCC's
__sdcccall(1) gain (3-22% on Whetstone/Dhrystone/Coremark/stdcbench).

---

## What Already Exists (discovered by code inspection)

### ✅ "register" convention is already implemented

**Callee side** (`z80.go:1794` — `loadParametersFromRegisters()`):
```
u8 param 0  → A   (storeFromA)
u8 param 1  → E   (LD A,E)
u8 param 2  → D   (LD A,D)
u16 param 0 → HL  (storeFromHL, already there)
u16 param 1 → DE  (EX DE,HL)
u16 param 2 → stack (POP HL)
```

**Caller side** (`z80.go:1662–1697`): already places args into A/HL/DE/stack before CALL.

**Guard**: `if fn.IsRecursive || fn.IsSMCEnabled || len(fn.Params) > 3 { return }` — so
when IsSMCEnabled=true (current default), this path is never reached.

### ✅ HL stale tracking already fixed at OpLabel

`z80.go:1908`: `g.currentRegister = 0` at every `ir.OpLabel`. The ADR-0006 mentioned
in CLAUDE.md refers to the *MIR backend* bugs (tracked separately), not an unfixed codegen bug.

### ✅ ADR-0008: flag returns already designed

`docs/adr/0008-flag-based-boolean-abi-for-iterators.md` (Status: Proposed):
- Iterator predicates return CF/ZF instead of HL=0/1
- `OpCallPredicate` + `OpJumpIfFlag` already sketched in the ADR
- `FlagCondition` enum (CY/NC/Z/NZ) already exists in `ir/ir.go:258`

### ✅ Call graph / recursion detection already exists

`optimizer/recursion_detector.go`: full DFS cycle detection, direct/mutual/indirect
recursion. Already used to disable SMC for recursive functions.

### ✅ Shadow registers tracked in IR

`ir.ir.go:32–44`: `Z80_A_SHADOW` through `Z80_HL_SHADOW` in `Z80Register` enum.
`codegen/register_allocator.go:47–57`: `RegA_Shadow` through `RegHL_Shadow` in `PhysicalReg`.
`codegen/z80.go:1836–1856`: `generateInterruptPrologue` already uses `EXX`/`EX AF,AF'`.

### ❌ Missing: IXH/IXL/IYH/IYL in PhysReg and RegHint

`PhysicalReg` enum stops at `RegIY`. No halves. No alias constraints.

### ❌ Missing: FunctionContract struct

Only `CallingConvention string`. No typed parameter location tracking.

### ❌ Missing: Interprocedural analysis pass

No pass that computes optimal register assignments across function boundaries.

---

## The Default Change (Step 1)

### What needs to change

**`ir/ir.go` — `NewFunction()`** (line 778–779):
```go
// BEFORE:
IsSMCDefault: true,
IsSMCEnabled: true,

// AFTER:
IsSMCDefault: false,
IsSMCEnabled: false,
```

**`semantic/analyzer.go`** (line 1677–1681) — default for functions without `@abi`:
```go
// BEFORE:
if irFunc.CallingConvention == "" {
    irFunc.IsSMCDefault = true
    irFunc.SMCParamOffsets = make(map[string]int)
}

// AFTER:
if irFunc.CallingConvention == "" {
    irFunc.CallingConvention = "register"
    irFunc.IsSMCDefault = false
    irFunc.IsSMCEnabled = false
}
```

**`semantic/analyzer.go`** (line 1807–1823) — finalize SMC decision block:
The existing fallback logic (recursive → disable SMC, many locals → disable) already works
correctly. With the new default, this block becomes unreachable for most functions (CC is
already set). Keep it for safety.

**`semantic/analyzer.go`** (line 10351–10354) — lambda IR function init:
```go
// Change lambda/local function defaults too:
CallingConvention: "register",
IsSMCDefault:      false,
IsSMCEnabled:      false,
```

### Current "register" convention mapping vs. Krause optimal

| Param | Current MinZ "register" | Krause __sdcccall(1) optimal |
|-------|------------------------|------------------------------|
| u8 #0 | A | A |
| u8 #1 | E | (not separately benchmarked) |
| u8 #2 | D | (stack) |
| u16 #0 | HL | HL |
| u16 #1 | DE | DE |
| u16 #2 | stack | stack |
| return u8 | A (convention) | A |
| return u16 | HL | DE (Krause found DE better!) |

**Discrepancy**: Krause found returning u16 in **DE** is better than HL, because HL is then
free at the call site for the next operation. This is worth investigating as a follow-up tweak.

---

## MIR Changes Required for Full Plan

### Phase 1: Default switch (this session)

Changes: `ir/ir.go` + `analyzer.go` as above.
Tests: `go test ./pkg/... -vet=off`, all 11 E2E iterators, example compile rate.

### Phase 2: FunctionContract struct (next session)

Add to `ir/ir.go`:
```go
type LocationKind uint8
const (
    LK_PhysReg  LocationKind = iota
    LK_Stack
    LK_Flag      // CF, ZF, SF, PVF
    LK_SMCPatch
)

type ConventionLocation struct {
    Kind        LocationKind
    Reg         PhysicalReg  // if LK_PhysReg
    Flag        FlagCondition // if LK_Flag (reuse existing CY/NC/Z/NZ)
    StackOffset int          // if LK_Stack
    Size        int          // 1 or 2 bytes
}

type FunctionContract struct {
    Params     []ConventionLocation
    Returns    []ConventionLocation
    Clobbers   RegisterSet
    StackBytes int
}
```

Add `Contract *FunctionContract` field to `ir.Function` (nil = use CallingConvention string).

### Phase 3: IXH/IXL/IYH/IYL (eZ80/configurable)

Add to `codegen/register_allocator.go`:
```go
RegIXH PhysicalReg = ...
RegIXL
RegIYH
RegIYL
```

Add alias constraints: IXH+IXL conflict with IX (cannot mix full-word use with halves).
Gate behind `target.SupportsIXHalves` (true for Agon eZ80, optionally for ZX Spectrum).

Add `RegHintIXH/IXL/IYH/IYL` to `ir.RegisterHint` enum.

### Phase 4: Interprocedural analysis (multi-week)

New pass `optimizer/cc_synthesizer.go`:
1. Use `RecursionDetector` to build SCC-collapsed call graph
2. Bottom-up: compute clobber sets from RegHints + callee clobbers
3. Greedy assignment: assign params to free registers at each function
4. Emit `FunctionContract` metadata for each function
5. Codegen uses `Contract` if non-nil, falls back to string-based convention

### Phase 5: ADR-0008 implementation

`ir/ir.go`: Add `OpCallPredicate` opcode (already in ADR design)
`semantic/iterator.go`: Emit `OpCallPredicate` for filter/takeWhile
`codegen/z80.go`: New case for `OpCallPredicate` — emit PUSH HL / CALL / POP HL / JR Cflag

---

## Risk Assessment

| Change | Risk | Mitigation |
|--------|------|-----------|
| Default → "register" | Medium | SMC-dependent code breaks. Full test suite catches. Fallback: `@abi(smc)` annotation |
| Lambda functions default | Low-Medium | Already tested via iterator E2E suite |
| TRUE SMC functions | None | `UsesTrueSMC` flag takes precedence, unaffected |
| @extern functions | None | Have explicit `TargetReg` per param, bypasses convention |
| Recursive functions | None | Already handled: `IsRecursive` disables SMC |
| asm fun bodies | None | `IsAsm` flag, never goes through convention codegen |

**Key test to run after the change:**
```bash
cd minzc && go test ./pkg/... -vet=off -timeout 120s
bash ../examples/e2e_iterators/run_e2e.sh
./mz ../examples/fibonacci_cpm.minz -b z80 --target cpm -o /tmp/fib.a80
./mze /tmp/fib.com -t cpm
```

---

## Expected Impact

- **Parameter passing**: 13T SMC patch → 0T (params already in registers at call site)
- **Binary size**: 3 bytes per SMC patch site eliminated × N call sites
- **Re-entrancy**: Register convention is inherently re-entrant (SMC is not)
- **ROM targets**: Code can now live in ROM (SMC requires RAM)
- **Recursive functions**: Already work (SMC already disabled for them, no change)
- **Iterator loops**: Already optimized via hints; register default helps surrounding code

Compared to Krause's 3-22% SDCC improvement from fixed-convention switch: MinZ
should see similar or better gains because current SMC has additional overhead
(patch memory stores at call sites) beyond what SDCC's stack convention had.

---

## Prior art in docs/_in/

- `claude-deep-01.md`: Full PBQP formulation, two-phase strategy, Krause references
- `claude-deep-01-b.md`: A-register asymmetry (BC/DE for A-only ops), flag return ABI
- `2112.01397v2.pdf`: Krause paper — Z80 experiments show 3-22% from fixed convention switch

**Key insight from claude-deep-01-b.md not yet addressed:**
> For pure accumulator pipelines (A is sole workhorse), HL is NOT the prize pointer register.
> BC and DE equally valid, and freeing HL for arithmetic is strictly better.
>
> Proposed use-analysis classification:
> - POINTER_A_ONLY → BC or DE (frees HL)
> - POINTER_MULTI_REG → HL (mandatory)

This is a genuine MinZ-specific optimization that SDCC/z88dk miss entirely.

---

## Recommended Next Actions

1. **Now**: Apply the default switch (2 places in analyzer.go + NewFunction). Run tests.
2. **Measure**: Compare T-states before/after on `examples/` programs.
3. **Next**: Add `FunctionContract` struct to MIR (non-breaking, nil = string fallback).
4. **Then**: Implement ADR-0008 (flag returns for iterator predicates).
5. **Later**: IXH/IXL addition + interprocedural synthesizer.
