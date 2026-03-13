# Report #066 — Multi-Caller Cost Accounting + Trivial Function Inlining

**Date:** 2026-03-13
**Category:** Architecture / Optimization Analysis
**Status:** Phase 6e + Phase 6f complete · 26/26 pkg tests PASS

---

## The Problem Multi-Pass Contracts Solve

A compiler's calling-convention optimizer faces a fundamental asymmetry: when it
evaluates a callee function, it looks inward (body cost) and forward (what the
callee pays to prepare its return values), but it cannot look *backward* — at
what each caller pays to set up arguments.

This asymmetry is benign for functions with a single call site.  It becomes
expensive for shared utilities called from multiple places.

### A concrete example: `swap`

```nanz
fun swap(a: u16, b: u16) -> (u16, u16) {
    return (b, a)
}
```

The MinZ ABI optimizer (PFCCO) discovered a clever convention for `swap`:
assign `a=DE, b=HL` so the function body becomes a bare `RET` — the returned
pair `(b, a)` is already in `(HL, DE)` exactly as the multi-return ABI requires.
Zero instructions in the body.

A spectacular result.  Except there are three call sites.

Each caller must pre-arrange: put its first argument into DE (the "wrong" register
from a natural HL-first perspective) before the call.  On Z80 this costs an
`EX DE,HL` — 4 T-states per call site, 12 T-states total for three callers.

The alternative convention (EX inside the body, normal arg order at call sites)
costs 4 T-states in the body, 0 T-states per caller: 4 T-states total.

The optimizer, in its single-pass form, could not see any of this.  It evaluated
`swap` before any caller had been decided, saw "bare RET = 0 body cost", and
chose it — correctly by local reasoning, wrongly by global reasoning.

---

## The Solution: Iterative Incoming-Edge Cost

**Pass 0** runs the existing bottom-up greedy DP: process callees before callers,
minimise body cost + outgoing-edge cost.  This is unchanged and correct for the
first pass.

**Pass 1+** re-evaluates each function augmented by `incomingEdgeCost`:

```
incomingEdgeCost(f, choice) =
    Σ over all callers c of f:
        Σ over each argument arg_i:
            classMoveCost(class(arg_i in c), choice.ParamClass[i])
```

This sums the move costs every caller would pay if `f` adopted `choice` as its
contract.  The optimizer compares:

```
totalCost(choice) = bodyCost(f, choice)
                  + outgoingEdgeCost(f, choice)   ; what f pays for return moves
                  + incomingEdgeCost(f, choice)   ; what all callers pay for arg setup
```

Passes repeat until convergence (no function changes its contract).  In practice
the benchmark suite converges in 1–2 additional passes; the cap is 4.

For `swap` with N=3 callers, the optimizer NOW correctly identifies:

| Convention | Body cost | Per-caller cost | Total (N=3) |
|------------|-----------|-----------------|-------------|
| B (bare RET, twisted ABI) | 0 T | 4 T (EX) | **12 T** |
| A (EX inside, standard order) | 4 T | 0 T | **4 T** |

Convention A wins by 8 T.  Pass 1 switches the contract.

---

## What Actually Improved in Generated Code

Multi-pass contract optimization has a concrete, verifiable effect on the
showcase assembly.  The improvements come from loop CFG edges where a
re-evaluated contract class for a callee changes which register the caller
places values in — and that in turn eliminates a "trampoline" block that
existed only to shuffle registers at the loop exit edge.

### ex9b — `factorial_fold`

A trampoline block was previously emitted at the loop exit:

```z80
; Before multi-pass:
    JRS NC, .factorial_fold_trmp0
    ...
.factorial_fold_trmp0:
    LD D, C          ; 4T: move C → D for exit block param
    JRS .factorial_fold_loop_exit4
```

After multi-pass convergence the optimizer assigns the loop counter a class
that the exit block already expects:

```z80
; After multi-pass:
    JRS NC, .factorial_fold_loop_exit4   ; direct — no trampoline
```

**Saved:** 2 instructions, 1 label, 8 T-states on the hot exit path.

### ex14 — `sum_to`

`sum_to` had a four-instruction trampoline at its loop exit:

```z80
; Before multi-pass:
    JRS NC, .sum_to_trmp0
    ...
.sum_to_trmp0:
    LD A, C          ; ─┐
    LD C, D          ;  │  4 instructions
    LD D, A          ;  │  + 1 label
    JRS .sum_to_loop_exit4  ; ─┘
```

After multi-pass:

```z80
; After multi-pass:
    JRS NC, .sum_to_loop_exit4           ; direct
```

**Saved:** 4 instructions, 1 label, 16 T-states on the hot exit path.

