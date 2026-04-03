# Grace Solver-Friendly Reshaping Proposal

## Summary

This report captures a concrete direction for semi-automatic reshaping of Nanz/MIR2/VIR programs into more solver-friendly forms before register allocation and backend lowering. The core idea is:

- use declarative facts and statistics to detect pressure-heavy shapes,
- use Grace as the main structural rewrite engine,
- use ISLE-style local canonicalization for small tree rewrites,
- keep EXX as a later, controlled region feature rather than the next default step.

The immediate target is not "perfect code generation everywhere", but reducing cases where large or branchy functions fall into PBQP fallback or awkward whole-function solve modes when a modest structural rewrite would have made them tractable.

## Problem

Several current bottlenecks are not primarily caused by missing instructions or weak location tables. They are caused by function shape:

- repeated indexed address arithmetic in inner loops,
- too many simultaneously live wide and narrow temporaries,
- mixed setup and hot-loop work in the same region,
- branchy draw/update workers with screen-address math intertwined with byte writes,
- counters occupying hot registers when colder registers or halves would have been cheaper overall.

Even with IX/IY-half-aware allocation tables, some functions remain expensive because the CFG and live-range shape are poor.

## Guiding Principles

### 1. Shape first, local peepholes second

When the function shape is poor, allocator tables and peepholes only partially help. The first job is to make the region solver-friendly:

- smaller live sets,
- fewer mixed-width temporaries alive at once,
- clearer roles for registers,
- fewer repeated address terms inside hot loops.

### 2. Declarative detection, imperative rewriting

Datalog-like analysis is useful for discovering candidate regions and proving conditions. Grace should do the actual rewrites.

That division is cleaner than trying to make one system do everything.

### 3. Do not overuse IX/IY indexed memory when a walker is cheaper

`(IX+d)` and `(IY+d)` are valuable, but repeated bytewise traversal of consecutive memory is often cheaper as:

- `(HL)` / `INC HL`
- `(DE)` / `INC DE`

and sometimes direct byte-half register handling is better still.

### 4. Do not reach for EXX too early

Current architecture still has obvious wins available through reshaping, loop-role-aware allocation, and pointer-walk conversion. EXX should be introduced later as a controlled region mechanism, not as the first answer to pressure.

## Proposed Roles

## Grace

Grace should be the main structural transformation engine.

Good fits:

- split hot helpers out of large workers,
- convert indexed loops into pointer walks,
- hoist invariant table loads,
- separate wide setup from byte loops,
- extract row helpers from nested grid/screen loops,
- normalize repeated address formulas into shared bases plus offsets.

## ISLE-style local combining

Useful after structural transforms for local cleanup:

- fold small address expressions,
- normalize increment/test forms,
- canonicalize bit-intent patterns,
- clean up artifacts left by helper extraction or pointer-walk conversion.

This is a local tree-shape tool, not the main region-rewrite engine.

## Datalog / declarative facts

Useful as an analysis and trigger-selection layer:

- identify repeated address arithmetic,
- detect indexed inner loops,
- find loop-invariant table loads,
- mark likely counters and walkers,
- estimate pressure motifs,
- expose profitability preconditions to Grace.

The right model is:

- Datalog derives facts,
- Grace consumes facts,
- backend and solver exploit the improved shape.

## Candidate Facts

The following facts would be high value:

- `inner_loop(block)`
- `indexed_access(base, idx, stride, block)`
- `repeated_addr_term(term, block, count)`
- `loop_invariant(value, loop)`
- `candidate_counter(value, loop)`
- `candidate_walker(ptr, loop)`
- `mixed_width_pressure(loop)`
- `screen_row_formula(loop)`
- `single_use_temp(value)`
- `hot_branchy_worker(fn)`
- `byte_loop_with_wide_setup(loop)`

These facts do not need to be perfect initially. Even conservative detection would already help.

## Quick Wins

These can fit current architecture with relatively low risk.

### 1. Loop-aware counter placement

Do not treat loop-counter cost as a single-instruction local decision.

Instead compare:

- local cost of `DJNZ`, `DEC reg/JR NZ`, or equivalent,
- amortized value of freeing `B`,
- value of freeing general hot registers in the loop body.

This means counters may legitimately prefer:

- `B` when `DJNZ` is clearly best,
- `IXL/IYL/IXH/IYH` when freeing `B` or another hot register is worth it,
- only later and more cautiously, shadow-bank placement in controlled EXX zones.

