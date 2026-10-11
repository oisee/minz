This isolated stage adds a policy-aware four-mask selector v2. One explicit MemoryHostBaselineOptions value is owned before factory calls and applied to the original association plus all three privately constructed variants. The public output retains mandatory baseline v2 envelopes; source precompile proofs remain the existing HIR-only aggregate proof and are additionally bound to each policy envelope.

The old selector, association and CLI remain Legacy/v1. GoalSize compares actual byte lengths at the requested policy; it does not enable OptSize. There is no CSE, fifth policy bit, arbitrary assembly or transformed-source callback.

The selector retains the reviewed 24 public factory calls: one initial source, five association calls, then six calls per nonzero mask. Strong references keep every source graph alive before compilation or private transformation. Only the known association compilation candidate is excluded from immutable-source checks. Every mask repeats its precompile proof and checks previously retained sources across later masks.

The association producer, strict loader and replay APIs belong to the independent association owner. This branch owns only the new selector implementation and tests, reusing existing pure construction/proof helpers without modifying old acceptance.

Before measurement, tests cover policy/schema mismatch, malformed source profile, stale source/workload, proof and candidate envelope mismatches, forced GC/shared graph refusal, late cross-mask mutation, ties and unobserved sites. Whole-caller measurements require frozen source and independent review. No CPU measurement has run in this checkpoint.
