# Constraint propagation over composition candidates

Before Cartesian search, checked fragments now form finite domains of admitted
register assignments. Pairwise arc consistency removes an assignment only when
another fragment has no compatible placement for shared vregs. Nonadjacent
fragments are checked too: a value may remain live across an intervening piece
that does not mention it.

This is the constraint-propagation part of the WFC idea, integrated in bounded
checked-table preparation. There is no random collapse, solver subprocess or
new production backend. Arc consistency does not prove global satisfiability.
Full native/ABI/scratch admission remains required after composition.

Independent complete enumeration of 343 cyclic domain configurations preserves
exactly the same full solution sets. Tests cover fixpoint, empty domains,
nonadjacent conflicts and nonmutation of input slices. An integration test has
unsupported constant placements plus one compatible return placement: pruning
fits the valid composition under the bounded search budget.

Existing native evidence is unchanged: weighted complete module 44→20 bytes,
caller 238→142T in PFCCO greedy/PBQP × Grace off/on; add5 8→6 bytes. Heldout
+19/-11/XOR90 have unchanged caller times. This is reduced search, not another
runtime speed claim. Explicit adapters for mismatched placements and CFG islands
remain future work. The API stays bounded, opt-in preparation.


An explicit cyclic contradiction remains arc-consistent but has no full
solution. The test prevents presenting pairwise propagation as a complete solver.
On this host (Ryzen 7 5800X), an eight-domain benchmark prunes 8 choices per
fragment to one in about 20.6 microseconds, 1170 bytes/25 allocations per call.
This is a local microbenchmark, not total compiler latency.
