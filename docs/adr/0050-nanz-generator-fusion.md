# ADR 0050 Nanz demand preserving generator fusion

## Status

Draft. Date: 2026-10-18. Existing array iterator fusion is implemented; generalized
generator fusion described here is proposed. Stage G1 lowered only the pull path —
`for x in gen(args)` desugars to a local frame plus `while pull_next(&frame, &x)`,
with no fusion, so the measured pull overhead (see the G1 progress notes) is the
baseline a fusion pass must beat.

## Context

A concrete pull iterator has per-item call, dispatch and state-access costs. An immediate consumer can often execute in the same loop as its producer. Existing recognized chains are useful infrastructure but do not define arbitrary generator control flow or demand semantics.

## Proposed decision

Use the pull lifecycle of [ADR 0048](0048-nanz-stackless-generators.md) as the reference meaning. Fuse an immediate consumer only when frame identity, lifetime and control effects permit an equivalent execution. Leave a valid pull path whenever fusion is inapplicable or unprofitable.

Represent generator and yield explicitly before lowering to ordinary MIR2. A fusion pass splices consumer execution into yield sites, preserves binding hygiene and reconnects producer continuations. Give all loop exits and return targets explicit identities: consumer continue advances the producer; producer continue retains its own target; consumer break stops consumption; nested consumer loops keep their own exits. Consumer return still leaves the enclosing consumer function.

Preserve values and ordered observable effects, including traps, reads and writes through aliases. Creation must not run body effects. Do not request another source item after demand has been satisfied or consumption abandoned. Keeping the instruction order of one path is insufficient if aliasing changes observations in another path.

For `filter(p).take(k)`, count accepted outputs, not inspected source items. For source `[0,0,7,8,9]`, predicate `x>0` and k=2, output is `[7,8]`; 9 is not requested. Do not replace this with a source bound `min(length,k)`. Stage order remains significant.

Current `for i in 0..n` and iterator `range(0..n)` have different order semantics. Preserve existing contracts during this feature; any normalization is a separate compatibility change with sequence-sensitive tests.

A future generator pass may use Grace to nominate and perform bounded transformations after checking the required effects, aliasing and liveness conditions. Current HIR facts are conservative summaries, not a general ownership or generator-fusion proof engine. Profiles can guide profitability, not justify changing semantics. Generalize walker/verifier/effect coverage before allowing rules to traverse suspension nodes.

Choose among legal variants by final target cost, following [ADR 0046](0046-optimisation-policy-and-certificates.md). Measure code/data bytes, per-instance frame RAM, peak stack, first-item latency, steady-state cost and terminal cost. Include helpers, spills, duplicated consumer bodies and specialization growth. Size mode may retain pull calls where fusion duplicates code; speed mode still has a code budget.

## Consequences

Fusion can erase frame storage and resume calls for qualified sites. Multiple yield sites or consumers may increase code size. Correct binding and exit handling are necessary; textual callback substitution is insufficient. A missed optimization is visible and measurable without breaking the language contract.

## Alternatives considered

Always fusing can duplicate large consumers and change partial-consumption effects. Always using pull sacrifices useful native loops. Profile-only elimination of state/alias effects is unsound. New MIR2 coroutine opcodes are unnecessary after structured generator lowering.

## Acceptance

Compare manual fused, manual pull, generated pull and generated fused versions on the same workload/ABI/options. Test creation, one-item demand, early break/return, nested loops, shared input mutation, multiple yield sites and exhaustion. Compare ordered traces across independent reference, raw/final VM and native Z80, including code/data/frame boundaries and PC/SP. Report missed fusion and native growth. Assert absence of per-element resume calls only for cases actually demonstrated to fuse.

## References

- [Design report and test matrix](../../reports/2026-10-18-nanz-generators-design.md)
- [Existing chain lowering](../../minzc/pkg/hir/lower.go)
- [Pipeline native tests](../../minzc/pkg/nanz/pipe_e2e_test.go)
- [HIR facts](../../minzc/pkg/hir/facts.go)
