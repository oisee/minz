# Report #083 — Z80 Codegen Hardening: Div/Mod + Caller-Save Liveness

**Date:** 2026-03-15
**Branch:** master (merged from fix/z80-width-mismatch)
**Commits:** 2e69822, 9106e99, 2fdd539, 145c514

---

## Summary

Two sessions of focused MIR2→Z80 codegen work: new opcodes, bug fixes,
and a major perf optimization. Combined with the neighbor's C89 frontend
work, the compiler now supports 6 frontends with 191 corpus asserts.

---

## 1. MZV2 — MIR2 VM Runner (145c514)

Built `mzv2` — TUI display for ZX Spectrum programs running on MIR2 VM.
Tetris runs correctly, proving MIR2 IR is correct and all open bugs are
in Z80 codegen only.

- Host function overrides: zx_poke/peek/key_row/halt/border
- ZX ROM font OCR: matches pixel data against 96 glyphs
- ANSI 32×24 color renderer with ink/paper from attribute memory
- Flags: `--headless`, `--max-frames`, `--trace`

**Files:** `minzc/cmd/mzv2/main.go` (250 LOC), `minzc/cmd/mzv2/font.go` (120 LOC)

## 2. Bug Fixes (2fdd539)

| Fix | Before | After |
|-----|--------|-------|
| BUG-A: emitMov 8↔16 width | `LD A, HL` (invalid) | `LD A, L` |
| BUG-B: ADD HL, r8 | `ADD HL, A` (invalid) | `LD B,0; ADD HL,BC` |
| DeadBlockArgElim | sum_to: 30B trampoline | 22B (−8B) |
| genCmp16 no-swap | max: 13B, 2× EX DE,HL | 11B (−2B, −16T) |

## 3. OpDiv/OpMod Z80 Codegen (9106e99)

Implemented the last 3 unimplemented opcodes:

**8-bit (OpDiv/OpMod u8):** Dark/X-Trade shift-and-subtract (DIVU111,
Spectrum Expert #01, 1997). 236–244 T-states, O(1). Algorithm from
`~/dev/antique-toy/listings/ch04_method_1_shift_and_subtract.z80`.

```z80
; B=dividend, C=divisor → B=quotient, A=remainder
    XOR A
    LD D, 8
.loop:
    SLA B           ; shift dividend MSB → CF
    RLA             ; CF → accumulator
    CP C            ; trial subtract
    JR C, .skip
    SUB C           ; accept
    INC B           ; set quotient bit (free after SLA)
.skip:
    DEC D
    JR NZ, .loop
```

**16-bit (OpDiv/OpMod u16):** Restoring long division, 16 iterations,
~1000 T-states. HL/DE → HL=quotient, BC=remainder.

**Nanz `%` operator:** Added `case "%": bld.Mod()` in HIR lowerer.

**E2E verified:** 24 test cases (div8×8, div16×8, mod8×8, combined×6).

## 4. Caller-Save Liveness Optimization (2e69822)

The single biggest codegen improvement this session.

**Problem:** `callerSavePairs()` saved ALL allocated registers around
every CALL, even if the register was dead after the call.

**Fix:** Added `regsLiveAfterInst()` — scans forward from call site to
find which virtual registers are actually used by subsequent instructions
or the block terminator. Only live registers are saved.

**Results on Tetris:**

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| PUSH instructions | 105 | 39 | **−63%** |
| POP instructions | 101 | 39 | **−61%** |
| Total instructions | 2076 | 1879 | **−9.5%** |
| Assembly lines | 2464 | 2245 | **−8.9%** |

Savings: ~132 bytes, ~1320 T-states per frame (at 10T per PUSH/POP pair).

## 5. C89 Frontend (neighbor, 53c9672 + 151beae + 63460ea)

| Feature | Status |
|---------|--------|
| &&/\|\| short-circuit | ✅ CondExpr desugaring |
| switch/case | ✅ if-chain lowering |
| % modulo | ✅ native OpMod (was: a-(a/b)*b decompose) |
| enum constants | ✅ |
| C99 types (uint8_t etc) | ✅ |
| /= %= compound ops | ✅ |
| Corpus | 191/191 asserts, 14 files |

## 6. Test Coverage

| Suite | Count | Status |
|-------|-------|--------|
| E2E Z80 (MIR2 pkg) | 24 | ✅ all pass |
| C89 corpus asserts | 191 | ✅ all pass |
| Go test packages | 26/26 | ✅ all pass |
| MIR2 VM (mzv2 tetris) | 100 frames | ✅ correct |

## 7. Next Targets

| Optimization | Expected Impact | Effort |
|---|---|---|
| Loop head double-check elimination | −4 inst per DJNZ loop | Medium |
| PUSH HL;POP BC → LD B,H;LD C,L | −12T per occurrence | Easy |
| IXH/IXL avoidance in hot functions | −4-15T per IX instruction | Hard (PBQP) |
| Post-RA dead move elimination | −2-5% code size | Medium |
| Edge-cost driven contracts (ADR-0020) | systemic ABI improvement | Large |