Both changes are correctness-preserving transformations verified by the
full test suite and Z80 emulator E2E checks.

---

## The Swap Case: Optimizer is Right, Allocator is Bound by Stale Physics

Here is an important nuance worth documenting.

The multi-pass optimizer correctly selects Convention A for `swap` (EX inside
the body).  The contract record changes.  But the PBQP register allocator, when
it runs on the swap body, sees a different reality:

The body contains virtual registers `r1..r4` with explicit crossover-move
instructions synthesized at HIR lowering time:

```
r3 = OpMove(r2, ClassPointer)   ; "put r2 into HL-class reg"
r4 = OpMove(r1, ClassIndex)     ; "put r1 into DE-class reg"
```

`r1` and `r3` both want `HL` (ClassPointer).  They interfere.  PBQP cannot
assign both to `HL`, so it falls back:

```
r1 → DE,  r2 → HL    (liveness forces this)
r3 = OpMove(r2, ClassPointer) → r3 = r2 = HL   (coalesced, noop)
r4 = OpMove(r1, ClassIndex)   → r4 = r1 = DE   (coalesced, noop)
```

The return instruction sees `r3=HL, r4=DE` — identical to the twisted ABI.
`RET` is emitted.  The callers still see `EX DE,HL` in practice.

**What happened:** The optimizer correctly labels the function's contract as
"standard order."  The allocator, working from the stale body IR, physically
allocates the same way regardless.  The two layers disagree.

This is not a bug in either layer in isolation — each layer is internally
consistent.  It is a structural limitation: the body IR encodes the calling
convention implicitly in its move instructions, and changing the contract
label without re-synthesizing those moves has no physical effect.

### "In-Mind Swap" — What Zero-Cost Actually Means at CPU Level

When a callee uses the twisted ABI and a caller has its values in the "wrong"
registers already (e.g. `a` is in `DE` naturally because it came from a prior
call that returned into `DE`), the `EX DE,HL` at the call site may already be
free — the caller just reinterprets which name lives in which register.  No
bits move.

This is why the twisted-ABI analysis matters most for inter-procedural
optimization: in an inlined or fused context, the "register-tag rename" is
truly zero-cost.  The CALL+RET overhead (27 T-states) is the actual cost, not
the convention mismatch.

---

## Phase 6e PBQP Affinity Nudges — Complete Summary

Three affinity nudges are now active in the pre-PBQP cost adjustment phase:

### BC★ / DE★ — Page-Aligned LUT Index (Phase 6c/6d)

```
Pattern:  src8 → OpExt(u8→u16) → OpPtrAdd(base, idx) → OpLoad
Reward:   −4T on C and E for src8
Rationale: BC★ path (LD B,sym^H + LD A,(BC)) = 14T
           HL  path (LD H,sym^H + LD L,idx + LD A,(HL)) = 18T
```

### mul16 rhs → DE (Phase 6e)

```
Pattern:  OpMul, 16-bit, Src[1] = rhs (the multiplier)
Reward:   −8T on DE for rhs
Rationale: genMul16 fast path skips LD D,high(rhs); LD E,low(rhs) = 8T
           when rhs already in DE
```

### DJNZ Counter → B (Phase 6e)

```
Pattern:  TermDJNZ.Counter
Reward:   −4T on B for Counter
Rationale: DJNZ uses B implicitly; LD B,other_reg = 4T saved
```

