# Context: Overnight Codegen Marathon

**Date:** 2026-03-15 → 2026-03-16
**Host:** macbookairm2 (alice)
**Branch:** master
**Report:** #086
**To continue on another machine:** `git pull && cd minzc && go build -o mz ./cmd/minzc && go build -o mzv ./cmd/mzv`

---

## What Was Done (16+ commits)

### Codegen Improvements (our session)
1. **MZV runner** (`cmd/mzv/`) — MIR2 VM with TUI + ZX font OCR. Tetris works.
2. **OpDiv/OpMod** — Dark/X-Trade DIVU111 (8-bit 236T), restoring long division (16-bit ~1000T)
3. **`%` operator** in Nanz — HIR lowerer `case "%"` → OpMod
4. **Caller-save liveness** — only save regs live after call. PUSH 105→39 (−63%)
5. **ADD HL,rr restore** — signed compare without PUSH/POP. max: 13→12B
6. **SimplifyIdentities** — Sub(0,x)→Neg, Add(x,0)→Move, Mul(x,1)→Move
7. **genMul16 HL save** — PUSH/POP HL when dst≠HL. BUG-014 partial fix.
8. **blockParamRegs** — exclude loop counters from constVals. BUG-014 fix #2.
9. **emitCallArgs actual alloc** — use callee's PBQP allocation, not canonical
10. **reorderAccMoves** — swap move→A with ALU→nonA to prevent A clobber
11. **8-bit ALU A-clobber interference** — extra edges in interference graph
12. **FuseAbsDiff** — `cmp.ugt+sub(b,a)` → `sub(a,b)+CmpSubCarry`. 9→5B.
13. **CondExpr return desugar** — `return cond?X:Y` → two Ret blocks. 14→5B.
14. **SplitJoinRet** — trivial join-blocks → per-predecessor Ret
15. **OpCallIndirect** — Z80 `CALL __call_hl` trampoline + VM dispatch
16. **emit8ALUImm** — AND imm in holdsValue path (AND D→AND 31 fix)

### C89 Frontend (neighbor session)
- Designated initializers, static locals, goto (forward), compound literals, unions
- 222 asserts, 25 files, 17/18 assembling

## Current State

### Benchmark: 52B vs SDCC 84B = −38%
```
twice      2B  (SDCC 3B)   MinZ
add        2B  (SDCC 3B)   MinZ
max       12B  (SDCC 12B)  TIE
abs_diff   5B  (SDCC 11B)  MinZ −55%
sum_to    21B  (SDCC 25B)  MinZ
clamp8    10B  (SDCC 30B)  MinZ
```

### Tetris
- **MZV (MIR2 VM):** ✅ works perfectly, 500+ frames
- **MZX (Z80):** ❌ black screen — zx_screen_addr register pressure
  - 3/4 sub-bugs FIXED (genMul16, constVals, emitCallArgs)
  - Remaining: PBQP doesn't spill when >7 live u8 regs

### Tests
- 24 E2E Z80 tests (all pass)
- 222 C89 corpus asserts (all pass)
- 26/26 Go test packages

## What To Do Next (priority order)

### 1. PBQP Spill Support (blocks tetris on Z80)
- `zx_screen_addr` has 8+ live u8 values, only 7 physical regs
- PBQP silently aliases → A clobbered by ALU scratch
- Plan: Level 1 detect → Level 2 PUSH/POP spill → Level 3 IX+d slots
- See `memory/pbqp_spill_plan.md`

### 2. Function Pointer C89 Lowerer
- Backend ready (OpCallIndirect + __call_hl + VM)
- C89 lowerer needs to detect fn ptr params and emit OpCallIndirect
- Nanz needs fn ptr type syntax

### 3. BUG-008 (only blocking bug)
- Arena codegen: `LD IXL,(IX+d)` impossible on Z80
- Blocks struct methods with pointer receivers

### 4. abs_diff v5 (7B → 5B)
- `d=a-b; if(a<b) d=b-a; return d` — SplitJoinRet works but neg-rewrite doesn't fire
- Then-block has `sub(b,a)` that should be `neg(sub_result)`

### 5. ADR-0027: Constraint-Driven Instruction Selection
- WFC/table approach for codegen
- Replaces 5K LOC if-chains with data tables
- Forward+backward constraint propagation
- See user's detailed ADR proposal in conversation

## Key Files Changed
```
minzc/cmd/mzv/main.go          — MZV runner (250 LOC)
minzc/cmd/mzv/font.go          — ZX ROM font OCR (120 LOC)
minzc/pkg/mir2/z80codegen.go   — many codegen fixes
minzc/pkg/mir2/absdiff.go      — FuseAbsDiff pass (NEW)
minzc/pkg/mir2/joinret.go      — SplitJoinRet pass (NEW)
minzc/pkg/mir2/div_e2e_test.go — div/mod E2E tests (NEW)
minzc/pkg/mir2/alloc.go        — A-clobber interference edges
minzc/pkg/mir2/constprop.go    — SimplifyIdentities expanded
minzc/pkg/mir2/vm.go           — OpCallIndirect VM dispatch
minzc/pkg/hir/lower.go         — CondExpr desugar, % operator
minzc/pkg/pipeline/pipeline.go — SplitJoinRet + FuseAbsDiff wired in
```

## Memory Files Updated
- `memory/codegen_roadmap.md` — prioritized improvement list
- `memory/pbqp_spill_plan.md` — spill support plan
- `memory/session_2026_03_15_codegen_hardening.md` — session log
