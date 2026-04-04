# Multi-Block Pointer-Threading Design Note

**Date:** 2026-04-03
**Scope:** Extending pointer-threading / Grace reshaping from single-block to multi-block loop shapes
**Constraint:** No EXX, no frontend changes, no giant compiler rewrite

---

## Current State

Single-block pointer-threading works today via manual source-level reshaping
(che_cascade report: hoist row_ptr, pointer-walk fill_buf). Grace handles
2-3 block patterns (`bounded-loop-unroll`, `cond-ret-sink`, `redundant-load`)
but has **no loop-aware pointer optimization**.

What exists:
- Block arguments (SSA-style, not phi) with typed params and edge args
- Grace matcher: single/two/N-block dispatch with edge connectivity checks
- Backward liveness dataflow with block-arg semantics
- RPO traversal, layout optimization
- 16 Grace rules, all operating on CFG topology + per-block predicates

What does NOT exist:
- Natural loop extraction (header/latch/exit identification)
- Induction variable analysis
- Pointer-increment detection across blocks
- Dominance frontiers (only simplified dominance check)

---

## Target Loop Shapes

### Shape A: Simple counted loop (2 blocks)

```
@header(%ptr: u16, %count: u8):
  %done = cmp.eq %count, 0
  br_if %done, @exit(), @body(%ptr, %count)

@body(%p: u16, %n: u8):
  ...use %p as pointer...
  %p_next = add %p, stride        ← pointer increment
  %n_next = sub %n, 1             ← counter decrement
  jump @header(%p_next, %n_next)  ← back-edge with updated args
```

**Status:** Grace can already match this with a 2-block pattern (like `bounded-loop-unroll`).
**Opportunity:** Detect when `%p` originates from address arithmetic in a pre-header
and thread the pointer walk through block args.

### Shape B: Header → body → latch (3 blocks)

```
@header(%ptr: u16, %idx: u16):
  %done = cmp.ge %idx, limit
  br_if %done, @exit(), @body(%ptr, %idx)

@body(%p: u16, %i: u16):
  ...load from %p, conditional work...
  jump @latch(%p, %i)

@latch(%p2: u16, %i2: u16):
  %p_next = add %p2, stride
  %i_next = add %i2, 1
  jump @header(%p_next, %i_next)
```

**Why 3 blocks?** The compiler splits body from latch when there's a conditional
branch inside the body (e.g., `if p^ != 0 { xor_blk(...) }`). This is the
`apply_buf` pattern: header tests row bound, body tests cell, latch increments.

### Shape C: Header → body with interior branch → rejoin → latch (4 blocks)

```
@header(%ptr: u16, %bx: u8, %by: u8):
  %done = cmp.ge %by, 24
  br_if %done, @exit(), @inner_header(%ptr, %bx, %by)

@inner_header(%p: u16, %x: u8, %y: u8):
  %col_done = cmp.ge %x, 32
  br_if %col_done, @row_latch(%p, %y), @cell(%p, %x, %y)

@cell(%p2: u16, %x2: u8, %y2: u8):
  %val = load %p2
  ...conditional call to xor_blk...
  %x_next = add %x2, 1
  %p_next = add %p2, 1
  jump @inner_header(%p_next, %x_next, %y2)

@row_latch(%p3: u16, %y3: u8):
  %y_next = add %y3, 1
  jump @header(%p3, 0, %y_next)    ← bx resets, ptr already advanced
```

**This is the `apply_buf` pattern after lowering.** The inner loop iterates
columns with pointer walk, the outer loop iterates rows. The pointer is
threaded through block args across 4 blocks.

### Priority order: Shape A first, then B, then C.

Shape A covers `fill_buf` and simple loops. Shape B covers `xor_blk`-like
nested loops. Shape C covers `apply_buf`. Each builds on the previous.

---

## Minimal Extra Facts Needed

The Grace engine needs 3 new facts to match pointer-threading opportunities:

### 1. Back-edge detection (required)

