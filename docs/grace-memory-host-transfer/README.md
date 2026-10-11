# Private fixed-choice transfer evaluator

Implementation-only checkpoint based on a27e40b5. The evaluator lives entirely in a pipeline test file; no production API, CLI, permission, source transform or optimization default changes.

The exact preregistration and immutable original source/evaluation byte hashes are mandatory. Fresh original factories are independently checked against the known parse, including full semantic/ABI/function-origin identity and assertion metadata, before private finite construction. Every original and candidate graph is retained before mutation; each callback rebuilds the same independent aggregate proof. Same-policy policy1/mask0 supporting evidence is measured rather than relabelled from policy0/mask0.

The four tuples are fixed: reported control0/0, supporting1/0, trained-Speed1/3 and trained-Size0/2. Across four already-known corpora this plans16 native executions,12 reported. No selector runs on evaluation data, and results cannot change the choices. This is retrospective out-of-training-workload evaluation, not blind unseen holdout.

The focused admission, independent source-identity, shared-root, precompile late-mutation, finite-shape and exact guard tests passed in 44.489 s. The guard tests force GC across 18 fresh returns and reject mutation through an old retained root or shared body at return 18; these tests do not execute the native evaluation. The exporter also forces GC and requires exactly 18 caller returns per workload.

The preregistration executable SHA identifies the training CLI (0d6); the preserved evaluation test executable will have its own separately exported identity. The evaluator returns either the complete four-tuple result or an error. On failure the exporter retains inputs and the error, but does not claim a partial native tuple bundle.

The released exporter completed all 16 native executions in 91.40 s, exit 0. Each workload required exactly 18 fresh original returns. The preserved evaluation executable SHA is `2ce28f20ed167885c845ecbfbbb8a94c446ced9f96d521efebaf5cb5632e1cc1`; the training executable remains `0d6df9fbf769bdcca6969717449f587ff856b0c7d38d525b78129811abe0de12`. An initial invocation from the wrong working directory failed before any native execution; its log and exit status are retained separately.

| Workload | Control 0/0 T | Support 1/0 T | Speed 1/3 T | Size 0/2 T |
|---|---:|---:|---:|---:|
| short-reuse | 330515 | 329597 | 326650 | 327008 |
| long-atoms | 4110605 | 4110217 | 3968826 | 3968942 |
| nested-mixed | 1297899 | 1286305 | 1271819 | 1281762 |
| fresh-dominant | 789462 | 786526 | 696957 | 699845 |

Binary sizes are 2000 B for control, 2001 B for support, 2100 B for trained Speed and 1978 B for trained Size. These are fixed-tuple measurements, not evaluation rankings. Independent saved-payload audits are pending. The packet preserves all results, including the cost of the larger trained-Speed binary, without changing either training decision.
