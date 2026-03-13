# Report #067 — BUG-001: PreallocCoalesce Wired + Five-Fix Chain

**Date:** 2026-03-13
**Category:** Register Allocator / Code Quality
**Status:** BUG-001 Phase 1 complete · 26/26 pkg tests PASS

---

## Background

BUG-001 described a recurring pattern in loop codegen: PBQP assigns a virtual register
in the loop body to one physical register, and its block-parameter counterpart across a
CFG edge to a *different* physical register.  The parallel-copy resolver then emits
a `LD` instruction on every iteration to shuffle the value across — pure overhead.

The fix is **pre-coalescing**: before PBQP runs, merge arg/param pairs at every CFG
edge using a Union-Find structure.  After merging, the two virtual registers become
one.  PBQP assigns them a single physical location; the parallel copy becomes a no-op.

This session wired `PreallocCoalesce(f)` into the pipeline and fixed every correctness
issue found along the way.

---

## Five Bugs Fixed

### Bug A — DJNZ BodyArgs offset (precoalesce.go, propagateClassHints)

`TermDJNZ.BodyArgs` does **not** include the counter.  `body.Params[0]` is the
implicit counter; `BodyArgs[0]` maps to `body.Params[1]`.  Three call sites used
offset 0 instead of 1, connecting BodyArgs[0] to the counter param instead of the
accumulator param.

**In `propagateClassHints` (pass 2):** offset=0 caused `ClassAcc` backward
propagation to be blocked at the counter (`canUpgradeClass(ClassAcc, ClassCounter)` =
false), silently stopping class upgrades before they reached the accumulator and
function parameter.

**In `PreallocCoalesce` union-find:** offset=0 merged `BodyArgs[0]` (the accumulator
result) with `body.Params[0]` (the counter, ClassCounter=B).  After remapping, the ADD
instruction read and wrote the same virtual register as both the counter block-param
and the instruction destination in the same block — breaking liveness analysis.

**Fix:** `propagateArgsAt(t.Body, t.BodyArgs, 1)` in both places.

### Bug B — DJNZ BodyArgs offset (coalesce.go, coalesceAllocResult)

The same offset error existed in the post-allocation copy coalescer.  With offset 0,
`addEdges(t.Body, t.BodyArgs)` created affinity edge
`{dst: counter_param (B-class), src: accumulator_result (A)}`.  The coalescer would
then attempt to recolor the counter to A, breaking DJNZ.

**Fix:** `addEdgesAt(t.Body, t.BodyArgs, 1)` with a new helper that takes `paramOffset`.

### Bug C — Function contract params merged with internal block params

`OptimizeContracts` decides each function parameter's physical register (ABI).
`PreallocCoalesce` was merging these with loop body block params.  After merging,
PBQP was free to re-assign the ABI parameter to any location — including B — producing
a calling-convention mismatch visible to callers.

**Example:** `sum_array(ptr=HL, n=C)` — without the guard, `n` ended up assigned to
`IXH` (an index register) because it merged with a block param that PBQP wanted there.

**Fix:** `contractParamRegs` map: skip any arg that is a function contract param.

```go
contractParamRegs := make(map[Reg]bool, len(f.Contract.Params))
for _, cp := range f.Contract.Params {
    contractParamRegs[cp.Reg] = true
}
// In mergeEdgeAt:
if contractParamRegs[arg] {
    continue
}
```

### Bug D — classOf missing f.Contract.Params scan

`classOf(r)` in `PreallocCoalesce` scanned block params and instruction dsts but not
function contract params.  Function parameters live only in `f.Contract.Params`, not
in `f.Blocks[0].Params`.  All function params returned `ClassGeneral` (zero value),
making the class-compatibility check unreliable.

**Fix:** scan `f.Contract.Params` first.

### Bug E — coalesceAllocResult recoloring ignores register class

The post-allocation coalescer recolored `dst` to `src`'s physical location without
checking whether `srcLoc` is a legal home for `dst`'s register class.  A
`ClassCounter` register (must be B for DJNZ) could be recolored to A.

**Fix:** gate recoloring on `ct.Cost(dstCls, srcLoc) < InfCost`.

```go
if dstCls, ok := regClass[e.dst]; ok {
    if ct.Cost(dstCls, srcLoc) >= InfCost {
        continue
    }
}
```

---

## The BrIf ClassAcc Trap — Why Conditional Edges Are Skipped

During debugging, merging across `TermBrIf` edges revealed a subtle interaction:

1. The BrIf source block computes a zero-check condition as `%r5 = AND(n, n)`.
   `AND` always writes to A on Z80 → `%r5` is `ClassAcc`.

2. The acc-init constant `%r2 = OpConst(0)` sits in the same block and feeds the
   body's accumulator param via `ThenArgs`.  After merging, the merged root is promoted
   to `ClassAcc` by `propagateClassHints` (it flows to the return value).

3. Both `%r5` and the merged `%r2` are `ClassAcc` = A only.  The interference graph
   **does** detect them as co-live (BuildInterferenceGraph's `addTermUses` puts both in
   the live set at the terminator).  PBQP must spill one.

4. The spill-then-recolor interaction produced incorrect codegen: `LD A, 0` (acc-init)
   immediately followed by `LD A, B; AND A` (condition), clobbering the initialised
   accumulator before the loop starts.

**Decision:** merge only across unconditional forward jumps.  `TermJmp` and
`TermDJNZ.Exit` have no condition computation — no ClassAcc instructions sit between
the arg's definition and the branch, so no A-register clobber is possible.

`TermBrIf`, `TermBrIf2`, `TermCondRet`, and `TermDJNZ.Body` (back-edge) are skipped.
Post-allocation `coalesceAllocResult` affinity coalescing still handles many of these
cases once all registers have stable physical locations.

