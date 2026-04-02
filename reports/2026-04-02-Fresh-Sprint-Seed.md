# Fresh Sprint Seed

Date: 2026-04-02

Scope: compact handoff for the next session, updated after today's actual fixes,
tests, examples, and report review.

Combines:

- Apr 1 backend/codegen audits
- today's Nanz fact-checking
- today's import/inlining investigation
- today's quick wiring work

## Session Wisdom

### 1. The reports were right about the main pattern

The useful lesson from the Apr 1 audits still holds:

- architecture is mostly present
- several good paths are already half-wired
- the best work is not inventing more layers
- the best work is wiring the intended path until it is visible

Today validated that directly.

### 2. Local fixes beat speculative architecture in this phase

The highest-payoff work today was not broad redesign.
It was small, sharp, testable fixes:

- preserve self-add shape in VIR tie-prep
- prune dead imported helpers after inlining
- expose addressable lvalues through `&obj.field`
- verify `&arr[i]` also works on the same path

That is exactly the "semi-wired but unfinished" category the audits were pointing at.

### 3. User-visible examples are part of the optimization work

The `fun/` fixtures were not decoration.
They acted as:

- language fact checks
- backend visibility checks
- regression fixtures
- documentation users can actually run

This should continue.

### 4. Keep EXX and larger backend activation out of the quick-win lane

The Apr 1 reports were not wrong to highlight EXX, intrinsic expansion, and
larger backend activation opportunities.

But these are not the same class of task as today's wins:

- they are more global
- they change allocator/solver behavior more broadly
- they need larger validation slices

So for the next session, keep prioritizing local fast-path wiring over
cross-cutting activation work.

## What We Actually Landed Today

### Backend / optimizer wiring

- fixed VIR self-add regression so inlined `x+x` preserves `ADD A, A`
- added dead-function pruning after MIR2 inlining
- hardened prune-root classification beyond the original rough `__` heuristic
- made prune results visible in assembly summary, including the list of pruned functions

### Language / frontend wiring

- `&obj.field` now parses, lowers, and compiles
- `offsetOf/offsetof` was deliberately *not* elevated into a language primitive
- `&arr[i]` is verified to compile and dereference correctly on the same path

### Trust / docs / examples

- expanded fact-check tests
- added `fun/` examples for:
  - field pointers
  - field pointers in methods
  - field pointers across objects
  - array-element pointers
- added ADR for intended `let` vs `var` semantics, while postponing enforcement

## Apr 1 Reports: What Still Helps Now

Most useful carry-over lessons:

1. Prefer local wiring fixes over new mechanisms.
2. Make whole-program optimization visible, not just theoretically present.
3. Move user-facing features onto machinery that already exists.
4. Treat text/post-emit cleverness as a smell; prefer earlier structural fixes.

Less immediate for the next fresh sprint:

- full EXX activation
- large intrinsic family expansion
- broad allocator restructuring

## Best Compact Task Seed For The New Session

### Sprint Goal

Continue turning already-existing machinery into explicit, testable wins.

### Task 1. Expand VIR regression coverage for tied/immediate patterns

Why:

- `ADD A, A` was fixed
- adjacent pattern families may still lose shape in similar ways

Good candidates:

- `INC`
- `DEC`
- `ADD HL, HL`
- immediate/tied cases that depend on pre-tie handling

Success condition:

- new focused tests prove the shape is preserved
- no regressions in existing import/inlining case

### Task 2. Add more addressable-lvalue coverage and examples

Why:

- `&obj.field` works
- `&arr[i]` works
- this is a productive, low-risk vein

Targets:

- more tests around nested combinations
- more `fun/` fixtures only if they teach something new

Good candidates:

- `&self.field` patterns
- indexed pointer write/read idioms
- maybe carefully test `(&expr)` syntax only if worth supporting

### Task 3. Improve pruning/optimization visibility

Why:

- the optimizer now does something important
- users should see that clearly in asm summaries

Targets:

- keep prune list visible
- consider adding pruned count/list tests
- maybe add one small trace note if it stays compact

### Task 4. Mine another small miswired backend path from Apr 1 audits

Constraint:

- choose only a local, bounded fix
- avoid cross-cutting activation work

Good shape of task:

- one existing fast path that should already fire
- one clear regression test
- one localized implementation change

## Do Not Start With

- EXX activation
- broad intrinsic framework work
- `let`/`var` enforcement
- timing/frame assert syntax
- raw ASM import design
- large allocator rewrites

Those may all be valid later, but they are not the best continuation of today's momentum.

## Suggested First Actions Next Session

1. Re-read:
   - [2026-04-01-codex-codegen-audit.md](/home/alice/dev/minz/reports/2026-04-01-codex-codegen-audit.md)
   - [2026-04-01-codex-codegen-audit-mini.md](/home/alice/dev/minz/reports/2026-04-01-codex-codegen-audit-mini.md)
   - [2026-04-02-Quick-Wins-Roadmap.md](/home/alice/dev/minz/reports/2026-04-02-Quick-Wins-Roadmap.md)
2. Pick one small backend fast-path that is clearly half-wired.
3. Add the regression test first.
4. Fix the path locally.
5. Add or update one `fun/` example only if it improves user-facing clarity.

## Compact Summary

The right continuation is still:

- wire what already exists
- make the wins visible
- prefer local fixes over new subsystems

Today proved that strategy works.
