# Apr 2 Night Handoff

Date: 2026-04-02

Scope: what actually landed today, why it matters, and the execution plan for
the rest of the night.

## What We Achieved Today

### 1. Wired one real VIR quality regression instead of adding more theory

We fixed the tied-op/pre-tie path so an inlined self-add keeps its intended
shape.

Concrete visible result:

- `x + x` can now stay `ADD A, A`
- the import/inlining fixture no longer degrades into `LD L, A; ADD A, L`

Why this matters:

- it validates the Apr 1 audit thesis that the best wins are often already
  present in the pipeline, just slightly miswired
- it preserves a high-value fast path with a small, local solver change

### 2. Added whole-module dead-function pruning after inlining

We now prune unreachable functions after MIR2 inlining and expose the result in
the assembly summary.

Concrete visible result:

- dead imported helpers no longer survive just because they existed before
  inlining
- summaries can show which functions were pruned

Why this matters:

- output now looks more like a whole-program compiler result
- optimization is no longer only “happening internally”; users can see it

### 3. Extended address-of onto the already-existing addressable-lvalue path

Today’s language/frontend win was not a new pointer model. It was exposing the
path that should have existed already.

Concrete visible result:

- `&obj.field` now parses, lowers, and compiles
- `&self.field` is supported through the same machinery
- `&arr[i]` is verified to compile and dereference correctly

Why this matters:

- this unlocks practical field/element pointer code without adding a larger
  feature surface
- it gives us a productive low-risk lane for more tests and examples

### 4. Tightened trust and documentation around verified facts

We turned today’s fact-check work into executable coverage and visible docs.

Concrete visible result:

- expanded fact-check tests
- new `fun/` examples for field pointers and indexed element pointers
- ADR-0042 for intended `let` vs `var` binding mutability
- refreshed `fun/README.md` so the new examples are the fast path for readers

### 5. Clarified one important non-goal

We explicitly did **not** promote `offsetOf/offsetof` into a built-in language
primitive today.

That is deliberate, not an omission:

- the compiler already computes field offsets internally
- but today’s productive path was exposing addressable storage directly
- this keeps the language from over-committing to physical-layout details too
  early

## What Pattern Worked

The Apr 1 reports were useful mainly as selection pressure:

- pick half-wired things
- pick local fixes
- pick user-visible wins
- avoid global activation work in the quick-win lane

Today validated that approach directly.

## Plan For Tonight

### Primary target

Expand VIR regression coverage around tied/immediate patterns adjacent to the
`ADD A, A` fix.

Why this should be next:

- it stays in the same local, high-signal lane as today’s solver fix
- it reduces the chance that nearby patterns still lose shape
- it gives us a better floor before any future table/intrinsic/backend
  activation work

### Concrete execution order

1. Add focused regression tests first for nearby tied/immediate cases.
2. Check whether `INC`, `DEC`, or `ADD HL, HL`-style patterns still degrade in
   pre-tie handling.
3. Fix only the smallest local path needed.
4. Re-run targeted tests and keep the change bounded.

### Secondary target

If the VIR regression slice lands cleanly, continue with one of:

- one more addressable-lvalue coverage case, preferably `&self.field`
- one small pruning-visibility test improvement
- one more clearly local backend fast path from the Apr 1 audit

### Explicitly not tonight’s first move

- EXX activation
- broad intrinsic/table-family expansion
- allocator restructuring
- `let`/`var` enforcement
- timing/frame assert syntax

Those are valid later, but they are not the right continuation of today’s
momentum.

## Short Take

Today was a good example of the repo’s current best mode:

- wire what already exists
- make the win visible
- prove it with tests
- document only what is actually true
