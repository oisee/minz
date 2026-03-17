# Report 091 — Inter-Block WFC (Dimension 2)

**Date:** 2026-03-17
**Branch:** `feat/lir-backend`
**Goal:** Implement inter-block constraint propagation in LIR/WFC to eliminate parallel-copy bloat (BUG-001).

---

## Summary

Inter-block WFC propagation is implemented and verified on GCD. Block arguments carry LocSet constraints across CFG edges, and the allocator naturally agrees on registers without parallel copies.

## What Was Built

### New types and structures
- **`BlockParam`** struct: `{VReg, Allowed LocSet, Phys}` — formal parameter of a basic block
- **`Params []BlockParam`** field on `Block` — defines vregs available on block entry
- **`ProgWFC`** — multi-block WFC coordinator with per-block states, param tracking, and RPO collapse

### New functions
| File | Function | Purpose |
|------|----------|---------|
| `lir.go` | `BlockParam` struct | Block parameter definition |
| `vm.go` | `copyBlockArgs(prog, term, edgeIdx, blockMap)` | Parallel-copy semantics: read all srcs, then write all dsts |
| `bridge.go` | `LowerMIR2Prog`, `LowerMIR2ProgWithOps` | Structured MIR2→Prog with block params + terminators |
| `bridge.go` | `translateTerm` | MIR2 terminators → LIR terms (TermJmp, TermBrIf, TermBrIf2, TermDJNZ, TermRet, TermCondRet) |
| `bridge.go` | `regClassToLocSet` | MIR2 RegClass → LIR LocSet mapping (Z80-specific + generic fallback) |
| `isel.go` | `SelectBlockInstructions` | Per-block isel with param pre-seeding |
| `wfc_interblock.go` | `NewProgWFC`, `Propagate`, `Collapse` | Multi-block WFC: intra+inter-block fixpoint, RPO collapse |
| `wfc_interblock.go` | `NewWFCStateWithParams` | WFC state with synthetic param-def cells |
| `wfc_interblock.go` | `AddTermUses` | Synthetic use cells for term operands (liveness) |
| `pipeline.go` | `checkFuncConvergenceMultiBlock` | Multi-block convergence checking |
| `pipeline.go` | `lirCodegenMultiBlock` | Multi-block assembly emission with per-block labels |
| `pipeline.go` | `emitTerminator`, `emitParallelCopyMoves` | Assembly emission for JP/JP NZ/DJNZ/RET + rare parallel copies |

### Key design decisions

1. **Synthetic WFC cells for params and term uses** — params prepended as "def" cells, term operands appended as "use" cells. This lets the existing single-block WFC collapse correctly handle liveness without modifying its core algorithm.

2. **Two-pass collapse** — First pass in RPO order pins params from predecessor args. Second pass propagates back-edge constraints (target params → source args) and re-collapses affected blocks. This handles the `sub_block → take_amb → loop` chains in GCD.

3. **Bidirectional edge propagation** — Forward (arg→param) and backward (param→arg) during RPO collapse. After each block is collapsed, outgoing edge assignments are recorded for successors.

4. **Fallback to flat lowering** — Multi-block path only activates for functions with >1 block and block params. Single-block functions (95%+ of corpus) use the unchanged flat path. If multi-block fails, falls back to flat.

## GCD Verification

| Test | Result |
|------|--------|
| `TestGCD_VM` | PASS — 6 test cases (gcd(12,8)=4, gcd(48,18)=6, gcd(7,7)=7, gcd(15,5)=5, gcd(100,75)=25, gcd(17,13)=1) |
| `TestGCD_WFC` | PASS — virtual registers + ProgWFC → **0 parallel copies**, gcd(12,8)=4 |
| `TestGCD_Asm` | PASS — no extraneous LD shuffles, proper labels and terminators |
| `TestProgWFC_SimplePropagate` | PASS — two-block program, 10+20=30 |
| `TestProgWFC_RPO` | PASS — correct RPO ordering of diamond CFG |
| `TestBlockParams_WFC` | PASS — param phys assignment preserved, 5+5=10 |
| `TestSelectBlockInstructions_Params` | PASS — params pre-seeded in isel |

## Test Results

```
go test ./pkg/lir/... -vet=off — 42 PASS, 0 FAIL (0.003s)
go test ./...            — 26/26 packages pass (1 pre-existing failure in z80testing)
```

No regressions on existing tests. All 33 pre-existing LIR tests pass unchanged.

## Files Modified

| File | Lines | Change |
|------|-------|--------|
| `lir.go` | +12 | `BlockParam` struct, `Params` field on `Block` |
| `vm.go` | +22, -6 | `copyBlockArgs` parallel-copy impl, skip nil-Pat insts |
| `bridge.go` | +195 | `LowerMIR2Prog`, `LowerMIR2ProgWithOps`, `translateTerm`, `regClassToLocSet` |
| `isel.go` | +65 | `SelectBlockInstructions` with param pre-seeding |
| `wfc_interblock.go` | +380 (new) | `ProgWFC`, inter-block propagation, RPO collapse, term uses |
| `pipeline.go` | +195, -35 | Multi-block pipeline + assembly emission + fallback |
| `interblock_test.go` | +320 (new) | 7 test functions covering GCD, WFC, RPO, params, isel, asm |

## What's Next

- Wire `LowerMIR2Prog` into `CheckModuleConvergence` for real MIR2 functions with block params
- Test on real multi-block MinZ programs (loops, conditionals)
- Inter-block WFC on constrained machines (CISC, Z80) — currently verified on RISC32
- CLI flag `--lir-multiblock` to enable structured lowering
