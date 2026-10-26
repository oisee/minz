# ADR 0051 Region handles and banked arenas

## Status

Accepted direction; experimental library/simulator first. Date: 2026-10-26.
General HIR address spaces, effect checking and new language syntax are proposed,
not implemented by this decision.

## Context

Crystal borrowed views, Nanz generators and arenas need explicit lifetimes without
mandatory GC. Banked machines expose only a window of physical RAM at once.
An ordinary near pointer becomes invalid when that mapping changes. A flat24
pointer ABI alone does not describe address spaces, banking or region lifetime.
Autosiril's current 87,179,690-byte VM reservation is mostly fixed-capacity tables,
not a measured minimum simultaneous live set.

## Decision

Persistent references in pageable collections are typed object handles. Resolution
returns a value, or a short exclusive borrow of a mapped chunk. Backends may use
flat RAM or bank+offset mapping without changing collection algorithms. The first
experiment uses read-only packed notes, scalar accessors and a one-window simulator.
It never yields a near pointer. A mapping generation detects stale explicit leases
in the simulator. Bounds and exhaustion return errors before truncating handles.

Objects fit within one bank; allocation leaves any unused bank tail. Split objects
require a distinct accessor protocol. Copies/comparisons spanning banks use values
or a fixed bounce buffer. Iterator chunk leases may amortize switching, but mapping
changes remain observable memory effects to optimizers.

A borrow cannot survive yield, arena reset, task switch or an unknown callback.
Generator frames persist handles and scalar values, not temporary mapped pointers.
Do not store a near pointer in a persistent object, return it, or pass it to an
unclassified foreign call. Code, stack and mapping control state remain accessible
while a bank changes; a hardware implementation must define IRQ save/restore or a
mapping-preservation policy. The simulator does not establish that hardware policy.

Future shared TargetABI/DataLayout must distinguish language integer widths,
pointer/address spaces, physical windows, field layout and calling conventions.
Imports/meta receipts must carry compatible layout contracts. No automatic change
to Crystal or C pointer semantics is authorized by this library experiment.

## Consequences

The same collection access contract can support flat eZ80 and banked Z80 targets.
Accessors and mapping have costs, which must be measured. Handle lifetime after
reset eventually needs generations/region checking. Existing raw-pointer generator
provenance limitations in ADR 0048 remain unresolved; this experiment avoids storing
borrowed pointers rather than claiming to solve them globally.

RAM banking does not compress data or add physical memory. Disk paging is a
separate storage system with eviction, dirty pages and I/O costs; it is not the
first implementation. A 128/256 KiB budget includes code, stack, runtime, windows,
input state and every simultaneously live arena.

## Acceptance

Run the same access sequence over flat and single-window backends. Force unrelated
mapping between accessors; results must agree. Test bank boundaries, largest handle,
exhaustion, stale leases and partial consumption. Report physical reservation,
used payload, frame size, mapping changes and errors separately. Never interpret
successful simulation as a native bank-switching implementation.

## Alternatives considered

Arbitrary transparent raw pointers cannot preserve validity across remapping.
An unrestricted demand pager hides potentially severe I/O thrashing. Requiring
application code to manually select every bank defeats reusable typed collections.

## References

- [Stackless generators](0048-nanz-stackless-generators.md)
- [Iteration protocol](0049-nanz-iteration-protocol.md)
- [Demand preserving fusion](0050-nanz-generator-fusion.md)
- [Bounded Autosiril pipeline](0052-autosiril-demand-driven-pipeline.md)