### 2. Indexed loop -> pointer walk conversion

Rewrite shapes like:

- `base + idx`
- `base + row*stride + col`
- `field_base + 0/1/2/...`

into walking-pointer forms when accesses are consecutive or affine.

This is one of the strongest immediate wins for both pressure and code quality.

### 3. Hoist repeated address subterms

If a hot loop recomputes row-base or other stable address pieces repeatedly, split them into:

- precomputed base,
- cheap offset,
- optional helper.

### 4. Separate wide setup from byte loop

Compute wide, branchy, or table-heavy setup once outside the hot loop, then run a smaller byte-oriented worker.

### 5. Better A-vs-non-A economics

Continue the current direction:

- do not force `SET/RES` on `A`,
- treat non-`A` GPRs and IX/IY halves as first-class direct destinations,
- let cost rather than habit choose the path.

## Mid Wins

These need some new infra but remain realistic within current architecture.

### 1. Grace fed by fact tables

Build a fact extraction pass over MIR2 or early VIR, then let Grace apply rewrites only when conditions are met.

This is the cleanest path to semi-automatic solver-friendly shaping.

### 2. Row-helper extraction

Nested grid/screen code often improves dramatically when rewritten into:

- outer row/base computation,
- inner row helper over a smaller byte loop.

This is especially relevant for ZX-like screen layouts.

### 3. Walker register preference

If Grace creates a pointer walk, allocator/backend should recognize the role and prefer:

- `HL` for general memory walking when destination is not forced into `A`,
- `DE` when dataflow or copy shape makes it cheaper,
- `IX/IY` when stable base plus displacement is genuinely better than sequential walk.

### 4. Osize / Ospeed split for repeated screen math

For row-base helpers and screen address forms:

- `Osize`: prefer runtime calc unless LUT clearly compresses,
- `Ospeed`: prefer LUT or precomputed row-base terms when hot.

This should be explicit policy, not accidental behavior.

## Long Wins

### 1. EXX zones

EXX is powerful, but it should be introduced only as a controlled region feature:

- clearly delimited hot kernels,
- explicit save/restore or non-call boundaries,
- region-local allocation model.

It is not yet the first thing to do globally.

### 2. Shadow-aware loop kernels

Once EXX zones exist, some counters, walkers, or temporaries may live profitably in shadow state, but only with strong region discipline.

### 3. Profile-guided or heuristic hotness-driven reshaping

Later, shaping decisions could use hotness estimates or profile data to decide whether a region should pay code-size cost for speed.

## Pointer-Walk Policy

There are two different questions:

### Structural level

Should the program be rewritten from indexed access to pointer walk?

Often yes, when accesses are consecutive or affine and alias conditions are clear.

### Backend level

Once we have a walk, which addressing strategy should be used?

Candidate policy:

- prefer `(HL)` / `INC HL` when not constrained otherwise,
- prefer `(DE)` / `INC DE` when copy/dataflow shape aligns better there,
- use `(IX+d)` / `(IY+d)` when stable-base addressing is more profitable than walking,
- do not repeatedly pay indexed-displacement cost when a simple walk would do.

## Concrete Pass Order

Recommended pipeline:

1. Early MIR2 or Grace fact extraction
2. Declarative fact derivation / profitability tagging
3. Grace structural transforms
4. Local canonicalization / ISLE-style cleanup
5. Normal VIR lowering and solving
6. Backend cost-sensitive selection

This keeps structural decisions early and instruction-selection decisions late.

## Concrete Next Steps

### Quick wins

1. Add fact extraction for:
   - indexed inner loops
   - repeated address terms
   - loop-invariant table loads
   - candidate counters and walkers
2. Add one Grace transform:
   - indexed affine loop -> pointer walk
3. Add one Grace transform:
   - nested row/column loop -> row helper extraction
4. Add loop-counter role preference:
   - `B` when `DJNZ` clearly wins
   - otherwise allow cold IX/IY halves

### Mid wins

1. Add Osize/Ospeed profitability split to row-base/LUT decisions
2. Add walker-role hints to allocator/backend
3. Add branchy-worker decomposition rules

### Long wins

1. Prototype EXX as a region-only mechanism
2. Measure on a small set of hot kernels before broader adoption

## Recommendation

The next best investment is not global EXX. It is a solver-friendly reshaping layer driven by facts and implemented by Grace.

That path preserves current architecture, uses existing strengths, and should produce visible wins on exactly the class of functions that still degrade into fallback today.
