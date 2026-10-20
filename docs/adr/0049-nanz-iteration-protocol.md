# ADR 0049 Nanz concrete iteration protocol and typed interfaces

## Status

Draft. Date: 2026-10-18. The baseline operation is implemented in stage G1 as the
compiler-known `pull_next(&frame, &out) -> bool` (concrete `name$next` function).
ERROR/PENDING states and typed interfaces remain proposed, not implemented.

## Context

Current structural interfaces retain method names and resolve an interface receiver only when one implementation exists in the module. They do not yet provide checked generic signatures or an associated element type. Generator support should not depend on implementing a universal trait system first.

## Proposed decision

Use a statically known concrete state type and a verified continuation function. A baseline operation is `next(frame, out) -> bool`: true publishes exactly one element; false is permanent completion. The output stays unchanged when false. Frame and output storage must not overlap. The compiler-known continuation receives exclusive access to the frame during advancement; reject untracked frame aliases and reentrant access. Proposed `pull_next` and method syntax must resolve to a concrete function without vtables.

A fallible source requires three distinguishable results: ITEM, DONE and ERROR. Use an explicit status and separate item/error outputs initially. DONE and ERROR are terminal; repeated ERROR returns the same error without repeating source effects. Item output remains unchanged on DONE/ERROR. Specify how errors are initialized and propagated before accepting fallible sources.

A nonblocking live source may need PENDING. That is a separate polling protocol; temporary emptiness is not DONE. An iterator over a queue must explicitly choose a finite snapshot or a close-aware adapter.

Portable HIR represents availability and errors as semantic values. Z80 may later use carry or another flag when verified contracts allow it. Do not allocate carry simultaneously to availability and language failure: current `FallibleCY` admits only defined u8/void single-return functions. A combined convention requires separate verification and cannot be assumed from existing flag machinery.

Later, typed interfaces may unify generated frames, manual cursors and other sources. Require complete method signatures, an element type contract and specialization by the concrete receiver at each call site. Multiple unrelated iterator types must not create module-wide ambiguity. Preserve direct calls and explicit diagnostics for unsupported cases.

## Consequences

Concrete generators can ship before generic traits. Some protocol plumbing may be compiler-known initially. Any future generalization must preserve behavior and diagnose signature mismatch rather than reinterpret arbitrary same-named methods.

Reference-valued items, borrowed views, frame moves, async polling and resource cleanup require additional contracts. They are not implied by matching a method name.

## Alternatives considered

A universal `Iterator<T>` syntax immediately would conceal missing type-system work. An overloaded bool/error carry conflates EOF with failure. Treating an empty ring as DONE prevents future arrivals from being observed. An allocation-heavy boxed iterator is unnecessary for statically known sources.

## Acceptance

Exercise at least two concrete cursor types in one module, reject mismatched element/signature declarations, and verify DONE/ERROR distinction, unchanged item output, sticky terminal outcomes and no repeated I/O. Compare native outputs and ordered effects with both MIR2 paths. Any flag ABI variant must preserve live caller values and error propagation.

## References

- [Generator lifecycle](0048-nanz-stackless-generators.md)
- [Design report](../../reports/2026-10-18-nanz-generators-design.md)
- [Interface parsing and dispatch](../../minzc/pkg/nanz/parse.go)
- [Existing error ABI](../../minzc/pkg/hir/error_abi.go)
- [MIR2 function contracts](../../minzc/pkg/mir2/func.go)