**Current status:** The DJNZ nudge correctly targets `TermDJNZ.Counter` (the
body block's ClassCounter param, already B-class by construction).  The `LD B,C`
seen in `forEach` loops originates from the **entry→loop-head parallel copy**
for the function parameter `n` (ClassGeneral=C).  Eliminating that copy requires
pre-coalescing the function param with the block param — the same blocker as
BUG-001/OpPtrAdd.  The nudge is correct and will activate once PreallocCoalesce
is wired in.

---

## The Path Forward: Function Inlining

The single remaining structural obstacle for the swap case — and more generally
for algebraic simplifications like `swap(a,b).1 == a` — is the absence of a
MIR2-level function inliner.

A proposal exists on the `research/abi-optimization-paper` branch
(`research/abi-paper/PROPOSAL_inlining.md`) detailing:

- `InlineTrivial()`: inline leaf functions with ≤4 instructions at all call sites
- `PropagateCopies()`: fold move chains introduced by inlining, eliminate tuple
  projections that become identity copies
- Integration point: run after `OptimizeContracts`, before PBQP

With inlining, `swap(a,b)` would be expanded inline, copy propagation would
resolve `(b_old, a_old)` projections, and DCE would delete unreachable values.
The `LD B,C` / `EX DE,HL` / `CALL+RET` overhead would all vanish simultaneously.

**This is a TODO-NICE-TO-HAVE** — correct code is generated today.  The 27
T-state CALL+RET overhead is the price of a non-inlined call regardless of
convention.  No showcase test is wrong.  Inlining is an optimization pass, not
a correctness fix.

---

## Phase 6f: Trivial Function Inlining — Implementation and Results

The inliner described in `research/abi-paper/PROPOSAL_inlining.md` is now live on
master in `pkg/mir2/inline.go`.

### What was implemented

**`InlineTrivial(mod, maxSize=4)`** — single-block leaf functions with ≤4 instructions:
- Substitute caller args for callee params (no copy — direct reg substitution)
- Splice remapped body instructions before the call site
- Emit `OpMove` for each return value → call destination register
- Remove the original `OpCall`

**`PropagateCopies(f)`** — follows `OpMove` chains and replaces all uses:
- `%3 = Move(%2)` → replace all uses of `%3` with `%2`
- DSE then removes the now-unused `OpMove` instructions

**Pipeline position:** runs BEFORE `OptimizeContracts` — fewer call-graph edges
means the contract optimizer works on the already-simplified call graph.

### The swap result: "in-mind swap" realized

```nanz
fun main() -> void {
    let a: u16 = 1000
    let b: u16 = 2000
    print_val(a)
    let (a2, b2) = swap(a, b)
    print_val(b2)        // b2 = swap(a,b).1 = a = 1000
}
```

**Before inlining:**
```z80
main:
    LD HL, 1000
    CALL print_val
    EX DE, HL          ; 4T adapter for swap's twisted ABI
    CALL swap          ; 17T: just RET inside
    LD H, D            ; 4T extract second return value
    LD L, E
    JP print_val       ; total overhead: 35T
```

**After inlining + copy propagation:**
```z80
main:
    LD HL, 1000
    CALL print_val
    JP print_val       ; total overhead: 0T
```

The entire swap call — including the EX DE,HL adapter, the CALL, the RET, and
the result extraction — disappears.  The compiler realizes `swap(a,b).1 == a`
and emits nothing.

### Additional inlining effects in the showcase

**ex13 — `min_of` becomes an alias:**
```z80
; fun min_of(a: u16 = HL, b: u16 = DE) -> u16 = HL
min_of    EQU minmax
```

`min_of` calls `minmax` and returns the first value — after inlining and
copy propagation the optimizer sees that `min_of` is identical to `minmax`
(same arguments, same register, zero transformation).  The EQU alias means
`CALL min_of` and `CALL minmax` compile to the same bytes — 0 overhead.

**ex2_ufcs — `Acc.reset` and `Acc.add` inlined into `sum_two`:**
The small UFCS helper methods (≤3 instructions each) are expanded inline,
eliminating two CALL+RET pairs (54T saved) at the cost of slightly larger
static code size.  On Z80, T-state count is the primary optimization target.

**ex9a — factorial parameter moved to B:**
The inliner changed the call-graph structure seen by `OptimizeContracts`,
causing it to choose `B` instead of `A` for the recursive `factorial` param.
The generated code is 1 fewer branch instruction at the base-case check:
```z80
; Before: CP 1; JRS C; JRS Z; JRS .join  (3 branches)
; After:  LD A,1; CP B; JRS C .join       (1 branch, 17T vs 40T worst-case check)
```

### Tests

```
26/26 pkg packages:  PASS (go test ./pkg/... -vet=off)
Showcase asserts:    all pass (MIR2 VM + Z80 emulator)
```

---

## Summary: What Multi-Pass Contracts + Inlining Achieved

| Metric | Before | After |
|--------|--------|-------|
| Contract passes per compilation | 1 | 1–2 (converges) |
| N-caller cost visibility | None (blind) | Full (incomingEdgeCost) |
| ex9b trampoline | 2-instruction block | Eliminated (direct jump) |
| ex14/sum_to trampoline | 4-instruction block | Eliminated (direct jump) |
| Swap ABI choice | Correctly twisted (optimizer) | Correctly standard (optimizer) |
| Swap physical code | CALL+EX+RET = 35T overhead | 0T (fully inlined + eliminated) |
| min_of | CALL minmax + extract = ~30T | EQU alias = 0T |
| Trivial method calls (UFCS) | CALL+body+RET per site | Body spliced inline |
| Test suite | 23/23 PASS | 26/26 pkg PASS |

The optimizer now reasons globally about calling conventions.  The inliner
completes the picture: what the convention optimizer chose correctly but
couldn't physically manifest, the inliner realizes in machine code.

---

*See also: Report #065 (Phase 6e implementation), `pkg/mir2/inline.go` (inliner + PropagateCopies), `docs/Open_Bugs_RCA.md` (BUG-001 PreallocCoalesce).*
