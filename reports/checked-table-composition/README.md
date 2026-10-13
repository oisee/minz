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

Before combining proposals, the API rebuilds the problem from current MIR and
actual parameter placements; mutating the prepared operation list is refused.

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


The existing weighted corpus helper also composes successfully in four modes:
PFCCO greedy/PBQP × Grace off/on. Complete module 44→20 bytes and real caller
238→142T in every mode, all 256 byte inputs per path. The caller keeps x live
across CALL, so saves and ABI transfers are included. This reproduces the
monolithic candidate's benefit through composition, without invoking Z3.


Heldout: the add5 fragment pool is reused without regeneration for +19, XOR90
and -11, each with a caller retaining x over CALL. All 256 inputs per workload
pass. +19/-11 use the composed candidate at unchanged 76T caller cost; XOR90
retains ordinary PBQP at 69T. This is reuse/correctness evidence, not an
independent extra speed win. Whole-function selection may prefer baseline.

Final local checks: 348 fast pipeline tests, four independent processes,
all exit 0. Checked composition/table/production/readmission VIR subset PASS.
Full research VIR/Z3 suite was not run.
