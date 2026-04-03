# Solver-Friendly Shapes for Nanz and VIR

Date: 2026-04-03
Author: Codex

## Goal

This note describes which source shapes are easier for the current Nanz → MIR2 → VIR → Z80 pipeline to solve well, and how to reshape code when a function compiles only through PBQP fallback or hits Z3 timeouts.

The point is not to write “worse high-level code”. The point is to keep the source readable while avoiding shapes that inflate live ranges, mix too many width domains, or force the solver to reason about too many coupled choices at once.

## Short Rule

When a hot function is unstable or keeps falling back, prefer:

- smaller functions
- fewer simultaneously-live values
- pointer walking instead of repeated indexed address recomputation
- LUTs for constant address terms and masks
- one width domain per loop when possible
- helper boundaries around nested loops and address formulas

## What Makes A Shape Solver-Friendly

### 1. Narrow Live Ranges

Good:

- compute value, use it, drop it
- load scalar parameters once into locals
- move repeated subexpressions into helpers

Bad:

- several loop counters live together with pointer bases, masks, temporary wide values, and branch results
- carrying both byte and word intermediates across long loop bodies

Why it matters:

- fewer simultaneously-live vregs means smaller interference graphs
- smaller graphs give the table path, Z3 path, and island decomposition much more room to succeed

### 2. Pointer Walks Beat Recomputed Address Expressions

Good:

- `p = p + 1`
- `row_ptr = row_ptr + 32`
- `px = px + blk`

Bad:

- recomputing `base + by * 32 + bx` inside the inner loop
- rebuilding `0xC000 + idx` every iteration when a walking pointer would do

Why it matters:

- repeated address arithmetic introduces more wide ops
- wide ops increase coupling between register-pair allocation and surrounding byte logic

### 3. Split Nested Loops Across Helpers

Good:

- outer loop in one function
- row helper for inner loop
- pixel/block helper for the innermost work

Bad:

- one large function that owns all counters, all address terms, all conditions, and all calls

Why it matters:

- nested loops multiply liveness pressure quickly
- helper boundaries give the solver smaller independent problems

### 4. Prefer LUTs Over Repeated Constant Arithmetic

Good:

- `mask = masks[n]`
- `addr = base_tbl[a] + off_tbl[b] + x`

Bad:

- rebuilding `(1 << n) - 1` in source loops
- repeated `row * 256`, `col * 32`, `third * 2048`

Why it matters:

- constant-multiply lowering is much better than it used to be, but LUTs still reduce solver work
- LUTs also keep source intent clearer when the values are static

### 5. Keep A-Only Operations Local

The Z80 still has strong accumulator bias.

Good:

- let accumulator-oriented logic happen in a short local region
- use direct non-`A` forms only where they are genuinely good (`BIT`, `SET`, `RES`, IX/IY halves)

Bad:

- forcing too many values to compete for `A` across a long mixed loop

Why it matters:

- the cost model now treats `A` vs non-`A` bit updates more honestly
- but source can still make life easier by not coupling unrelated accumulator-shaped work

### 6. Separate Byte Loops From Wide Setup

Good:

- wide address base computed once
- byte loop walks from there

Bad:

- every iteration mixes `u16` address building with `u8` per-pixel logic

Why it matters:

- the solver handles regular byte loops better than loops that constantly cross 8/16-bit boundaries

## Practical Refactoring Patterns

### Pattern A: Indexed Buffer Fill → Pointer Walk

Instead of:

- `p = base + idx`
- `idx = idx + 1`

prefer:

- `p = base`
- `p = p + 1`
- `remaining = remaining - 1`

### Pattern B: Nested Grid Loop → Row Helper

Instead of:

- outer `by`
- inner `bx`
- repeated `base + by * stride + bx`

prefer:

- row pointer passed into a row helper
- inner helper only advances one pointer and one x-coordinate accumulator

### Pattern C: Screen Address Formula → Small Helper + LUT Terms

Instead of:

- one function that computes `third * 2048 + prow * 256 + crow * 32 + xbyte`

prefer:

- row-address helper
- small static tables for the fixed stride terms
- final byte offset added at the end

### Pattern D: Table Loads + Work In Same Function → Split Boundary

Instead of:

- one function doing table indexing, state setup, warmup, fill, and draw

prefer:

- caller loads per-layer parameters from tables
- worker function operates only on scalar arguments

Why:

- table indexing and loop-heavy rendering stress different parts of the pipeline

## Applying This To `che_cascade.nanz`

The main hot spots are:

- `fill_buf`
- `xor_pixel`
- `xor_blk`
- `apply_buf`
- the old table-heavy `process_layer`

The better shape is:

1. `fill_buf` as a pointer walk over `0xC000`
2. `xor_blk` split into row helper + outer row loop
3. `apply_buf` split into row helper + outer row loop
4. ZX screen address arithmetic split so the hot pixel path is smaller
5. table loads moved to the caller side, with a scalar `process_seed(...)` worker

This does not guarantee zero fallback. But it moves the code toward smaller, more regular constraint problems, which is exactly what the current solver stack wants.

## One-Line Heuristic

If a function feels like “control flow + indexing + address math + calls + nested loops” all at once, it is probably too much function for the current solver. Split it until each piece mostly does one kind of work.
