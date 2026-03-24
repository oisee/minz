# Session Extract: VIR Codegen Improvements (2026-03-24)

## What We Did

Starting from VIR 520/520 (100% coverage) with -60% vs SDCC, we improved to **-71%** in 7 commits.

### 1. Deterministic Z3 encoding
- `sort.Ints()` on all Go map iterations in solver.go (8 sites) and cfgsolver.go (4 sites)
- Root cause of abs_diff flakiness: map order → different Z3 assertions → different models
- 30/30 stable after fix

### 2. Editorial review
- "swap" → "select_b" (was misleading — it's a dead-code elimination test)
- All asm examples replaced with actual compiler output
- Added sections: C89 out-param promotion, HIR function splitting, Grace on MIR2

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

### 6. Duplicate CP elimination
- `CP r / JR cc / [labels] / CP r` → skip second CP (flags unchanged on fall-through)
- gcd: 10→9

### 7. JP threading
- `JP .label` where `.label: JP .target` → `JP .target`
- Build label→target map, rewrite all JP/JR targets through chains
- Saves one indirection per iteration (no instruction count change, just cycle savings)

## Final Benchmark

| Program | SDCC | Start | End | vs SDCC |
|---------|------|-------|-----|---------|
| abs_diff | 12 | 11 | **4** | **-67%** |
| gcd | 17 | 16 | **9** | **-47%** |
| minmax | 60 | 11 | 11 | -82% |
| fib | 22 | 12 | 12 | -45% |
| select_b | 20 | 2 | 2 | -90% |
| **TOTAL** | **131** | **52** | **38** | **-71%** |

## Key Insight: Soft CFG Edges

The single most impactful change was replacing hard CFG edge equality with soft move-cost penalties. This is a fundamental architectural insight: in a multi-block solver, block boundaries are **move opportunities**, not invariants. The solver should treat cross-block register placement as an optimization variable, not a constraint. This one change dropped gcd from 15 to 10 instructions and made the CFG solver succeed for the first time on multi-path functions.

## Files Changed

- `minzc/pkg/vir/solver.go` — sort all map iterations (determinism)
- `minzc/pkg/vir/cfgsolver.go` — sort map iterations + soft edge constraints
- `minzc/pkg/vir/pipeline.go` — 5 peephole rules, invertCC(), fuseAbsDiffASM(), JP threading, FuseAbsDiff pre-pass, CmpSubCarry handling
- `minzc/pkg/vir/bridge.go` — CmpSubCarry no-op translation
- `minzc/pkg/vir/compare_test.go` — swap→select_b
- `reports/2026-03-23-109-VIR-100-Percent-Showcase.md` — full rewrite with actual output
- `research/abi-paper/vir-solver-draft.md` — updated benchmarks
- `README.md` — -71% featured

## Next Session Priorities

1. **DEC HL elimination at caller level** — when caller doesn't read HL after call, callee's trailing DEC HL is dead
2. **Fallthrough block reordering** — reorder blocks to maximize JR-to-next (eliminate JP)
3. **Paper draft Section 4** — update evaluation with 520/520, abs_diff fusion, soft edges
4. **`--vir` CLI flag** — make VIR accessible from command line
