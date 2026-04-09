# Allocator & Optimization Roadmap

**Last updated:** 2026-04-07
**Status:** PBQP is the production path again. Near-term focus: PBQP-first cleanup, PBQP-PFCCO, then bounded call-island allocation.

---

## Current State

Production direction is now explicit:

- PBQP remains the default allocator on the normal compile path
- enriched GPU tables remain the fast path for covered small shapes
- Z3 is no longer the intended production-critical allocator
- any future Z3 work is research / oracle / bounded-region work, not a dependency for normal compilation

That changes the roadmap priority. The next value is no longer "make the current Z3 path survive more shapes." The next value is:

1. make the PBQP-first path cleaner and more deterministic
2. extend PBQP interprocedurally (PBQP-PFCCO)
3. add bounded multi-function region solving where the live shape is small enough

## 2026-04 Direction

### 1. PBQP-first production cleanup

Goal:

- remove Z3 from the default/critical path
- keep `--vir` as explicit experimental opt-in only
- simplify fallback chains and default-path assumptions around "solver may timeout / return unknown"

Why first:

- improves reliability immediately
- removes the runtime dependency on `z3` for normal compilation
- makes future allocator work easier to reason about because the production baseline is stable

### 2. PBQP-PFCCO (module-level calling conventions)

Goal:

- choose parameter/return homes across the call graph with PBQP instead of Z3-PFCCO

Model:

- each function is a node
- each call site contributes an edge
- each edge carries adapter-cost matrices for param/ret mismatches

Expected benefit:

- most of the win of interprocedural register-first ABI tuning
- no SMT timeout / unknown behavior
- deterministic module-wide contract selection

### 3. Bounded call-island allocation

Goal:

- allocate small caller+callee clusters as one region when the merged live shape is still small

This is not "inline everything." It is:

- keep code as separate functions
- but solve a bounded private/clonable call cluster with one joint register problem

Use cases:

- hot caller with one tiny private callee
- outer loop + tiny helper
- small call island whose merged live webs still fit in tables or tiny PBQP

Guardrails:

- private or clonable callee only
- bounded web count
- no recursion / hostile SCCs
- normal PBQP/PFCCO fallback when the island escapes the budget

### 4. Loop and shape rewrites remain important, but after the allocator baseline is stable

Pointer threading, multi-access loop shaping, row-helper extraction, and Grace profitability are still valuable. They now sit after the PBQP-first allocator work because they multiply the value of a stable allocator baseline rather than compensating for a fragile Z3 path.

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

## New Workstream — PBQP-PFCCO

The ABI classes are good; the remaining gap is choosing them jointly across the module instead of per function.

### Goal

Replace Z3-PFCCO with a dedicated PBQP pass that chooses per-function contracts by minimizing adapter costs over the call graph.

### Formulation

For each function:

- candidate homes for params/returns become PBQP choices

For each call edge:

- add edge costs that encode:
  - zero cost when caller/callee homes agree
  - positive cost for adapter `LD` / shuffle sequences
  - optional bonuses for natural accumulator / pair conventions

### Why this matters

- keeps interprocedural ABI optimization without SMT fragility
- gives the right foundation for bounded call-island allocation
- keeps the production stack "tables + PBQP" instead of "tables + PBQP + SMT exception path"

## New Workstream — Call-Island Allocation

This is the strongest follow-up after PBQP-PFCCO.

### Idea

When a small hot caller and one or more tiny callees form a bounded cluster, solve them as a joint region instead of as fully isolated functions.

### Important constraint

This only works cleanly for:

- private callees, or
- cloned/specialized callee variants

Shared callees with many incompatible callers should stay on PBQP-PFCCO + per-function PBQP.

### Solving strategy

1. detect hot/private small call clusters
2. merge web/interference facts across the cluster
3. if the merged region fits table/island limits:
   - solve with enriched tables when the shape is directly covered, or
   - solve with tiny PBQP when boundary conditions need explicit handling
4. otherwise fall back to normal per-function PBQP with PBQP-PFCCO contracts

### Why it is interesting

- removes adapter moves across call boundaries inside the cluster
- approximates selective inlining benefits without always inlining code
- creates a novel "module-local optimal island" path that existing Z80 compilers generally do not have

---

## Priority Order

```
1. Finish PBQP-first default-path cleanup
   → Z3 off the critical path, deterministic default behavior

2. PBQP-PFCCO
   → module-level calling convention optimization without SMT

3. Phase 6e: PBQP edge costs
   → better local correlated-placement choices inside the PBQP path

4. Bounded call-island allocation
   → joint caller+callee solving for private small hot clusters

5. lospre / loop-shape work
   → reduces pressure and increases table/island hit rate

6. EXX save regions
   → only after PBQP/PFCCO/island groundwork is stable
```

---

## References

| File | Purpose |
|------|---------|
| `pkg/mir2/alloc.go` | Current greedy allocator |
| `pkg/mir2/contracts.go` | Register class definitions, PhysLoc |
| `reports/2026-04-07-Z3-Rewrite-Report.md` | Why Z3 leaves the critical path; how to rebuild it later if needed |
| `docs/adr/0017-lut-pointer-selection-and-pbqp-edge-costs.md` | Phase 6e spec |
| `docs/adr/0010-register-first-calling-convention.md` | ABI decisions |
| `docs/P4_Register_Allocator_Plan.md` | $F0xx elimination plan |
| `reports/2026-03-12-060-Codegen_Fixes_And_Allocator_Roadmap.md` | Session report with full analysis |
| `/Volumes/safe/z80-compiler/2112.01397v2.pdf` | Krause 2022 — Z80 calling conventions |
| `/Volumes/safe/z80-compiler/2011.10789v1.pdf` | Krause 2020 — lospre algorithm |
| `/Volumes/safe/z80-compiler/2010.04633v2.pdf` | Krause 2024 — Padauk C compiler |