```go
// New predicate: is the edge from ?body to ?header a back-edge?
// Simple check: ?header appears in RPO before ?body.
reg.RegisterPredicate("is-back-edge", func(bindings, g) bool {
    bodyIdx := rpoIndex[bindings["body"]]
    headerIdx := rpoIndex[bindings["header"]]
    return headerIdx <= bodyIdx
})
```

This is the minimum needed to distinguish loops from forward branches.
RPO already exists in `liveness.go:404-440`.

### 2. Block-arg stride detection (required)

```go
// New predicate: does block arg %ptr at position N get incremented
// by a constant stride on the back-edge?
// Checks: back-edge arg at position N = add(param_N, const_stride)
reg.RegisterPredicate("has-stride-arg", func(bindings, g, paramIdx, stride) bool {
    body := g.Block(bindings["body"])
    header := g.Block(bindings["header"])
    // Find the jump/br_if terminator targeting header
    // Check that arg[paramIdx] = add(header.Params[paramIdx].Dst, stride)
})
```

### 3. Address-origin detection (optional, for automation)

```go
// New predicate: is block arg %ptr at the loop pre-header computed
// as base + index * stride (the pattern we want to replace)?
reg.RegisterPredicate("is-address-computation", func(bindings, g, paramIdx) bool {
    // Walk def chain of the initial arg value
    // Check for add(base, mul(index, stride)) or add(base, index)
})
```

This third fact enables automatic detection. Without it, the programmer must
manually restructure to use pointer args (which is already effective).

---

## How Block Args Would Be Threaded Safely

Block args are the natural vehicle for pointer-threading because they already
model value flow across block boundaries without phi-node complications.

### Threading protocol:

1. **Pre-header:** Compute initial pointer value, pass as block arg to header.
   ```
   @pre_header:
     %base = add 0xC000, %row_offset
     jump @header(%base, 0)
   ```

2. **Header:** Receive pointer as formal parameter. Test loop condition.
   Forward pointer to body unchanged.
   ```
   @header(%ptr: u16, %count: u8):
     br_if %done, @exit(), @body(%ptr, %count)
   ```

3. **Body/latch:** Use pointer, increment, pass back via back-edge args.
   ```
   @body(%p: u16, %n: u8):
     ...use %p...
     %p_next = add %p, 1
     %n_next = sub %n, 1
     jump @header(%p_next, %n_next)
   ```

**Safety invariant:** Every path to the header must supply the same number and
type of args. The block-arg verifier (`VerifyFunc`) already checks this.
Pointer-threading adds a new arg or replaces an existing address-computation
arg — both are safe as long as edge args are updated consistently.

### Replacing address recomputation:

Before (indexed):
```
@header(%idx: u16):
  @body: %addr = add 0xC000, %idx    ← recomputed every iteration
```

After (pointer-threaded):
```
@pre_header: %ptr0 = add 0xC000, %idx0
@header(%ptr: u16, %idx: u16):
  @body: ...use %ptr directly...     ← no recomputation
         %ptr_next = add %ptr, 1
```

The `%idx` arg can be kept (if used for bounds check) or replaced with a
countdown counter, depending on loop structure.

---

## Main Correctness Hazards

### 1. Non-unit stride with conditional skip

If the loop body conditionally skips the pointer increment (e.g., `if p^ != 0`
branches around the work but still advances), the pointer and index can
desynchronize. The pointer must advance unconditionally or the threading is wrong.

**Mitigation:** The `has-stride-arg` predicate must verify the increment is on
the *unconditional* path to the back-edge, not inside a conditional branch.

### 2. Multiple back-edges (break/continue)

If the loop has multiple paths back to the header (e.g., early continue), each
must supply the correctly incremented pointer. Missing one path = stale pointer.

**Mitigation:** Check ALL edges to the header, not just the "main" back-edge.
The block-arg verifier catches arity mismatches but not semantic correctness.

### 3. Pointer escapes via call

