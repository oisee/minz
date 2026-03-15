---
name: Session 2026-03-15 macbookairm2 — full day context dump
description: Complete session state for handoff — tetris assembles, MIR2 verified, ABI verification proposed
type: project
---

## Session: 2026-03-15, alice-2.local (MacBook Air M2, arm64, macOS 15.5)

### Commits pushed (chronological)
```
b1cf9cd fix: tetris v1 & v2 compile and assemble to runnable binaries
43ddcc6 feat: output format flag (-f), computeDeadConsts u16 fix, showcase regression test
e6e9b7b chore: remove duplicate lizp files from showcase
037fbc5 fix: array index load bug + caller-save PUSH/POP around CALL
dca20c3 docs: Open_Bugs_RCA — add BUG-009/010/011
3d18c28 test: add Pascal (.pas) to showcase regression test
f50ea08 fix: OpAddrOf 8-bit guard + emitMov pair↔8bit truncation
47f277c fix: InlineTrivial ptr-op exclusion + ADD HL,IX guard
00eef29 docs: Open_Bugs_RCA — add BUG-012/013/014, profiling results
daa72cf test: MIR2 VM tetris assertions + consolidate to single tetris.nanz
```

### Neighbor's commits (merged in this session)
```
5781677 feat: Lizp frontend (522 LOC, 17 tests, 6 examples)
ff025e5 chore: gitignore cleanup
a54ac36 feat: cross-language import (.lizp ↔ .nanz)
049006d docs: Lizp frontend report (#078)
7c62a29 feat: Pascal frontend (TP3.0 subset → HIR → Z80)
44f2600 feat: Pascal auto-import system unit
c654217 fix: Pascal BDOS register convention
de07ac4+ neighbor working on mzd ABI annotations (IN/OUT/CLOBBER)
```

### Bugs Fixed (BUG-009 through BUG-013)
| # | Bug | RCA | Fix | File |
|---|-----|-----|-----|------|
| 009 | Array load [N]T | VarRefExpr no ArrayTy check | 3-line guard | hir/lower.go:1400 |
| 010 | Caller-save missing | No caller-save in codegen | callerSavePairs() PUSH/POP | z80codegen.go |
| 011 | Dead const u16 cmp | inst.Ty=bool, not operand width | Check lhs phys reg | z80codegen.go:664 |
| 012 | InlineTrivial ptr-op | ≤4 inst with addr_of inlined | Exclude OpAddrOf/OpPtrAdd | inline.go:99 |
| 013 | ADD HL,IX invalid | No IX/IY guard in ADD | Route through temp pair | z80codegen.go (3 paths) |

### Infrastructure Built
- **`-f code/sna/tap`** — output format independent of target
- **TestShowcaseCompileAssemble** — 89 files, 5 frontends (nanz/lanz/lizp/plm/pascal)
- **MIR2 VM OpPtrAdd** — tetris logic verified correct at MIR2 level
- **MZA case-sensitive labels** — symbolKey() helper
- **MZA JR auto-promotion** — pattern_matcher passes out-of-range to encodeJRRel
- **MZA JRS 3-byte reservation** — FromJRS flag

### Current State: Tetris
- **tetris.nanz** (was tetris_v2.nanz) — single canonical version
- **0 asm errors**, 3766 bytes CODE binary
- **MIR2 VM assertions PASS** — logic correct (piece_dx/dy, colors, board)
- **Screen BLACK on MZX** — game loop runs (1M instructions/100 frames)
- **Profiling shows:** zx_poke called 19K times, fill_cell barely runs (35 vs 1600+)
- **Root cause:** Z80 codegen, NOT logic — MIR2 VM proves correctness

### Open Bugs
| # | Bug | Severity | Status |
|---|-----|----------|--------|
| 001 | GCD parallel-copy bloat | 🟡 | Open (needs PBQP affinity) |
| 006 | Zero-size struct global | 🟡 | Open |
| 007 | Spurious adapter LD | 🟡 | Open |
| 008 | Arena IXL,(IX+d) | 🔴 | Open |
| 014 | Tetris black screen | 🔴 | Open (profiled, codegen) |

### Proposed: ABI Verification (see memory/abi_verification_proposal.md)
- Compare MIR2 contract (expected) vs mzd analysis (actual)
- Catches clobber bugs, inline bugs, contract bugs automatically
- Neighbor adding IN/OUT/CLOBBER to mzd
- Feature-branch: `review/mir2-codegen-hardening`

### Proposed: Codegen Hardening (Report #077)
1. TRS peephole (67 Go rules → struct literals, 1 week)
2. Embedded Datalog (liveness, inline candidates, 2 weeks)
3. Equality Saturation (global optimum, PLDI paper, future)
4. ADR-0020 EdgeCost (PBQP instruction-level constraints)

### Files Modified (uncommitted)
```
minzc/pkg/z80testing/tsmc_benchmark_report.txt — pre-existing flaky
minzc/tests/corpus/results/summary.json — stale
```

### Binary status
```
minzc/mz     — rebuilt with all fixes
minzc/mzx    — ZX Spectrum emulator (working)
minzc/mzd    — disassembler (working)
minzc/mztap  — TAP loader (working)
```

### How to reproduce tetris test
```bash
cd minzc
./mz ../examples/zx/tetris.nanz -o /tmp/tetris.a80
./mz /tmp/tetris.a80 -f code -o /tmp/tetris.code
./mzx --run /tmp/tetris.code@8000 --set EI,IM=1
# Profile: add --profile /tmp/prof.json --trace /tmp/trace.jsonl
```

### How to run all tests
```bash
cd minzc
go test ./pkg/... -count=1 -vet=off  # 28 packages, ~27 pass (TSMC flaky)
# Showcase: go test ./pkg/nanz/ -run TestShowcaseCompileAssemble -v
```
