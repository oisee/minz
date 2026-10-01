# MIR2 verification and traceability: a proposed maturity gate

Status: proposal, 2026-10-01. Scope: source/frontends → HIR → MIR2 → MZV;
register allocation and native backends are separate gates.

## Why this gate

`mir2.Verify` checks structural invariants, not semantic equivalence. The MIR2
VM executes programs, but a before/after comparison on the same VM shares its
semantics and blind spots. `pipeline.FuncTrace` counts optimisation passes and
Grace records firings by rule name; neither identifies the exact input,
output, source construct, or guard for one application. `Inst` has `SrcFile`
and `SrcLine`, but lowering does not populate them, and replacements in
`SimplifyIdentities` construct new instructions without copying them. The
current trace cannot answer why a particular instruction exists.

The sibling ABAP work offers two useful patterns. Verified lift (V) declares
each recipe's observable behaviour and explicit obligations, records evidence
from independent execution paths, and treats an unknown guard as a refusal to
rewrite. DSL L1–L3 carries a generated line through template line, typed model
node and rule line; it hashes the model, rejects stale output and tests that a
targeted mutation changes only the lines attributed to that node. The lesson
for MinZ is to keep *semantic evidence* and *origin trace* as two linked
records. Neither is a substitute for the other.

## What the gate must say

For each admitted frontend feature and each enabled MIR2 rewrite:

1. A typed, independently specified subset has defined values, width,
   signedness, overflow, traps and effects. Unsupported operations are
   reported as **unknown**, never counted as a pass.
2. A rewrite has a stable rule ID, typed match, guard, declared effects and
   an evidence profile. Pure `u8` rules can be exhaustively enumerated against
   an independent bit-vector evaluator; wider/CFG/memory rules need explicit
   boundary generators and, where feasible, SMT or another interpreter.
3. A program-level comparator observes returns, globals/heap bytes, writes,
   output, traps and termination within a fuel limit. The claimed equivalence
   declares which observations apply. Before/after agreement in MZV is useful
   differential evidence, not a mathematical proof.
4. Every MIR2 instruction and terminator can be traced to a source span or a
   generated origin. An optimisation event records its rule ID, matched IR
   IDs, guard facts, produced IR IDs and removed IR IDs. The trace survives
   subsequent rewrites and can be queried from final IR back to the source.
5. Tests contain negative controls: a deliberately wrong rewrite or mutated
   guard must fail the independent oracle; a wrong source/rule attribution
   must fail a trace assertion. An optimisation disabled by a failed guard
   leaves the IR unchanged.

The evidence profile is a matrix, not one maturity number:

| Evidence | First use | Claim it supports |
| --- | --- | --- |
| `Verify` + typed rule guard | every pass | structurally valid, admitted rewrite |
| independent bit-vector oracle, exhaustive `u8` | pure scalar rules | equivalent for all `u8` inputs under the declared model |
| before/after MZV + generated boundaries | CFG, memory, calls | no counterexample in the recorded corpus |
| frontend-to-HIR/MIR2 comparison | Nanz, Lanz, Lizp, C89, PL/M | agreed semantics for the admitted shared subset |
| mutation controls | every rule family | tests detect representative wrong results and traces |

The independent evaluator must not call MIR2's `FoldConstants`,
`tryFoldInst`, VM arithmetic helpers or rewrite predicates. Its specification
and code should be small enough to review as a second implementation.

## Origin record

Keep the human-readable trace in a sidecar so normal MIR2 dumps remain
stable. Give each source node and MIR2 node a stable ID within one compilation;
do not use block index or register number as durable identity because passes
renumber and delete them. One event could look like:

```json
{
  "rule": "mir2.identity.add-zero.v1",
  "input": ["ir:42", "ir:43"],
  "guard": {"rhs": "u8(0)", "effects": "pure"},
  "output": ["ir:57"],
  "origin": {
    "file": "examples/nanz/demo.nanz", "line": 12, "column": 9,
    "generated_by": ["meta:print@line:10"]
  }
}
```

For a generated instruction, preserve both the macro definition span and
call-site span. A source span alone cannot explain `@print` expansion. An
inlined instruction may have several source parents; a folded instruction
may depend on several input values. Record a small origin DAG, not a single
mutable `SrcLine`. An `explain` command should walk that DAG and report the
source construct, macro expansion and ordered rewrite events. It must say
`origin unknown` if a pass loses the trail.

## First vertical slice and ranking

```text
P0  MIR2 scalar contract                         quick win + foundation
    ├─ Freeze u8/u16 signedness, wrapping, shifts, division/trap semantics.
    ├─ Independent bit-vector evaluator for pure scalar expressions.
    ├─ Exhaustive u8 checks for add-zero, mul-one, sub-zero and constant fold.
    └─ Mutation controls: add-one-as-identity and wrong signed shift fail.

P1  Origin spine                                 foundation
    ├─ Source span + stable node IDs through Nanz → HIR → MIR2.
    ├─ Macro call-site and definition spans, beginning with @print.
    ├─ Preserve/compose origins in identity, const-fold and DSE passes.
    └─ JSON sidecar and `mzv explain <function>:<ir-id>` query.

P2  Rewrite evidence                             high value
    ├─ Stable rule IDs and guard/result events for Go and Grace passes.
    ├─ Before/after VM comparator over results, memory, effects and traps.
    ├─ Boundary corpus for CFG, block arguments and aliases.
    └─ CI gates for evidence coverage and trace completeness.

P3  Orthogonal frontend corpus                   maturity milestone
    ├─ One shared typed subset through Nanz and at least one independent
    │  frontend (Lanz is the likely first candidate).
    ├─ Compare both against the reference semantics and each other at MIR2.
    └─ Add Z3 story microfixtures as consumer tests, not as the sole oracle.

Later  SMT for larger finite scalar domains; memory/alias and termination
       proofs; native backend comparison when that track resumes.
```

Completion of P0–P2 means we can explain and challenge *each application* of
the admitted rewrite families. It does not establish correctness of arbitrary
programs or every frontend. The coverage report must name admitted and
uncovered operations, passes, frontends and observations, with denominators.

## Existing seams to use

- `minzc/pkg/mir2/verify.go`: structural verifier; extend checks only for
  invariants it can establish. Do not call semantic equivalence `Verify`.
- `minzc/pkg/mir2/inst.go`: source fields exist but have no producer yet.
- `minzc/pkg/mir2/constprop.go`: first rewrite family, including replacements
  that currently discard `SrcFile`/`SrcLine`.
- `minzc/pkg/rewrite/grace/grace.go`: rule result reports counts; add optional
  per-application events at the rule action boundary.
- `minzc/pkg/pipeline/pipeline.go` and `minzc/cmd/mzv/main.go`: optimisation
  schedules are separate today; the evidence harness should run the actual
  MZV schedule and expose the schedule version/hash.

The first implementation PR should contain the independent scalar oracle,
exhaustive tests and one mutation control. It should not introduce a general
rule language or claim proof for memory/CFG rewrites.
