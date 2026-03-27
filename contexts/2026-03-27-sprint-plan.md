# Sprint Plan: 2026-03-27

## Starting Status
- 12 PASS, 2 FAIL (MIR2 bugs: abs_val, gcd)
- Pipeline: Table → Z3 → PBQP fallback (нерушимый)
- 51 commits from birthday marathon
- v0.23.0 + z80-optimizer v1.0.0 released
- 500 arithmetic sequences (254 mul + 246 div) inline

## Sprint Goals

### #22 Morning: Fix MIR2 cmp.ugt codegen bug
**Files:** `minzc/pkg/mir2/z80codegen.go`
**Bug:** `cmp.ugt %r2, %r3` emits `OR A` which tests A (x=5) instead of %r2 (is_neg=0).
**Root cause:** Z80 codegen assumes the comparison operand is already in A. When PBQP puts is_neg in a different register (C, B, etc.), the codegen doesn't emit `LD A, %r2` first.
**Fix:** Before emitting CP/OR for cmp.ugt, check if the source vreg's physical register is A. If not, emit `LD A, <src_reg>` first.
**Impact:** Fixes abs_val AND gcd (same class). 12/14 → 14/14 E2E PASS.
**MIR2 dump for reference:**
```
fun @abs_val(%r1: u8 [acc], %r2: u8 [general]) -> u8 [acc]
  block @entry:
    %r3 = const 0 : u8 [general]
    %r4 = cmp.ugt %r2, %r3 : bool [flag]    ← condition: is_neg > 0
    cond_ret %r4, [%r1], @if_then1()         ← if FALSE, return x (%r1)
  block @if_then1:
    %r6 = neg %r1 : u8 [acc]
    ret %r6
```

### #23 Midday: Block→quasi-function prototype
**Files:** `minzc/pkg/vir/pipeline.go` (new function: `solveRecursive`)
**Design:** Each CFG block = quasi-function:
- params = live-in vregs with known locations
- body = VIR ops (straight-line)
- returns = live-out vregs
- Solve via table (≤6v) or Z3 (≤15v) or backtracking
**Algorithm:**
```
func solveRecursive(ops, liveIn, budget=15):
    try table/Z3/backtrack as single unit → success? done
    fail → find liveness bottleneck, split into 2 sub-problems
    recurse on each half
    stitch with LD moves at boundary
    base case: ≤6v = table hit (83.6M, guaranteed)
```
**Test:** _sel_rows counter drift → should produce consistent IXL (or correct shuffles)
**Replaces:** broken per-block fallback

### #24 Afternoon: Peephole 739K rules post-pass
**Files:** `minzc/pkg/vir/pipeline.go` (add to `peepholeCleanup`)
**Steps:**
1. `go get github.com/oisee/z80-optimizer`
2. Import `pkg/peephole`
3. After existing peephole rules in `peepholeCleanup`:
   - Split ASM into instruction pairs/triples
   - `peephole.Lookup(pair)` → if match, replace with optimal
4. Measure: count rules fired on ZSQL corpus
**Example:** `SLA A : RR A` → `OR A` (saves 12T, 1 byte)

### #25 Evening: --vir graduation check
**Checklist (from VIR_Reliability_Sprint.md):**
1. [ ] Zero known correctness bugs (abs_val/gcd fixed)
2. [ ] 14/14 E2E assert pass
3. [ ] 35/35 Nanz compile + assemble
4. [ ] Differential: VIR = production for all asserts
5. [ ] 112/112 ABAP compile + assemble
6. [ ] Loop+CALL stress test (5+ programs)
7. [ ] No instruction count regression
8. [ ] validateNoClobber = ERROR, 0 false positives
9. [ ] `go test` under 5 minutes
10. [ ] 1 week soak: --vir alongside production

## Dependencies
- #22 is independent (MIR2 fix, not VIR)
- #23 depends on #22 (need correct baseline to test)
- #24 is independent (post-pass, doesn't affect solver)
- #25 depends on #22 + #23 (need all fixes in place)

## Collaboration
- Main session (ju6yy047): may also fix #22 from their side
- z80-optimizer (um2dy4ex): 739K peephole rules ready, div8 246 entries
- antique-toy (eo29c66e): TSMC sidebar material delivered
