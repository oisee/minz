# Frozen-choice transfer evaluation preregistration

This is a plan, not an execution report. The four evaluation workloads were already used during development. Their costs are public to the team. This evaluation freezes the training choices and tests transfer outside the training workload; it is not a blind unseen held-out experiment.

## Frozen inputs and choices

PREREGISTRATION.json and the byte-for-byte inputs directory bind the unchanged SelfLanz source, the full30 training workload, four evaluation workloads (50 calls), logical source filename, and current executable SHA. Training choices are Speed policy1/mask3 and Size policy0/mask2. The report control is policy0/mask0. The newly collected production training evidence must be saved and audited before execution; its fresh context cannot be supplied by relabelling earlier profiles.

No evaluation result can change these choices. No selector is run on evaluation inputs. Report all four workloads, including regressions, rather than only successful transfers. Pure Speed remains minimum actual T-states; Size remains minimum assembled bytes. Neither objective gains an implicit byte penalty.

## Execution and proof boundary

There are 12 reported fixed-tuple evaluations: control0/0, trained-Speed1/3, trained-Size0/2 on each of four workloads. There are also four supporting policy1/mask0 originals, making 16 actual closed-native executions. These support the existing same-policy proof binder; policy0 native evidence must never masquerade as policy1 evidence. They are reported as supporting baselines, not additional selected alternatives.

Implement only after review and authorization, in an isolated private evaluator. It accepts owned known-source bytes/name, an owned prepared workload, and a fixed typed tuple. Independently parse the exact known source, validate complete one-unit provenance and original function/body/ABI/origin identity, and create fresh original and candidate modules. Use existing finite arena/append construction and independent aggregate precompile proof, including VM storage exclusion. Preserve all ordinary functions, source origins, global metadata and ordered host events. Each fresh callback must retain its pre-mutation graph and be rechecked against the immutable original. No arbitrary transformed factory, ASM input, new memory permission, inherited loop counters, generic CSE, or modified source is admitted.

Run existing closed baseline-v2 execution with explicit backend options. The policy1/0 reference binds the1/3 proof; policy0/0 binds0/2. Bind typed compatibility, full function maps, source sites and actual tail sites to corresponding original/candidate envelopes. Prepared MIR remains diagnostic, not a post-HIR certificate. Report comparison to0/0 separately from same-policy legality binding.

## Independent verification and reporting

Before execution, verify every copied input SHA, call count and known source SHA. Existing independently implemented source model remains frozen; it checks full persistent nodes/strings/source bytes, all ten globals, returned roots, ordered peek/poke events and typed failure prefixes. Capacity limits are unchanged: source4096, nodes4096, strings8192; admitted ASCII atoms are at most240 bytes. Maximum live node bytes are48/24/144/8 and persistent string bytes23/723/117/477 for short/long/nested/fresh respectively.

Save executable bytes and SHA, commands, source/workload bytes, options, all16 complete original/candidate envelopes, exact precompile proofs, ASM/BIN, final MIR/allocation, per-call T-states/steps and totals. Recompute all native hashes and assembly/binary identities; retain any refusal or failed execution. Do not rank a surviving subset after an error. Report absolute T/B, percent change versus0/0, and per-workload regressions. No new PC attribution or instrumentation-counter cost claim is made.

Earlier corpus experiments prove semantic correctness and measured per-workload reselection on those inputs. They do not prove that the full30 winner transfers, that sites compile independently, or that a universal winner exists. A later genuinely unseen evaluation requires separately generated sealed inputs, a preregistered generation/split procedure, immutable hashes before optimization work, and no cost-driven changes to that set or to the frozen training choices.
