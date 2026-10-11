# Independent final PC-cost audit

The two actual runs' saved payloads pass independent original-source semantics:
all thirty invocation results, ten global values, full resource snapshots and
ordered host events match. Original: **1,221,494 T / 127,538 steps**. Both:
**1,147,025 T / 119,303 steps**. Binary SHA/bytes are bound to embedded and saved
payloads, and every observed PC's owned pre-step opcode bytes match the exact
assembled listing and binary.

| Exclusive physical owner | Original T | Both T | Difference |
|---|---:|---:|---:|
| intern_region | 334605 | 263235 | -71370 |
| region_match | 51575 | 48476 | -3099 |

All other physical owner costs are unchanged. These two differences sum exactly
to the total saving 74,469 T. The final unowned bucket is zero in both runs;
peek/poke are explicitly adapter owners. `node_tag EQU peek` is alias metadata,
not a second overlapping span. Costs belong to physical entry-to-next-entry
ranges, with final end at the last emitted instruction; internal generated code
inside a span is not separately claimed to have source-semantic lineage.

The assembler listing omits standalone labels and EQU declarations, so the raw
runtime ledger initially marked every PC unowned. That raw ledger is preserved
unchanged. The owner's separate offline projection reads the exact captured
assembly text, actual symbol table and emitting instruction byte table to derive
physical spans and aliases. Our independent auditor reconstructs those spans,
verifies every projected row equals the raw observation except Owner, and checks
unique PC keys, per-invocation steps/T, whole-run sums, all owner buckets and hot
PC rows. Data/string directives are excluded from code. Raw and derived payload
hashes remain separate; no runtime counter is reconstructed from a static opcode
cost model.

The source filename was corrected in the diagnostic factory to the saved CLI's
logical source unit so final MIR hashes match. Earlier filename-only diagnostic
failure is preserved by the owner. The final two runs passed exact prior binary,
assembly, adapter, MIR, allocation, sidecar, frame and per-call timing/step
comparison. The host diagnostic executable identity differs from authoritative
CLI1bd; that diagnostic context is not represented as the old CLI's profile.

Exclusive instruction-start attribution charges CALL/RET where each executes.
No inclusive recursion costs, HIR loop costs, source-to-target lineage
certificate, emulator-independent timing proof, or universal workload gain is
claimed. All checks here use saved files; the reviewer repeated no CPU execution
and edited no production compiler or runtime files.

Portable reproduction:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 docs/grace-memory-host-pc-cost-review/audit_pc_pack.py "$EVIDENCE/final" "$SOURCE" final-audit.json
```

Use the saved memory-host-pc-cost-evidence pack and the unchanged original
SelfLanz source. Lossless gzip artifact payloads are supported.
