# Backend Sprint Progress Report

Date: 2026-04-03

## Scope

This report summarizes the recent backend sprint around:

- MIR2/Grace solver-friendly shaping
- VIR/Z80 backend intent preservation
- IX/IY-half-aware codegen quality
- Che-gap reduction groundwork

## What Landed

### 1. Bit intent now survives through the pipeline

Implemented and validated:

- scalar bit selector syntax: `x.N`
- pointer-backed bit access: `ptr^.N`
- wide-scalar bit access for `u16` and scalar-width-based lowering
- direct MIR2 bit ops:
  - `bit_get`
  - `bit_set`
  - `bit_reset`
- VIR support for direct bit intent
- Z80-native lowering to:
  - `BIT`
  - `SET`
  - `RES`

This moved bit manipulation from pattern-recovery-only territory into an
intent-preserving path across parsing, lowering, MIR2/VIR, and backend codegen.

### 2. IX/IY halves are treated as first-class citizens

Backend policy and codegen now consistently treat:

- `IXH`
- `IXL`
- `IYH`
- `IYL`

as normal 8-bit registers, not special-case afterthoughts.

This already improved:

- bit operations on pair halves
- register-backed byte paths
- compare and move handling
- general backend reasoning for `IX/IY`-resident values

### 3. Typed globals and split `u16` LUT layout

Improved static data emission and LUT lowering:

- typed global array emission
- `u16` LUT split into `lo/hi` byte tables
- layout more natural for Z80 access patterns
- less pressure to synthesize `idx * 2` in hot lookup code

### 4. Grace shape-fact groundwork exists

Added MIR2 shape fact collection for:

- loop regions
- indexed accesses
- repeated address terms
- in-loop candidates

This is the analysis substrate for structural Grace rewrites instead of only
local peepholes.

### 5. First real structural Grace rewrites landed

Two new MIR2 shape-driven passes are now wired into `RunGracePasses`:

- `ptr-threading`
  - loop-carried walking pointer through block args
- `ptr-add-cse`
  - local dedup of repeated `ptr_add`

These are no longer isolated experiments. They are part of the active Grace
path and visible in trace output.

### 6. Compile-trace observability improved

Per-function trace annotations now include:

- `ptr-threading`
- `ptr-add-cse`

This matters because we can now distinguish:

- a pass that exists but never fires
- a pass that fires but gives no visible benefit
- a pass that does not match a real program shape

## End-to-End Proofs

### `che_cascade.nanz`

Current state:

- compiles through the active VIR path
- trace visibility works
- pointer-threading / ptr-add-cse do not currently fire on its draw-path loops

This is important. We now know the problem is not pass invisibility but real
program shape mismatch.

### `pointer_threading.nanz`

Added as a small real-source proof case.

Result:

- compiles through `--grace --compile-trace`
- trace shows `ptr-threading=1`
- proves that the new pass can fire on actual Nanz source, not only synthetic
  MIR2 tests

This closes the “does this ever hit real code?” question for the first time.

## What We Learned

### Good news

- intent-preserving lowering is paying off
- IX/IY-half-aware backend work is worthwhile
- Grace structural rewrites can be made real, not hypothetical
- trace visibility was a necessary missing layer and is now in place

### Honest limits

- `che_cascade` still does not fall into the new pointer-threading shapes
- the main remaining bottleneck is loop/body structure, not pass existence
- current wins are foundational and real, but not yet the final Che-gap
  reduction

## Current Best Next Steps

Recommended order from here:

1. `B2` row-helper extraction / shape reshaping for `che_cascade` draw-path loops
2. `B3` wire `ApplyVIRDSE` more aggressively in the default path
3. `B4` move closer to full `RunGracePasses` default usage
4. `B5` loop-role-aware allocation economics
5. `B6` enriched-table Path A exploration as a parallel experimental track

## Bottom Line

The backend sprint has produced real infrastructure and real wins:

- bit intent now survives to backend selection
- IX/IY halves are modeled correctly
- shape facts exist
- structural Grace passes are wired
- trace visibility exists
- pointer-threading has a real source-level proof case

What is not done yet is the final Che-gap reduction on `che_cascade`.

The project is now past “backend ideas” and into “measured shaping work on
real programs”.
