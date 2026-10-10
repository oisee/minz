# ADR-0046: Optimisation goals and checked evidence

Status: Proposed architecture. Conservative HIR size gate implemented;
certificate loader, profiler and semantic site mapping not implemented.
Date: 2026-10-10.

## Legality and profitability

```text
program identity + assumptions + checked proof / runtime guard
                          ↓
                 legal candidate rewrite
                          ↓
target bytes/T-states + profile distribution + optimisation policy
                          ↓
             choose candidate or keep original
```

Grace, profiles and LLMs may nominate candidates. None bypasses legality checks.
Profiles describe executed inputs; they cannot establish universal no-alias,
no-overflow or linear-ownership facts. Two reviews improve confidence but do not
constitute a machine-checkable proof.

## Optimisation goals

- Size: minimise target code/data bytes, including guards, fallback, tables,
  helpers and spill storage. Runtime may increase within the declared policy.
- Speed: minimise workload-weighted T-states subject to byte/compilation budgets.
  Use distributions for length/type-sensitive choices, not just a mean input.
- Balanced: impose a growth budget or expose Pareto alternatives. A weighted
  score needs explicit weights and units.

PBQP/codegen output determines actual bytes. HIR estimates can prune candidates,
but cannot guarantee final size. Compare the same target/ABI/options and charge
setup, guard failure and fallback too. Unknown target costs must not silently
inherit VM/Z80 costs.

Implemented first step: existing `--Osize` sets HIR Grace's `OptimizeForSize`.
This excludes guarded pointer-walk (it duplicates the loop), while retaining
constant-if/invariant-base. Measured sum_array selects 38 bytes instead of 85.
This is a conservative rule gate, not global size optimality. The ordinary
explicit `--hir-grace` path retains its previous choice.

## Evidence kinds and trust

| Kind | Contents | Authorises unchecked rewriting? |
|---|---|---|
| profile | Counts/histograms, corpus/build fingerprints, collector metadata | No; profitability of legal candidates or proposal for guarded speculation |
| proof | Claim/evidence accepted by a named trusted checker, with assumptions/dependencies | Only the exact checked claim/site/context |
| review | Human/LLM rationale and suggested obligations | No; guides investigation, proof generation and tests |

An LLM certificate is a proof candidate or guarded plan, not permission based on
issuer reputation. Checkers accept restricted data and whitelisted proof rules
or guard templates; never execute supplied code. Unknown kinds, incomplete
assumptions and ambiguous identities refuse optimisation.

For representation/ownership changes, instrumenting the old representation can
find sharing sites. The profile cannot justify value copies elsewhere. Copy
insertion requires an explicit semantic argument; a guard cannot detect sharing
after the new representation has made that sharing unobservable.

## Guarded speculation

Record the assumption, guard, fast path, original general path and observable
effects. Check before irreversible effects. Replay requires rollback, buffering
or idempotence; a write barrier must precede the write. Repeating I/O is not
rollback. Stale profiles may worsen cost, never justify a different answer.

This relies on correct guard lowering and a correct general path. Fallback does
not repair arbitrary Go/ABAP or VM/Z80 semantic divergence. Cross-backend
conformance remains an independent requirement.

## Identity and propagation

Use semantic sites, not generated line numbers or transient node IDs. Proposed
origin: declaration identity + source span + node kind + ordinal + inline-context
path. Include declaration/dependency and typed-IR/semantic fingerprints. Spans
and ordinals help lookup, but never permit reuse after changed fingerprints.

Illustrative envelope; not an implemented schema/loader:

```json
{
  "schema": "minz.optimisation-evidence/v1",
  "kind": "profile",
  "site": "declaration/source-site/inline-context",
  "program": {
    "declaration_sha256": "<full hash>",
    "dependency_sha256": "<full hash>",
    "typed_ir_sha256": "<full hash>",
    "semantic_contract": "<version>"
  },
  "claim": "loop-length-distribution",
  "evidence": {
    "corpus_sha256": "<full hash>",
    "collector_version": "<version>",
    "samples": 4096,
    "histogram": {"0": 64, "1": 32, "255": 4000}
  },
  "use": "profitability-only"
}
```

Proof/guard envelopes additionally identify checker/template versions, complete
assumptions, target/ABI conditions and evidence dependencies. Cost evidence has
its own target/compiler/pipeline/options fingerprints.

Collect on a cheap backend, resolve event IDs via a site map, and reissue on HIR
only after validating identity and contracts. One HIR event may lower to many
MIR instructions; fusion, duplication and inlining need explicit lineage and
counter semantics. Each rewrite invalidates/recomputes affected evidence.
Observed backend facts do not automatically become HIR proofs.

Source maps recover origins, not equivalence. Semantic receiver histograms and
loop counts may be portable when event definitions agree. Register occupancy,
allocation counts and target cycles remain backend-specific without additional
justification.

## MinZ implementation sequence

1. Complete pointer conformance from ADR-0045 and verifier contracts. Keep current
   guards until semantics agree.
2. Add stable HIR sites and lineage through splitting/inlining/lowering. MIR Inst
   has SrcFile/SrcLine, but HIR statements/expressions lack a common source identity.
   The abapiti site mechanism discussed by the user is not present in this checkout.
3. Add an LLM-less VM/Z80 event collector; first pilot: loop length and guard
   hit/miss distributions for sum_array, keyed to source/build.
4. Add a refusing-by-default data loader. Profile input only changes cost choices.
   Test stale/dependency hashes, inline-context mismatch, unknown templates,
   malformed input and no-profile behavior.
5. Add target byte budgets and profile-weighted T-state comparison. Select guarded
   pointer-walk only when legality and expected benefit both pass; report reasons.
6. Add narrow proof certificates with trusted checkers. LLM-produced evidence
   faces the same checks as any other issuer.

No external collaborator messages or abapiti modifications are part of this ADR.
