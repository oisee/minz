# MIR2 Phase 5b — Interprocedural Contract Optimisation (PBQP)
**Date:** 2026-03-09
**Report:** #040
**Status:** ✅ COMPLETE — 23/23 test packages pass

---

## What Changed

### Before Phase 5b

Every function's parameter and return register classes were **fixed at IR-build time**
(either by hand-annotation in examples, or by HIR lowering's conservative default: `ClassGeneral`).

Two independently-optimised functions could create needless adapter moves at call sites:

```
; double_bad(x: ClassGeneral)  ← param annotated as "any general reg" (C/D/E/H/L)
double_bad:
    LD A, C          ; ← adapter: body needs A, param landed in C
    ADD A, C
    RET

apply_twice:
    CALL double_bad  ; caller passes arg in A, but callee expects C — mismatch!
    RET
```

### After Phase 5b

The optimizer performs a **bottom-up DP pass on the call graph** and finds the
globally-cheapest class assignment for each param/return slot:

```
; double_bad(x: ClassAcc)  ← optimizer promoted ClassGeneral → ClassAcc
double_bad:
    ADD A, A         ; ← no adapter needed: param IS in A
    RET

apply_twice:
    CALL double_bad  ; passes arg in A, callee expects A — zero overhead
    RET
```

**The `LD A, C` adapter instruction is eliminated.**

---

## How the Allocator Pipeline Works Now

```
HIR Module
   │
   ▼
hir.LowerModule()          → *mir2.Module (params assigned HIR default classes)
   │
   ├─ ReorderBlocks(f)     ← block layout (cold/hot reordering)
   ├─ DeadStoreElim(f)     ← removes dead OpConst results
   │
   ▼
mir2.Verify(m)             ← structural correctness check
   │
   ▼ ← NEW: Phase 5b
BuildCallGraph(m)          → edges + Kahn topological sort (leaves first)
OptimizeContracts(m, ct)   → ContractSet (name → ContractChoice)
ApplyContracts(m, cs)      → patches Contract.Params[i].Class in place
   │
   ▼
ComputeLiveness(f)         ← per-function liveness analysis
Allocate(f, lr, ct)        ← greedy register allocator (uses updated classes)
   │
   ▼
Z80Codegen(m, combined)    ← emits assembly using Contract.Params[i].Class
                              to decide how to set up call args (emitCallArgs)
```

The key connection: `Z80Codegen` already uses `callee.Contract.Params[i].Class` in
`emitCallArgs` to decide how to move values before each `CALL`. Changing the class
through `ApplyContracts` automatically changes codegen — no other modifications needed.

---

## The Algorithm: Greedy DP on the Call Graph

### Step 1 — Build call graph

Scan every `OpCall` instruction in every function. Build:
- `Edges[caller]` → list of `(callee, callCount)` pairs
- `Callers[callee]` → list of callers (for topo sort)

### Step 2 — Topological sort (leaves first)

Kahn's algorithm on the **reverse** call graph (callee→caller direction).
Result: callees always appear before their callers.

Why leaves-first? When we optimise caller `f`, all callees of `f` are already decided.
The `edgeCost` function can then look up the callee's chosen class in `ContractSet`.

### Step 3 — For each function in topo order, find cheapest class vector

For each function `f`, enumerate all valid class combinations for its params/returns:

```
plausibleClasses(TyU8)  → {ClassAcc, ClassCounter, ClassGeneral}
plausibleClasses(TyU16) → {ClassPointer, ClassIndex, ClassPair}
plausibleClasses(TyPtr) → {ClassPointer, ClassIndex}
```

Cartesian product, filtered for **physical conflicts**:
> Two params that both uniquely force the same physical register are rejected.
> e.g. (ClassAcc, ClassAcc) → both want A → filtered out.
> But (ClassAcc, ClassCounter) is fine: A ≠ B.

For each candidate vector, compute:

```
total_cost = unaryCost(f, candidate) + edgeCost(f, candidate)
```

**unaryCost**: does the function body need an adapter at the entry?
- `inferNaturalClass` scans the function body for the first meaningful use of each param:
  - `src[0]` of ADD/SUB/AND/OR/XOR/CMP → `ClassAcc`
  - `src[0]` of Load/Store/Field/PtrBump → `ClassPointer`
  - Counter of `TermDJNZ` → `ClassCounter`
  - Passed as `arg[i]` to a known callee → `callee.Params[i].Class`
  - Otherwise → `ClassGeneral`
- If `candidate ≠ natural`, cost = `classMoveCost(candidate, natural)`

**edgeCost**: how much do call sites to already-decided callees cost?
- For each `OpCall` in `f`'s body: look up callee's decided class
- If `argClass ≠ calleeExpectedClass`, add `classMoveCost(arg, callee)`
- Param regs use the **candidate** class (not the current regInfo class) — critical!

**Stability rule**: only replace current contract if `candidateCost < currentCost`
(strict inequality). Ties keep the existing manually-set contract unchanged.

### Step 4 — Apply chosen classes

`ApplyContracts` patches `f.Contract.Params[i].Class` for each function.
The register allocator then assigns physical regs using the updated classes.

---

## On/Off Switch

```go
// Default (Phase 5b enabled):
asm, err := pipeline.CompileHIR(hm)

// With explicit options (e.g. A/B comparison):
asm_opt, _ := pipeline.CompileHIRWithOptions(hm, pipeline.Options{ContractOpt: true})
asm_base, _ := pipeline.CompileHIRWithOptions(hm, pipeline.Options{ContractOpt: false})
```

---

## Concrete Before/After — `TestContractOptimize_BeforeAfterComparison`

```
double_bad(x: ClassGeneral)   ← param starts as conservative default
```

**WITHOUT contract opt:**
```asm
double_bad:
    LD A, C      ; 4T — adapter: body wants A, param is in C
    ADD A, C     ; 4T
    RET          ; 10T
                 ; total body overhead: +4T per call site
```

**WITH contract opt:**
```asm
double_bad:
    ADD A, A     ; 4T — no adapter
    RET          ; 10T
                 ; saving: 4T per call site
```

For a function called N times: **saves N×4T**. For hot inner loops: significant.

---

## Why Existing Examples Mostly Confirm Current Contracts

On Z80, all register-to-register moves cost 4T. The contract optimizer compares:
- `unaryCost(current_class)` — adapter cost inside the function
- `edgeCost(current_class)` — adapter cost at call sites

For hand-annotated examples (like `buildDoubleSum`), these were already set correctly:
`double(a: ClassAcc)` is confirmed optimal because `ADD A, A` naturally wants A.

The real payoff is **HIR lowering**: when the frontend assigns all params as `ClassGeneral`
(safe conservative default), the optimizer automatically promotes them based on usage.
No manual annotation needed.

---

## Cost Model: Target-Neutral by Design

The contract optimizer uses only the `CostTable` interface:

```go
type CostTable interface {
    Cost(cls RegClass, loc PhysLoc) int  // cost to place class at physical location
    Locs() []PhysLoc                     // all physical locations
}
```

The optimizer logic has **zero Z80-specific knowledge**. The same code runs for:

| Target | CostTable | ClassAcc maps to | ClassPointer maps to |
|--------|-----------|------------------|----------------------|
| Z80    | Z80CostTable | A (reg) | HL (reg pair) |
| SM83 (Game Boy) | SM83CostTable | A | HL |
| eZ80 | eZ80CostTable | A | HL (or 24-bit UHL) |
| 6502 | 6502CostTable | A | ZP pointer pair |
| 68000 | 68kCostTable | D0 | A0 |

Add a new `CostTable` implementation → contract optimisation works automatically.

---

## Z80 Limitations and When It Really Helps

On Z80, all `LD r,r'` moves cost 4T regardless of which regs. So cost differences only
arise when:

1. **HIR-lowered params start as `ClassGeneral`** — optimizer upgrades to correct class
2. **16-bit params**: `ClassPointer` (HL) vs `ClassIndex` (DE) — operations like `ADD HL,xx`
   only work with HL; wrong choice forces `EX DE,HL` (4T) or `PUSH/POP` pair
3. **3+ param functions** — conflict filtering prevents impossible combinations

On architectures with heterogeneous move costs (68000, RISC), the gains are larger because
some moves are free and others are expensive — the optimizer finds those sweet spots.

---

## Test Coverage (12 new tests in contracts_test.go)

| Test | What it verifies |
|------|-----------------|
| `TestCallGraph_LeafFunction` | single leaf: acyclic, IsLeaf=true |
| `TestCallGraph_DoubleSum` | double before double_sum in topo order |
| `TestCallGraph_IterativeFib` | iterative fib = leaf (no OpCall) |
| `TestCallGraph_Recursive` | recursive fact → cyclic, still in Order() |
| `TestCallGraph_ThreeLevelChain` | a→b→c: c < b < a in topo order |
| `TestContractOptimize_ConfirmsDoubleChoice` | double(A) confirmed optimal |
| `TestContractOptimize_PreservesOutput` | double_sum assembly correct after opt |
| `TestContractOptimize_PointerFuncPreservesPointer` | ptr param stays ClassPointer |
| `TestContractOptimize_CostReduction` | ClassGeneral→ClassAcc for ADD A,A body |
| `TestContractOptimize_BeforeAfterComparison` | before: LD A,C; after: ADD A,A |
| `TestBestLocForClass` | A/B/HL/DE are cheapest for their classes |
| `TestClassMoveCost` | same class=0T, different class>0T |

---

## Phase 5 Roadmap Status

```
Phase 5a — Standard optimisations (DCE, const prop, coalescing)    [ ] not started
Phase 5b — Function contract optimisation (interprocedural PBQP)   [x] DONE ✅
Phase 5c — SoA256 page assignment (memory layout PBQP)             [ ] gated on profiling data
Phase 5d — Bank switching layout (128K targets)                    [ ] gated on 128K backend
```

Next: Phase 5a (DCE, constant propagation across blocks) unlocks further 5b gains because
more params become dead/constant-propagated, simplifying the candidate space.