---

## Concrete Examples

### sum_range — DJNZ exit coalescing

```nanz
fun sum_range(n: u8) -> u8 {
    return range(0..n).fold(0, |acc, i| acc + i)
}
```

The exit block accumulator param is now coalesced with the loop-body ADD result via
the DJNZ exit edge (`TermJmp`-equivalent).  No parallel copy at exit.

```z80
; fun sum_range(n: u8 = A) -> u8 = A ; clobbers: B, C, F
sum_range:
    LD B, 0
    AND A
    JRS NZ, .sum_range_trmp0
    JRS .sum_range_trmp1
.sum_range_rng_body1:
    ADD A, B
    DJNZ .sum_range_rng_body1
.sum_range_rng_exit2:
    RET
.sum_range_trmp0:
    LD B, A        ; move n → counter (B) — parallel copy entry→body
    LD A, 0        ; acc_init = 0
    JRS .sum_range_rng_body1
.sum_range_trmp1:
    LD A, 0        ; acc_init = 0 (n=0 path)
    JRS .sum_range_rng_exit2
```

The hot path (body→body back-edge): `ADD A, B; DJNZ` — **2 instructions, 0 copies**.
The exit edge (body→exit): accumulator stays in A — **0 copies**.

n=A satisfies the caller's `LD A, n; CALL sum_range` convention.

### sum_array — contract params correctly isolated

```nanz
fun sum_array(ptr: ptr, n: u8) -> u8 {
    var acc: u8 = 0
    while n > 0 {
        acc = acc + ptr[0]
        ptr = ptr + 1
        n = n - 1
    }
    return acc
}
```

`ptr=HL` and `n=C` are ABI-decided by `OptimizeContracts`.  The `contractParamRegs`
guard prevents PreallocCoalesce from merging these with loop body params, preserving
the calling convention.

```z80
; fun sum_array(ptr: ptr = HL, n: u8 = C) -> u8 = A ; clobbers: BC, D, DE, F, IXH, IXL
```

Before this fix, `n` was being promoted to `IXH` (index register) — an unnecessary
11T load cost at every call site.

### forEach sum_chain — DJNZ body tight loop preserved

```z80
; fun sum_chain(buf: ptr = HL, n: u8 = C) -> u8 = A ; clobbers: B, F
sum_chain:
    LD A, 0
    LD B, C
.sum_chain_fe_head1:
    LD E, A
    LD A, B
    AND A
    LD A, E    ; restore block param from scratch
    JRS Z, .sum_chain_fe_exit4
.sum_chain_fe_body2:
    LD C, (HL)
    ADD A, C
.sum_chain_fe_cont3:
    INC HL
    DJNZ .sum_chain_fe_body2
.sum_chain_fe_exit4:
    RET
```

The DJNZ body loop body is 3 instructions: `LD C, (HL); ADD A, C; INC HL` plus the
`DJNZ` branch.  The exit edge (body→exit) has zero parallel copy — the result stays
in A directly.

### GCD — BrIf2 unchanged (future work)

GCD uses a three-way `BrIf2`.  BrIf2 merges are intentionally skipped in this phase.
The coalesceAllocResult affinity pass handles some of the copies post-allocation, but
a `LD E, A / LD A, E` round-trip at the loop head remains:

```z80
; fun gcd(a: u8 = A, b: u8 = C) -> u8 = A ; clobbers: F
gcd:
.gcd_loop_head1:
    LD E, A            ← save A into scratch E
    LD A, E            ← load A back from E (redundant after coalescing)
    CP C               ← compare
    LD A, E            ; restore block param from scratch
    JRS Z, .gcd_loop_exit3
```

The `LD E, A; LD A, E` pair is the pre-existing BUG-001 copy bloat that safe BrIf2
merging would eliminate.  This remains open for Phase 6h.

---

## Pipeline Change

Two allocation loops in `pipeline.go` now call `PreallocCoalesce` before liveness:

```go
for _, f := range m.Funcs {
    mir2.PreallocCoalesce(f)  // BUG-001: union block-arg/param pairs before PBQP
    lr := mir2.ComputeLiveness(f)
    ar := mir2.PBQPAllocate(f, lr, ct)
    for r, loc := range ar.Locs {
        combined.Locs[r] = loc
    }
}
```

---

## Test Results

```
26/26 pkg packages: PASS (go test ./pkg/... -vet=off)
sum_range E2E:   PASS (n=A, correct accumulator, all 5 cases 0/1/4/5/10)
sum_array E2E:   PASS (ptr=HL, n=C, ABI preserved)
forEach E2E:     PASS (11/11 cases)
range fold E2E:  PASS (range(n..0), range(0..n), range(3..8))
```

---

## What Remains (BUG-001 Phase 2)

| Scenario | Status | Blocker |
|----------|--------|---------|
| DJNZ exit edge merge | ✅ Active | — |
| TermJmp merge | ✅ Active | — |
| BrIf / BrIf2 merge | ⏸ Deferred | ClassAcc/AND conflict in source block |
| DJNZ back-edge merge | ⏸ Deferred | Dual-def breaks liveness analysis |

BrIf merging requires either intra-block liveness precision (to correctly detect
interference between acc-init and condition-compute registers) or a smarter
class-upgrade policy that avoids promoting constant-init registers to ClassAcc when a
condition computation also occupies A in the same block.  Filed as Phase 6h.

---

*Files changed: `pkg/pipeline/pipeline.go`, `pkg/mir2/precoalesce.go`, `pkg/mir2/coalesce.go`*
*See also: Report #066 (inliner), `docs/Open_Bugs_RCA.md` (BUG-001)*