If the pointer is passed to a callee that modifies the pointed-to memory AND
the stride depends on memory content (e.g., variable-length records), the
pointer-walk is invalid.

**Mitigation:** For che_cascade loops, all strides are constant (1 byte per
cell, 32 bytes per row). Only apply threading when stride is a compile-time
constant.

### 4. Overflow on u16 pointer arithmetic

Pointer walks that wrap around 0xFFFF→0x0000 produce incorrect addresses.
The original indexed addressing (`base + idx`) would also wrap, so this is
not a new hazard — but it's worth noting that the transformation preserves
the overflow behavior, not fixes it.

### 5. Dead block-arg elimination interaction

After threading, the original `%idx` arg may become dead if only used for
address computation. `DeadBlockArgElim` (existing Grace rule, priority 45)
will remove it automatically. This is correct and desired — but the rule
must run AFTER pointer-threading, not before (otherwise the address
computation looks live because it's still being used).

---

## What to Test First

### Test 1: `fill_buf` pattern (Shape A, simplest)

```nanz
// Before: indexed
var idx: u16 = 0
while idx < 768 { let p: ^u8 = 0xC000 + idx; ...; idx = idx + 1 }

// After: pointer-walk (manual reshape, verify codegen equivalence)
var p: ^u8 = 0xC000
var remaining: u16 = 768
while remaining > 0 { ...; p = p + 1; remaining = remaining - 1 }
```

Verify: identical output bytes, lower T-state count (INC HL vs ADD HL,idx).

### Test 2: `apply_buf` inner loop (Shape B, nested)

The inner `bx` loop with `0xC000 + by*32 + bx` → hoisted `row_ptr + bx`.
Two levels: outer row pointer (stride 32), inner cell pointer (stride 1).

### Test 3: Grace rule matching

Write a Grace rule for Shape A and verify it fires on the MIR2 of `fill_buf`.
Don't implement the action yet — just confirm the pattern matches:

```grace
(grace detect-pointer-walk-opportunity 0
  (match
    (block ?header (term-kind "br_if"))
    (block ?body (term-kind "jump"))
    (edge ?header ?body "then")
    (edge ?body ?header "succ"))
  (where
    (is-back-edge ?body ?header)
    (has-stride-arg ?body ?header 0 1))  ;; param 0, stride 1
  (action
    (custom "logPointerWalkCandidate" ?header ?body)))
```

### Test 4: Correctness after threading

Compile both versions, run through MZE, compare output bytes.
The che_cascade renderer produces deterministic output — any mismatch = bug.

---

## Better Near-Term Bet Than EXX?

**Yes, for che_cascade-class workloads.**

| Factor | Pointer-threading | EXX shadow regs |
|--------|------------------|-----------------|
| Complexity | Low (Grace rules + block-arg threading) | High (all 7 regs swap simultaneously, bank tracking) |
| Scope | Loop bodies with address recomputation | Any high-pressure function |
| Risk | Low (mechanical, verifiable) | Medium (EXX bank confusion, interrupt safety) |
| Payoff for che_cascade | High (address arith is the main pressure source) | Medium (helps with live-across-call, but calls are cheap) |
| Payoff for general corpus | Medium (many loops have address patterns) | High (universal pressure relief) |
| Solver interaction | Reduces vreg count → fewer Z3 vars | Doubles available regs → solver handles rest |
| Implementation path | Grace rule → predicate → action → test | VIR solver extension + PIR emit + peephole |

**Recommendation:** Pointer-threading is a better near-term bet because:
1. It reduces the problem size (fewer vregs) rather than expanding the solution space (more regs).
2. It composes with the existing enriched table path (fewer vregs → more table hits).
3. It's testable incrementally (one loop shape at a time).
4. EXX is still valuable long-term but has higher blast radius and more correctness hazards.

The ideal sequence: pointer-threading first (reduces easy cases), then EXX
for the remaining high-pressure functions that can't be simplified.
