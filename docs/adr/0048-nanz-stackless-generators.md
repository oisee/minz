# ADR 0048 Nanz stackless generators and caller owned state

## Status

Draft. Date: 2026-10-18. Stage G1 is implemented: scalar `gen`/`yield` lowers to a
caller-owned frame plus a concrete `next(frame, out) -> bool` (`minzc/pkg/hir/lower_gen.go`,
`minzc/pkg/nanz/parse.go`; differential gate `minzc/pkg/nanz/gen_yield_e2e_test.go` on raw
MIR2 VM, optimised MIR2 VM and Z80). Fusion (ADR 0050) and fallible sources remain proposed.

## Context

Nanz has fused iterator chains and named pipelines. Streaming producers need an explicit meaning for suspension, independent instances and partial consumption. A scheduler or a separate stack per producer would add costs that sequential iteration does not require.

## Proposed decision

Introduce a structured generator declaration and `yield` in typed HIR. Proposed surface syntax is `gen name(args) -> Element { ... yield value ... }`; exact syntax is subject to parser design. A declaration describes a factory for a single-pass concrete iterator.

Arguments are evaluated and captured once during construction. The body does not run until requested. Each `next` resumes until one yielded scalar or completion. Code following a yield runs only upon another request. Completion is sticky and effect-free on repeated requests. An unsuccessful request leaves the item output unchanged.

Lower suspension to caller-owned concrete state plus ordinary functions and MIR2 branches, loads, stores and returns. Store continuation identity and values live across a suspension. No implicit heap, private stack, runtime coroutine opcodes or hidden global per-instance state.

Initially admit scalar yields, nonrecursive structured control, nested loops and multiple yield sites. A frame is noncopyable, nonescaping and exclusively resumed. Borrowed sources must outlive it; reject a borrowed local source escaping its scope, references yielded to locals/frame fields, ambiguous aliasing and reentrant advancement. Nanz's raw pointers do not establish these conditions automatically. Accept only statically established source lifetimes and access bounds; reject unknown pointer provenance unless a separate explicit unsafe contract is introduced. Lexical containment of a frame alone does not prove its pointee survives. Define checked admission before exposing the feature.

An immediate consuming loop owns its temporary iterator; leaving it discards that frame without running the remaining body. Breaking a manual loop over an explicit live cursor only stops requests; a subsequent next resumes the same cursor. Abandoning the frame discards state without running the remaining body. Resource ownership across suspension, implicit cleanup, moving frames with interior pointers, recursion across suspension and ISR resumption are outside the initial profile. A generator that can loop forever before its next yield has no bounded-step guarantee.

Channels, temporary lack of input and scheduler yield are separate contracts. A live empty queue must not be reported as sticky exhaustion.

## Consequences

Persistent generators incur frame RAM and per-request dispatch. Their storage and lifetime are visible and bounded. Immediate consumers may eliminate those costs under [ADR 0050](0050-nanz-generator-fusion.md). A pull implementation remains the semantic fallback.

Frontend/HIR visitors, verifier, effect analysis, source mapping and lowering must all understand suspension before optimization. Generator verification precedes generic rewrites; rejecting an unhandled node is preferable to silently dropping its effects.

## Alternatives considered

Early frontend desugaring into ordinary HIR is a viable alternative suggested by the GLM review. This draft instead favors one generator-aware HIR representation for both pull and fusion; G0 must assess visitor/verifier coverage and implementation cost before acceptance. This is a lowering decision, not a requirement for coroutine machinery in MIR2.

A fused-only macro cannot supply persistent lazy iteration. Stackful coroutines and a scheduler solve a larger problem with additional memory and lifecycle requirements. A singleton global frame breaks independent instances. These are not the initial implementation.

## Acceptance

Compare an independent reference, raw/final MIR2 VM and native Z80 for ordered output/effects, interleaved instances, creation without body effects, nested yields, early abandonment, sticky exhaustion, output preservation and lifetime refusals. Verify frame bounds and PC/SP. Measure code, persistent RAM and peak stack; no universal zero-cost claim.

## References

- [Design report](../../reports/2026-10-18-nanz-generators-design.md)
- [HIR types](../../minzc/pkg/hir/hir.go)
- [HIR lowering](../../minzc/pkg/hir/lower.go)
- [Native frame support](../../minzc/pkg/mir2/z80codegen_frame.go)
- [Historical generator proposal](../2026-01-03-303-Generator_Implementation_Spec.md)
