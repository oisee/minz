# Loop Header Block-Arg Materialization Fix

**Date:** 2026-04-07
**Scope:** VIR cfgsolver / bridge / pipeline -- missing edge moves for loop header block arguments.

---

## Bug Description

The VIR (Z3-based) codegen path failed to emit correct assembly for functions
containing counted loops with parameter-based bounds.  Three independent bugs
combined to produce the failure:

### Bug 1: Dead constant elimination removes terminator-arg vregs

`EliminateDeadConsts` in `bridge.go` removes `OpConst` instructions whose dst
vreg is "unused".  It determines "used" by scanning `op.Src` within the same
block.  However, a vreg used only as a `TermJmp.Args` / `TermBrIf.ThenArgs`
block argument never appears as a Src in any VIR op, so its defining `OpConst`
was eliminated.

**Effect:** `i = 0` (counter initialization) was removed from the entry block.
The loop header received an uninitialized register as the counter.

### Bug 2: Parameter liveness not injected for term-arg-only params

The `paramUsedLater` set in `cfgsolver.go` only checked if a function parameter
vreg appeared as a `Src` in VIR ops of non-entry blocks.  When a parameter is
consumed exclusively via terminator block-arguments (e.g., `TermJmp{Args: [idx,
p, i]}` where `idx` is never used inside any VIR op but is forwarded to the
loop header as a block arg), it was not found by the Src scan.

**Effect:** No Z3 variable was created for the parameter at instruction 0 of the
entry block, so the param-location constraint was never applied and the edge
move from the ABI register to the solver-chosen register was never emitted.

Additionally, the param-pinning code only pinned params at the first VIR op
that references them.  Params consumed only via terminator args were never
pinned, so Z3 was free to place them in any register -- ignoring the ABI.

### Bug 3: Table-based emit bypasses edge moves for multi-block functions

`emitFromTable` in `pipeline.go` flattens all blocks into a single op sequence
and emits PIR ops with the GPU-table assignment.  It does not emit edge moves,
PHI moves, or intra-block moves between blocks.  For single-block functions
this is correct; for multi-block functions it silently drops all cross-block
register transfers.

**Effect:** Simple multi-block functions (e.g., `count_to` with 4 blocks, ~4
vregs) matched the enriched table and were emitted without any edge moves,
producing broken assembly.

---

## What Was Changed

### 1. `minzc/pkg/vir/bridge.go` -- EliminateDeadConsts

Before calling `EliminateDeadConsts`, the code now extracts all vreg IDs from
the block's terminator arguments (`TermJmp.Args`, `TermBrIf.ThenArgs/ElseArgs`,
`TermBrIf2.EqArgs/LtArgs/GtArgs`, `TermDJNZ.BodyArgs/ExitArgs`) and passes
them as extra "used" vregs to the elimination function.

`EliminateDeadConsts` signature changed from `func(ops []VIROp) []VIROp` to
`func(ops []VIROp, extraUsed ...int) []VIROp` (backward-compatible variadic).

### 2. `minzc/pkg/vir/cfgsolver.go` -- paramUsedLater + param pinning fallback

`paramUsedLater` now also scans terminator args across all MIR2 blocks (not
just VIR Src references) to detect params consumed via block arguments.

Added fallback: params that are never referenced in any VIR op (only consumed
as terminator args) are pinned at instruction 0 of block 0 to their ABI
physical register.  This ensures the Z3 solver knows where the param starts
and can emit the necessary intra-block move before it gets clobbered.

### 3. `minzc/pkg/vir/pipeline.go` -- Skip table emit for multi-block functions

`emitFromTable` is now skipped when `len(vf.Blocks) > 1`.  Multi-block
functions fall through to the Z3 CFG solver which properly handles edge moves
and PHI moves.  The cut-vertex decomposition path (which also uses
`emitFromTable`) is similarly skipped for multi-block functions.

---

## Test Results

Two regression tests added to `minzc/pkg/vir/assert_test.go`:

| Test | Description | Status |
|------|-------------|--------|
| `TestVIR_Assert_LookupLoopHeader` | Standalone `lookup(idx)` -- pointer walk with param bound | PASS |
| `TestVIR_Assert_CountedLoop` | `count_to(n)` -- counted loop returning counter | PASS |

All pre-existing passing tests remain passing (no regressions).  Pre-existing
failures (`Arithmetic`, `Bitwise`, `ReturnABI`, `NestedCalls`, `MultiReturn`,
`ChainedCalls`, `GCD`) are unchanged and unrelated to this fix.

---

## Commands to Reproduce

```bash
cd minzc

# Run both new regression tests:
go test ./pkg/vir/ -run "TestVIR_Assert_LookupLoopHeader|TestVIR_Assert_CountedLoop" -v -timeout 120s

# Run all assert tests to verify no regressions:
go test ./pkg/vir/ -run "TestVIR_Assert_" -v -timeout 300s

# Debug edge moves for a specific function:
VIR_DEBUG_EDGES=1 go test ./pkg/vir/ -run "TestVIR_Assert_LookupLoopHeader" -v -timeout 120s 2>&1 | grep -E "EDGE|PHI|INTRA"
```

---

## Files Modified

| File | What |
|------|------|
| `minzc/pkg/vir/bridge.go` | Extract terminator args as extra used vregs for dead const elimination |
| `minzc/pkg/vir/cfgsolver.go` | Scan terminator args for param liveness; pin term-arg-only params at entry |
| `minzc/pkg/vir/pipeline.go` | Skip table/cut-vertex emit for multi-block functions |
| `minzc/pkg/vir/assert_test.go` | Two regression tests |
