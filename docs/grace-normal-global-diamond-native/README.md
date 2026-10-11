# Negative isolated native experiment

Exactly one verified preallocation MIR instruction in original SelfLanz cur changes: repeated direct u16 pos load becomes a Move from its dominating entry load. Source HIR and raw closed-host oracle remain untouched. Private compilation hook is test-only on this branch; none of these changes is admitted to production or a public selector.

Both full30 native runs pass exact complete state and ordered host event checks. Original reproduces current production dba native ASM/BIN, MIR/allocation/sidecar/frame hashes, per-call steps/T and observations exactly. The diagnostic executable context is distinct eca67; no production profile is relabelled.

| Variant | T-states | Binary bytes | Steps |
|---|---:|---:|---:|
| Original | 1135370 | 2000 | 109829 |
| Reuse | 1137339 | 1998 | 111323 |

Reuse is 1969 T slower (+0.173%) while saving two bytes. Static code difference shows live pos in DE requires PUSH DE/POP DE around comparison. The former pos reload disappears and src_buf changes from ED DE to direct HL. This is an allocation tradeoff explanation; no new per-PC or helper invocation ledger was collected.

All original source/raw compatibility and function maps remain equal across the two diagnostic runs; target roots are regenerated from actual rewritten MIR/allocation. factory-baseline.json is only an internal diagnostic container here, not a new source-to-MIR public certificate. Existing runtime memory/frame permissions and host byte order remain unchanged.

Exact source snapshots, raw MIR before/after, ASM/BIN, final MIR/allocation, logs, reference command and portable full-host/typed hash audits are saved. Large files use deterministic gzip with mtime zero and original-byte hashes. Independent final reviews will be copied before final inventory. No more native runs are needed for this experiment.
