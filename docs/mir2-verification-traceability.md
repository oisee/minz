# MIR2 maturity: semantic evidence first, traceability when useful

Status: scoped proposal, 2026-10-01. Scope: HIR → MIR2 → MZV. Native backends
are a separate gate.

## Verdict and evidence

An orthogonal semantic check is worth building. A mandatory source-to-every-IR
instruction trace is not yet worth its cost. `mir2.Verify` checks structure,
not meaning; running both versions in the same MIR2 VM can miss a bug shared
by the VM and optimizer. Conversely, provenance explains a result but does not
establish that a rewrite preserves behavior.

The sibling ABAP project's V track offers the useful pattern: state a rule's
obligations, compare independent before/after paths, and refuse a rewrite when
a guard is unknown. Its DSL L1–L3 track shows how a rule line can be carried
through a typed model and template to generated lines, with mutation tests for
attribution. Those are distinct goals for MinZ too. The first one can be small;
the second requires frontend and HIR source data that does not yet exist.

Two current findings make the first goal concrete:

- `BranchEquiv` compared only integer returns under MIR2 VM samples. Its old
  generator sampled wider types and extra parameters, and calls were its only
  side-effect exclusion. A store, patch, or output could differ while returns
  matched. It is now limited to pure functions with at most two `u8` inputs,
  whose equality boundary is fully enumerated. This is agreement under VM
  semantics for that admitted domain, not an independent semantic proof.
- A probe found `i8 CmpLt(255,1)` yielding `1` before constant folding and
  `0` after; `u8 Shl(1,64)` yielded `1` before and `0` after. It also found
  common-mode wrong answers: `i8 Sar(128,1)` yielded `64` in both paths, and
  `i8 SDiv(255,2)` yielded `127` in both, rather than signed results. These
  are candidate contract/implementation bugs to reproduce in committed tests
  before changing semantics. The same-VM comparison alone cannot catch them.

## Ranked work

```text
P0  Constrain unsafe rewrites                         immediate safety fix
    ├─ BranchEquiv: admit only pure, fully enumerated u8 domains (done).
    ├─ Negative tests for effects, wider types, extra parameters (done).
    └─ Audit other VM-sampled rewrites for observations and coverage.

P1  Independent scalar oracle                         foundation + high value
    ├─ Write the u8/i8 contract for compare, shifts, signed division,
    │  overflow and traps; mark unspecified cases unknown.
    ├─ Implement a small evaluator independent of MIR2 VM and constprop.
    ├─ Compare raw VM, folded VM and oracle: exhaustive small domains
    │  where feasible, plus signed and shift-count boundaries.
    ├─ Include a deliberately wrong fold as a negative control.
    ├─ Fix confirmed divergences and gate the relevant MZV passes in CI.
    └─ Publish an admitted/unknown coverage table by operation and type.

P2  Local rewrite diagnostics                          useful after P1
    ├─ On a failed check, report function, pass/rule, operands and
    │  compact before/after MIR2 snippets.
    ├─ Add per-application events to the Go passes MZV actually runs,
    │  starting with FoldConstants and SimplifyIdentities.
    └─ Carry source line where already available; say unknown otherwise.

P3  Origin trace and broader semantics                 separate investment
    ├─ Add source spans to frontend tokens and HIR nodes, including
    │  meta-function call site and definition when needed.
    ├─ Define IDs and origin composition across lowering and rewrites.
    ├─ Build a queryable sidecar only for a demonstrated diagnostic need.
    ├─ Extend semantic cases to CFG, memory, effects and other frontends
    │  with explicit observation contracts and independent references.
    └─ Add Grace rule events when Grace is on the MZV execution path.
```

P1 is the first semantic maturity claim: agreement for a *named scalar
subset*, not correctness of MIR2 in general. The evaluator must not use
`FoldConstants`, `tryFoldInst`, VM arithmetic helpers or rewrite predicates.
The first admitted byte slice and its exact coverage are recorded in
[`mir2-scalar-contract.md`](mir2-scalar-contract.md); wider and effectful
operations remain outside that claim.
The comparator must state what it observes. For P1 pure scalars, values and
traps suffice; later memory/CFG cases need memory, output, effects and
termination observations. A timeout or unsupported operation is **unknown**,
never a pass.

## Why trace is later

`Inst` has `SrcFile` and `SrcLine`, but lowering does not populate them and
`SimplifyIdentities` replacements can drop them. Nanz tokens currently carry
a line, while HIR expressions and statements lack source spans. A reliable
source → HIR → MIR2 → rewrite → final-IR origin graph would require changes
across all these layers. It is valuable for explaining a real miscompile,
especially one involving `@print`, but should not block the scalar semantic
gate. A first failure report can name the function, pass and local IR without
pretending to know its source origin.

The MZV command runs its own Go-pass schedule. `pipeline.FuncTrace` counts
passes, and Grace has per-rule counts, but Grace is not on the MZV command's
current optimization path. Instrument the executed path first. Keep structural
`Verify` as a separate invariant check; never label it semantic verification.

## Evidence required for a completed slice

| Slice | Claim | Countercheck |
| --- | --- | --- |
| P0 `BranchEquiv` | Same MIR2 VM returns for every admitted equality input | Effects/wider-domain negative tests refuse rewrite |
| P1 scalar | Raw and optimized MIR2 agree with independent typed oracle for named operations and inputs | Wrong-fold mutation fails; shared-VM signed cases fail before fixes |
| P2 diagnostics | A failing case identifies its pass and local transformation | Injected bad fold points to the intended application |
| P3 origin | Final node maps to actual source and rewrite chain | Source/rule mutation changes only attributed descendants |

The coverage report should list admitted and unknown types, operations,
passes and observations with denominators. Wider symbolic, memory and
termination proofs can follow when a specific failing or high-value case
justifies them.
