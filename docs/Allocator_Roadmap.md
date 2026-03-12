# Allocator & Optimization Roadmap

**Last updated:** 2026-03-12
**Status:** Phase 6 (PBQP node costs) done. Phase 6e next.

---

## Current State

`pkg/mir2/alloc.go` — greedy graph-coloring allocator, 518 LOC.

- Builds interference graph (virtuals that can't share a physical location)
- Sorts by degree, colors greedily using Z80CostTable
- Uses **node costs** only: `cost[virtual][physical_location]`
- Spills to `$F0xx` memory when no physical location fits
- No edge costs — allocation decisions are independent per virtual

Result: works correctly, but misses correlated allocation opportunities (e.g. LUT index in C
with pointer in B → 14T instead of 18T, but the allocator doesn't know they're related).

---

## Phase 6e — PBQP Edge Costs (~150 LOC)

**Goal:** Add pairwise edge costs to the PBQP problem so correlated allocation decisions are
jointly optimized.

### What it models

```
minimize  Σ nodeCost[v][loc(v)]  +  Σ edgeCost[u][v][loc(u)][loc(v)]
           v                         (u,v)∈edges
```

### Edge patterns to implement

| Pattern | Edge | Benefit |
|---------|------|---------|
| Page-aligned LUT: `idx` + `ptr_hi` | `edgeCost[idx][ptr][C][BC] = -4T` | 18T → 14T per LUT access (ADR-0017 BC★) |
| Same for DE: `idx` in E | `edgeCost[idx][ptr][E][DE] = -4T` | same |
| `mul16` rhs | `edgeCost[rhs][_][E][_] = -2T` | avoids zero-extend clobber of D |
| DJNZ counter | `edgeCost[counter][*][B][*] = -∞` | keeps B reserved for DJNZ in loop body |
| `ADD HL, rr` result | `edgeCost[dst][*][HL][*] = 0` | HL preference propagates from ADD HL constraint |

### Implementation

1. Add `edgeCost map[[2]Reg][2]int` to the PBQP problem struct in `alloc.go`
2. Pattern detector: walk function instructions, detect LUT/mul16/DJNZ patterns, add edges
3. RN bucket-elimination: project edge costs onto neighbor's node cost vector (standard PBQP)
4. Wire into `Allocate()` before the greedy coloring loop

Reference: ADR-0017 (`docs/adr/0017-lut-pointer-selection-and-pbqp-edge-costs.md`)
LLVM has a reference PBQP implementation (`lib/CodeGen/RegAllocPBQP.cpp`).

---

## lospre — Lifetime-Optimal PRE (~500 LOC)

**Goal:** Eliminate redundant computations while minimizing the lifetime of new temporaries,
reducing register pressure and live-range length → fewer $F0xx spills.

### What it handles

Subsumes: CSE, GCSE, LICM (loop-invariant code motion), common subexpression elimination.

Key property vs plain PRE: minimizes the *lifetime* of newly introduced temporaries, not just
redundancy count. Shorter live ranges → less register pressure → fewer spills.

### Why we care

Our main spill source is long-lived virtuals accumulating across multi-expression statements.
lospre moves computations to where they're needed (not speculatively early), keeping live
ranges tight.

### Implementation

Based on: Krause 2020, "lospre in linear time" (`/Volumes/safe/z80-compiler/2011.10789v1.pdf`)
Linear time for structured programs (no `goto` — all Nanz/PL/M programs qualify).
Uses CFG tree-decomposition — Nanz already has clean block-structured CFG in `mir2.Func.Blocks`.

Steps:
1. Build weighted CFG from MIR2 blocks (edge weight = execution frequency estimate)
2. For each expression pattern: compute use set U, invalidation set I
3. Find optimal life set L minimizing Σ c(e) + Σ l(v) via tree-decomposition DP
4. Insert computations at L, replace uses with the temporary

---

## EXX Save Regions — Shadow Register Bank (~300 LOC)

**Goal:** Borrow {BC', DE', HL'} as 3 extra pair registers in hot loops where EXX overhead
is amortized.

### The constraint

`EXX` and `EX AF, AF'` are **bulk swaps** — all 6 of {BC,DE,HL,BC',DE',HL'} swap atomically.
This is a hyper-edge constraint: you can't independently access individual shadow registers.
Standard PBQP handles binary edges only → shadow registers need a separate pass.

### Algorithm

1. Identify "EXX regions": loops where {B,C,D,E,H,L} are all live and register pressure > 6
2. If EXX cost (4T in + 4T out) is less than spill cost of the hot variable: insert EXX pair
3. Variables allocated to shadow slots are marked "shadow-only"; codegen emits EXX at region
   entry/exit

### When to implement

After Phase 6e + lospre. Shadow registers are only valuable when everything else is fully
utilized. In practice, most functions use ≤ 4 live pairs, making EXX regions a niche
optimization for the hottest loops.

---

## Calling Convention (already optimal)

Per Krause 2022 ("Efficient Calling Conventions for Irregular Architectures",
`/Volumes/safe/z80-compiler/2112.01397v2.pdf`): empirically evaluated thousands of Z80 ABI
variants against Whetstone, Dhrystone, CoreMark, stdbench. Found that SDCC's 20-year-old
stack-based convention was significantly suboptimal.

**MinZ/Nanz current ABI (ADR-0010) matches the empirically optimal convention:**
- 8-bit return → A
- 16-bit return → HL
- 32-bit return → HL+DE (our multi-return)
- First 8-bit arg → C (or A for accumulator-heavy ops)
- First 16-bit arg → HL
- Further args → DE, BC, then stack

No ABI redesign needed. The paper validates our existing direction.

---

## Priority Order

```
1. Fix 3 remaining showcase failures (ex12/13/14) — correctness bugs, ~1 day
   → 23/23 showcase passing

2. Phase 6e: PBQP edge costs — optimization, ~150 LOC, ~2 days
   → LUT BC★ (14T vs 18T), mul16 class hints, DJNZ B propagation
   → ADR-0017 fully specifies this

3. lospre MIR pass — optimization, ~500 LOC, ~3 days
   → reduces live range length → fewer $F0xx spills
   → prerequisite: stable Phase 6e

4. EXX save regions — niche, ~300 LOC, defer until needed
   → only valuable when register pressure is consistently > 6 pairs

5. BUG-003 / BUG-006 — blocking bugs (ptr[i] in while loop, zero-size structs)
   → parallel workstream, not allocator-related
```

---

## References

| File | Purpose |
|------|---------|
| `pkg/mir2/alloc.go` | Current greedy allocator |
| `pkg/mir2/contracts.go` | Register class definitions, PhysLoc |
| `docs/adr/0017-lut-pointer-selection-and-pbqp-edge-costs.md` | Phase 6e spec |
| `docs/adr/0010-register-first-calling-convention.md` | ABI decisions |
| `docs/P4_Register_Allocator_Plan.md` | $F0xx elimination plan |
| `reports/2026-03-12-060-Codegen_Fixes_And_Allocator_Roadmap.md` | Session report with full analysis |
| `/Volumes/safe/z80-compiler/2112.01397v2.pdf` | Krause 2022 — Z80 calling conventions |
| `/Volumes/safe/z80-compiler/2011.10789v1.pdf` | Krause 2020 — lospre algorithm |
| `/Volumes/safe/z80-compiler/2010.04633v2.pdf` | Krause 2024 — Padauk C compiler |
