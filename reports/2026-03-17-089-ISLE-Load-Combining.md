# 2026-03-17-089: ISLE Load Combining

**Date:** 2026-03-17
**Branch:** feat/lir-backend
**Status:** Complete (gap #1 of 3)

---

## Context

A colleague comparing MinZ C89->MIR2->C output vs SDCC Z80 output for FatFS identified 3 key codegen gaps:

1. **ld_word (16-bit LE load)**: SDCC fuses shift+or into LD pair; MinZ emits verbose SSA chain
2. **check_fs (switch/case)**: SDCC generates computed jump table; MinZ does if/else chain
3. **get_chain_length (do-while)**: SDCC generates tight JR NZ loop; MinZ does while(true)+break

This report covers gap #1 — load combining via ISLE rewrite rules.

---

## What Was Built

### combine.go — ISLE-Based Pre-Isel Instruction Combining

- Converts MIROp DAG into ISLE term trees, applies rewrite rules, extracts combined MIROps
- Dead code elimination for consumed intermediate ops
- FatFS `ld_word` pattern: 8 MIROps (2x load8 + const + add + shl + or) reduced to 2 MIROps (const + load16_le)

### New Opcodes

- `OpShl`, `OpShr`, `OpLoad16LE` with full VM support

### Machine Patterns (4 targets)

| Target | Pattern |
|--------|---------|
| RISC32 | Generic load16_le |
| RISC8 | Generic load16_le |
| CISC (Z80-like) | `LD L,(HL); INC HL; LD H,(HL)` — real Z80 sequence |
| MICRO (6502-like) | Indirect indexed load via zero-page pointer |

### RewriteFixpoint(fuel)

Added to ISLE rewriter for termination safety — prevents infinite rule application.

### bridge.go + bridge_test.go

MIR2->LIR bridge (from previous session, now committed on this branch).

---

## ISLE Combining Rules

The key innovation — pattern matching on the SSA DAG expressed as S-expressions:

```lisp
;; FatFS ld_word: or(shl(load8(base+1), 8), load8(base)) -> load16_le(base)
(rule 20 (or (shl (load8 (add ?base (const 1))) (const 8))
             (load8 ?base))
         (load16_le ?base))
```

This single rule captures the entire 8-op -> 2-op transformation that SDCC performs in its monolithic codegen.

---

## Pipeline Position

```
MIR2 -> bridge.go -> MIROps -> combine.go (ISLE) -> isel -> WFC -> VM
```

The combining pass sits between the MIR2 bridge and instruction selection, operating on the MIROp DAG before target-specific lowering.

---

## Test Results

5 new tests, all passing:

| Test | What it verifies |
|------|-----------------|
| TestCombine_Load16LE | 8 ops reduced to 2 ops |
| TestCombine_Load16LE_Convergence | 3-way convergence (RISC32, RISC8, CISC) |
| TestCombine_Load16LE_FullPipeline | Raw SSA -> ISLE -> isel -> WFC -> VM = 0xBEEF |
| TestCombine_IdentityRules | add(42,0) -> 42 |
| TestCombine_ISLE_RuleParsing | 6 rules parsed correctly |

Full LIR suite: 30+ tests, all green. 4-way convergence maintained across RISC32, RISC8, CISC, MICRO.

---

## What's Next (Gaps #2 and #3)

- **Switch densifier** (gap #2): Recognize dense integer cases and generate jump table. ISLE + Datalog density analysis.
- **Loop canonicalizer** (gap #3): Transform while(true)+break into do-while, then lower to DJNZ/JR NZ on Z80.
- Both are pre-MIR2->LIR passes, feeding into the same ISLE+WFC pipeline.

---

## Key Insight

SDCC does all this in one monolithic codegen. We have each step as a separate, verifiable pass. The colleague with FatFS can compare output at every level, and 4-way convergence catches regressions automatically. The ISLE rule DSL makes each optimization explicit, auditable, and independently testable — rather than buried in procedural codegen logic.
