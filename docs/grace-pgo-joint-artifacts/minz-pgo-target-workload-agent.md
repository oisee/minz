Implemented `RunZ80ProfileWorkload` and tests in the two requested files.

Provides persistent RAM, ABI bootstraps, per-call results/T-states, final state, binary digests, strict budgets, and refusal checks. Pointer-valued evidence is refused.

Workload and caller/joint tests pass using the specified Go toolchain. API notes: `/tmp/minz-pgo-target-workload-api.txt`. No commits.