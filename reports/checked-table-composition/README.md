# Composition of checked assignment proposals

A bounded preparation API combines allocation assignments for consecutive
straight-line fragments. It operates on existing checked-table records;
there is no runtime solver, new backend, instruction rewrite or external
dependency. The resulting full record goes through existing admission and
ordinary `--checked-local-table` emission.

Shared vregs must keep identical physical locations. A mismatch rejects that
combination rather than inventing an unmodelled adapter. Full admission checks
live-through storage, expanded scratch writes, the actual ABI, all operations
and the existing explicit return move. Fragment costs are never added or
trusted: complete cost is recomputed. Work limits are checked before target
mutation. This is an allocation proposal search, not shape-equivalence proof.

The initial native consumer composes two fragments of add5. The ordinary
pipeline selects the full candidate; full module is 8→6 bytes and all 256 byte
inputs pass native return/result checks. This demonstrates composition plumbing,
not a new gain beyond the existing monolithic checked add5 candidate.

A boundary counterexample contains individually valid fragments whose dead
local constant overwrites a value live through the next fragment. Whole-function
admission rejects it. Different return ABI and excessive search bounds are also
refused. The current API requires matching placements; explicit boundary adapters,
CFG regions, automatic partition selection and larger real-program benefits
remain the next steps. No optimality claim or ordinary default enablement.
