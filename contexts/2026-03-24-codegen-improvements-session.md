# Session Extract: VIR Codegen Improvements (2026-03-24)

## What We Did

Starting from VIR 520/520 (100% coverage), we improved code quality across 4 areas.

### 1. Deterministic Z3 encoding (fix)
- Sorted all Go map iterations in solver.go and cfgsolver.go
- Root cause: map iteration order → different Z3 assertion order → different (sometimes wrong) register assignments
- 8 sites in solver.go, 4 in cfgsolver.go: `sort.Ints(vregs)` before every map-to-slice iteration
- abs_diff was 60% flaky before (wrong values 0 or 10 instead of 7), now 30/30 stable

### 2. Editorial review of VIR 100% report
- Renamed misleading "swap" to "select_b" (dead-code elimination test)
- Replaced hand-written asm examples with actual compiler output
- Fixed all instruction counts (gcd 15→16, fib 11→12, abs_diff 13→11)
- Documented gcd cross-block register state bug (else-path SUB L uses stale L)
- Added 3 new report sections: C89 out-param promotion, HIR function splitting, Grace on MIR2

### 3. Three peephole rules
- **Conditional RET**: `JR/JP cc, .skip / [labels] / RET / .skip:` → `RET cc_inverted` with `invertCC()` helper
- **Dead LD elimination**: `LD r, X / LD r, Y` → remove first (dead store). Fixed gcd `LD D,A / LD D,B`
- **Reverse-copy elimination**: `LD X, Y / LD Y, X` → keep only first

### 4. abs_diff fusion (grace pass)
- `fuseAbsDiffASM()` detects `CP r / JR Z+JR C / block1(SUB,RET) / block2(SUB,RET)` at assembly level
- Replaces with `SUB r / RET NC / NEG / RET` — 4 instructions, matches MIR2 production optimal
- Also wired `mir2.FuseAbsDiff(f)` pre-pass and `CmpSubCarry` bridge support for future solver integration
- abs_diff: 11 → 4 instructions (-67%)

## Benchmark Results

| Program | SDCC | VIR (start) | VIR (end) | vs SDCC |
|---------|------|------------|-----------|---------|
| abs_diff | 12 | 11 | **4** | **-67%** |
| gcd | 17 | 16 | 15 | -12% |
| minmax | 60 | 11 | 11 | -82% |
| fib | 22 | 12 | 12 | -45% |
| select_b | 20 | 2 | 2 | -90% |
| **TOTAL** | **131** | **52** | **44** | **-66%** |

## Key Insights

### 1. Assembly-level pattern matching beats solver restructuring for idioms
The abs_diff optimal sequence (`SUB/RET NC/NEG/RET`) requires fusing a comparison, two subtraction blocks, and a conditional return into one sequence. This is natural at the assembly level but would require restructuring VIR blocks and adding new solver constraints. Post-emission grace rewriting was 10x simpler to implement and debug.

### 2. Go map non-determinism is the #1 correctness risk in SMT-based compilers
Z3 is deterministic given identical input. But Go map iteration makes the SMT encoding order-dependent. Every `for k := range map` in the encoding path must be sorted. This is not obvious because the constraints are semantically identical — but Z3's heuristics pick different models.

### 3. Dead LD elimination has cascading benefits
Removing `LD D,A / LD D,B` → `LD D,B` freed register D earlier, allowing the solver to find better allocations in subsequent runs. gcd went from 16→15 with just this rule.

## Files Changed

- `minzc/pkg/vir/solver.go` — sort.Ints on all map iterations, sort.Slice for coalesce/spill/candidates
- `minzc/pkg/vir/cfgsolver.go` — sort.Ints on 4 map iterations
- `minzc/pkg/vir/pipeline.go` — 3 peephole rules, invertCC(), fuseAbsDiffASM(), FuseAbsDiff pre-pass, CmpSubCarry in TermBrIf/TermCondRet
- `minzc/pkg/vir/bridge.go` — CmpSubCarry no-op translation
- `minzc/pkg/vir/compare_test.go` — swap→select_b rename
- `reports/2026-03-23-109-VIR-100-Percent-Showcase.md` — editorial review
- `research/abi-paper/vir-solver-draft.md` — updated benchmarks
