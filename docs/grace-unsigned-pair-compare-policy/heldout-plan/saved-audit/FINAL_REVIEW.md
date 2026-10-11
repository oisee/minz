# Fixed-choice transfer: final saved-native audit

PASS: sixteen fixed executions across four previously exposed workloads, covering 200 raw/native invocation pairs. The owner ran the frozen evaluator once, terminal `91.40s`, exit zero. This review ran saved-data auditors only, without CPU replay or evaluation selection.

The preserved evaluation executable SHA is `2ce28f20ed167885c845ecbfbbb8a94c446ced9f96d521efebaf5cb5632e1cc1`; every native/raw context binds it. The original training CLI SHA `0d6df9...` and both exact training decision byte hashes are separately recorded. Known source, preregistration, workload byte hashes, and eighteen fresh factory returns per workload match the frozen protocol.

All mandatory v2 envelopes retain exact policy 0/1, same-policy original/proof endpoints, full source and actual function maps, unchanged ABIs, and the two original loop IDs with the expected actual paths. Only the declared transformed functions change. The four native images match the previously established same-tuple image hashes; embedded ASM/BIN and diagnostic MIR/allocation agree with the saved files. Typed Go reserialization verifies sidecar/frame hashes and compact allocation hash. Every protected memory record binds its opcode and bounded symbol address. Arena-containing images retain the exact nineteen-instruction A-byte call-frame snapshot; no permissions were expanded.

The independent workload-driven original SelfLanz model agrees with all results, ten globals, resource snapshots, ordered peek/poke events, and successful completion. Per-call T-state sums agree with each total. Supporting policy 1/mask 0 remains a real additional measurement, not a relabelled policy 0 control.

| Workload | Control 0/0 T | Frozen Speed 1/3 T | Speed improvement | Frozen Size 0/2 T |
|---|---:|---:|---:|---:|
| short-reuse | 330515 | 326650 | 1.169% | 327008 |
| long-atoms | 4110605 | 3968826 | 3.449% | 3968942 |
| nested-mixed | 1297899 | 1271819 | 2.009% | 1281762 |
| fresh-dominant | 789462 | 696957 | 11.717% | 699845 |

Control images are 2000 B; frozen Speed images are 2100 B; frozen Size images are 1978 B. These are the preregistered choices applied without reselection. They are not declared winners among unmeasured evaluation alternatives. All four workloads were already used during development: this is retrospective frozen-choice transfer outside the training workload, not blind unseen held-out evaluation.

`binding.txt`, sixteen source-model logs, sixteen typed-hash logs, `fixed-results.json`, and `raw-inventory.json` preserve the checks. Derived inner v1-shaped files used by old auditors are diagnostics extracted losslessly from mandatory v2 envelopes; they are not standalone legacy attestations. The initial wrong-working-directory failure happened before native execution and remains a separate owner diagnostic.
