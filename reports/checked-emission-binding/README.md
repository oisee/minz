# Bind checked emission to the actual MIR lowering

A checked machine assignment is valid for its operation stream; this alone
cannot prove that the stream still represents the source MIR. LocalTableProblem
is exported and mutable. Previously Emit checked the MIR hash and table key but
could emit a separately re-admitted altered stream under the unchanged MIR hash.

The regression constructs MIR returning 37 in A and then independently admits
altered prepared operations. Before the fix it emits LD A,99 or returns the
original value in C. Other cases alter raw-emission and optimization options.
All four are locally admissible table entries, so an ordinary cache miss would
not reproduce this failure. Evidence is in before.txt.

Emit now rebuilds the prepared problem from the actual function and parameter
placements using the existing composition validator, then compares complete
operations and options before table lookup or emission. Mismatch returns an
error and no assembly. This does not mutate MIR or run an external solver.

Full VIR passes (vir.txt); pipeline native consumers for checked composition,
word shifts/callers, local table selection and weighted V4 pass (native.txt).
This closes a public API binding gap. No actual unmodified CLI workload
miscompile or optimization benefit is claimed. Wider CFG/adapters remain work.
Integration is pending.
