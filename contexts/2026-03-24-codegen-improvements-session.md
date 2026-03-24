# Session Extract: VIR Codegen Improvements (2026-03-24)

## What We Did

Starting from VIR 520/520 (100% coverage) with -60% vs SDCC, we improved to **-71%** in 14 commits.

### 1. Deterministic Z3 encoding
- `sort.Ints()` on all Go map iterations in solver.go (8 sites) and cfgsolver.go (4 sites)
- Root cause of abs_diff flakiness: map order → different Z3 assertions → different models
- 30/30 stable after fix

### 2. Editorial review of VIR 100% report
- "swap" → "select_b" (was misleading — dead-code elimination test, not a swap)
- All asm examples replaced with actual compiler output
- Added sections: C89 out-param promotion, HIR function splitting, Grace on MIR2
- Renamed "SDCC-inspired" → "superoptimizer-derived" (z80-optimizer/CUDA)

### 3. Three peephole rules
- **Conditional RET**: `JR/JP cc, .skip / [labels] / RET / .skip:` → `RET cc_inverted`
- **Dead LD**: `LD r, X / LD r, Y` → remove first
- **Reverse-copy**: `LD X, Y / LD Y, X` → keep first only

### 4. abs_diff grace fusion
- `fuseAbsDiffASM()`: detects `CP r / JR Z+JR C / block(SUB,RET) / block(SUB,RET)`
- Replaces with `SUB r / RET NC / NEG / RET` — 4 insts, matches MIR2 optimal
- Also wired `mir2.FuseAbsDiff` pre-pass + `CmpSubCarry` bridge support

### 5. CFG solver: soft edge constraints (THE BIG FIX)
- Root cause of unsat: hard `(assert (= from to))` on CFG edges forced vregs to stay in same register across blocks
- abs_diff else-block needs `b` in A (for SUB), but PFCCO put `b` in C → impossible
- Fix: `(ite (= from to) 0 4)` — soft penalty instead of hard constraint
- gcd: 15→10 (solver can now move b from L to C at block entry)

### 6. Duplicate CP elimination + JP threading
- `CP r / JR cc / [labels] / CP r` → skip second CP (flags unchanged after JR)
- `JP .L1` where `.L1: JP .L2` → `JP .L2` (thread jumps)
- gcd: 10→9

### 7. `--vir` CLI flag
- Added, tested, made default briefly, then **reverted to --lir default**

### 8. Paper draft Section 4 rewrite
- 520/520, -71%, soft edges, case studies for abs_diff (4 insts) and gcd (9 insts)
- 6 design insights, updated conclusion with 3 key contributions

### 9. VIR vs LIR real-world stress test (CRITICAL FINDING)
- Ran all 30 nanz examples through both backends
- **VIR: 14/30 pass, 16/30 fail** (11 invalid asm, 3 wrong results, 2 parse)
- **LIR: stable on all** — PBQP handles features VIR doesn't
- Reverted `--vir` default → `--lir` remains production default

## Final Benchmark (validated corpus)

| Program | SDCC | VIR | vs SDCC |
|---------|------|-----|---------|
| abs_diff | 12 | **4** | **-67%** |
| gcd | 17 | **9** | **-47%** |
| minmax | 60 | 11 | -82% |
| fib | 22 | 12 | -45% |
| select_b | 20 | 2 | -90% |
| **TOTAL** | **131** | **38** | **-71%** |

## Key Insights

### 1. Soft CFG edges are THE breakthrough
Hard equality at block boundaries causes UNSAT when successor blocks need values in different registers. One-line change (hard → soft penalty) resolved all UNSAT cases. gcd: 15→9.

### 2. Go map non-determinism is the #1 correctness risk
Z3 is deterministic given identical input. Go maps make the encoding order-dependent. Every `for k := range map` must be sorted.

### 3. Assembly-level fusion beats solver restructuring for idioms
abs_diff `SUB/RET NC/NEG/RET` — 50 lines of Go pattern matching vs weeks of solver redesign.

### 4. VIR is NOT production-ready
520/520 corpus passes, but 16/30 real examples fail. Missing: 16-bit symbol addresses, some conditional patterns, match expression codegen. LIR/PBQP is the stable backend.

### 5. LIR vs VIR on gcd shows the architectural difference
LIR gcd: 14 insts (parallel-copy artifacts, NEG+ADD in else path). VIR gcd: 9 insts (soft edges, duplicate CP elim, JP threading). The gap is entirely due to cross-block register movement capability.

## Files Changed (14 commits)

- `minzc/pkg/vir/solver.go` — sort all map iterations
- `minzc/pkg/vir/cfgsolver.go` — sort maps + **soft edge constraints**
- `minzc/pkg/vir/pipeline.go` — 5 peephole rules, invertCC(), fuseAbsDiffASM(), JP threading, FuseAbsDiff pre-pass, CmpSubCarry, duplicate CP elim
- `minzc/pkg/vir/bridge.go` — CmpSubCarry no-op translation
- `minzc/pkg/vir/compare_test.go` — swap→select_b
- `minzc/cmd/minzc/main.go` — --vir flag (opt-in, LIR default)
- `reports/2026-03-23-109-VIR-100-Percent-Showcase.md` — full rewrite
- `research/abi-paper/vir-solver-draft.md` — Section 4 rewrite
- `README.md` — -71% featured
- `contexts/` — session docs
