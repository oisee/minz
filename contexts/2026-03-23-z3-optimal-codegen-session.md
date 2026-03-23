# Session: Z3 Optimal Codegen + Frill Frontend + ADR-0039

**Date:** 2026-03-22 — 2026-03-23
**Branch:** `feat/z3-optimal-codegen`
**Commits:** 34
**LOC:** +7215 across 49 files
**Corpus:** 68/79 (86%) → 83/87 (95.4%)

---

## What Was Done

### Z3 SMT Solver Integration
- `z3solve.go` — SMT encoding of regalloc, Z3 subprocess, model parsing
- `z3_joint.go` — joint isel+regalloc via e-graph + Z3
- `z3validate.go` — post-WFC consistency check + Z3 repair
- `--z3` CLI flag, PBQP hints as soft constraints
- `saturate.go` — equality saturation framework with Z80 rewrite rules

### 15+ Codegen Bug Fixes
- addr_of symbol propagation (LD HL,0 → LD HL,symbol) — CRITICAL
- fixInvalidZ80Template guard (ADD HL,HL not a store!) — CRITICAL
- INC/DEC vs double disambiguation
- 16-bit cond_ret (EX DE,HL instead of LD A,DE)
- 16-bit CMP pattern (SBC HL,DE + ADD HL,DE)
- CmpSubCarry elision (carry flag from preceding SUB)
- OpNeg bridge (NEG instruction was skipped!)
- Label sanitization ($, @, ., Z80 register names)
- String pool emission in PBQP fallback path
- dedupAsmLabels for hybrid LIR+PBQP output
- Return-value spill tracking (retVRegs from terminators)
- Safe tail-call optimization (only when return val = call result)
- cond_ret Meta survives WFC (Meta field in WFCCell)
- Spill reload for L2-L7 (IXH, TSMC, memory)

### Frill Frontend
- ML-inspired functional language (.frl)
- Cherry-picked master's complete Frill (pattern matching, ADTs, currying, do notation)
- 8 examples all passing
- stdlib/frill/ from master

### Architecture
- MIR1 archived to pkg/_archive/mir1_deprecated
- ADR-0037: target-neutral optimization split + cost tiers
- ADR-0038: TSMC spill tiers (from neighbor)
- ADR-0039: Unified VIR Solver — joint isel+regalloc in one pass
  - Plan A: clean-room (3-4 weeks)
  - Plan B: iterative (3 weeks, RECOMMENDED)
  - Both branches created: feat/z3-optimal-codegen (B), feat/unified-vir-solver (A)
- Terminology: HIR → MIR → VIR → PIR → ASM

### Outparam Detection
- `outparam_promote.go` — detect write-only pointer params → tuple return
- For paper: get_minmax(buf, n, *min, *max) → (min, max) = get_minmax(buf, n)

### Philipp Reply Draft
- `docs/philipp-reply-draft.md` — addresses all 6 feedback points from SDCC maintainer

---

## Current State

### Corpus: 83/87 (95.4%)

Remaining 4 failures:
| File | Issue | Category |
|------|-------|----------|
| nc.nanz | PBQP _spill_ label never emitted (pre-existing PBQP bug) | Not LIR |
| assert_test.c | clamp8: multi-cond_ret + br_if interleaving | LIR — in progress |
| import_test.c | MIR2 VM: #import cross-file resolution | C89 frontend |
| struct_promote.c | MIR2 VM: outparam codegen mismatch | MIR2 codegen |

### ASM Quality — Perfect Leaf Functions
```z80
double16:  ADD HL, HL / RET          ; was 6 instructions!
add:       ADD A, C / RET            ; optimal
main:      LD A, 7 / RET             ; constant folded
pipe:      LD A, 11 / RET            ; pipe chain folded
abs_diff:  SUB C / JR C,.cr / RET / .cr: NEG / RET  ; correct with NEG
```

### Branches
- `feat/z3-optimal-codegen` — Plan B (iterative), production, 83/87
- `feat/unified-vir-solver` — Plan A (clean-room), created, not started

---

## Next Steps (Plan B)

1. **B1: Fix clamp8** — multi-cond_ret with br_if (interleaved blocks)
2. **B1: Remove fixInvalidZ80Template** — move rules to pattern constraints
3. **B2: Remove spill_reload.go** — move to WFC LocSet constraints
4. **B3: Remove validate-reject** — constraints sufficient
5. **B5: Merge isel into WFC** — unified solver
6. **Philipp reply** — send with verified paper examples
7. **Paper revision** — update SDCC 4.5.0, fix terminology, add 6502

---

## Key Insights

1. **Root cause of all codegen bugs:** 5 sequential phases losing information at boundaries
2. **Unified solver (ADR-0039)** eliminates phase boundaries — one pass for isel+regalloc
3. **Z3 value:** not performance (WFC is already optimal for simple cases) but correctness oracle + consistency checker
4. **TSMC spill (ADR-0038):** 20T cheaper than stack (21T), 6T cheaper than memory (26T)
5. **VIR level is the sweet spot** for solver: Z80-flavored but still virtual registers
