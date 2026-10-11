# Measured choice boundaries

Unchanged SelfLanz source and unchanged authoritative executable `1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041`. No production files changed. Six successful CLI runs measured four candidates each; five additional CLI invocations refused without creating an output directory, selected binary or decision.

| Workload | Original T | Arena T | Append T | Both T | Speed choice | Size choice |
| --- | ---: | ---: | ---: | ---: | --- | --- |
| Zero body | 784 | 763 | 948 | 927 | Arena | Append |
| First byte equal | 1672 | 1587 | 1836 | 1751 | Arena | Append |
| First byte mismatch | 1608 | 1564 | 1772 | 1728 | Arena | Append |

Both exact source loop sites have positive entries. The append loop has zero body entries in all three workloads. Speed selects arena alone, crossing the previous persistent corpus choice of both rewrites. Size selects the 2076-byte append candidate even though that choice takes more cycles here. Binary sizes remain original 2091, arena 2212, append 2076 and both 2197; all were measured from actual binaries. These results are workload specific.

A preloaded repeated intern hit has no append-loop entry and refuses selection. The three semantic failures preserve diagnostic raw prefixes: undeclared source read C001 after three resource events, undeclared destination write 9002 after four events, and read-only header write 9000 before any event. Each refuses selection; partial profiles are diagnostics, never selection evidence. A one-step VM budget has an infrastructure error and no typed memory-access Failure. It also refuses. The gas limit is not modeled as a semantic memory denial.

The independent original-source model checks all global values, full declared regions, ordered resource events, exact typed failure fields and all eleven source-loop counters for complete and partial calls. The typed Go auditors check source/actual proof bindings, loop identity/origin, allocation, MIR diagnostic, access and frame records, saved binary/assembly hashes and measured costs. Twenty-four successful payloads, six selections and eight raw diagnostics passed again from saved gzip files. The five refusal exit codes and output absence are recorded in `exit-codes.json`.

Large files use deterministic gzip with zero mtime, preserving original bytes listed in `preserved-payloads.json`. The native assembly includes closed workload adapters. Internal global read traffic remains outside ordered resource-event equivalence.

Portable audit from repository root:

```sh
python3 docs/grace-memory-host-choice-boundaries/evidence/audit_pack.py docs/grace-memory-host-choice-boundaries/evidence
python3 docs/grace-memory-host-choice-boundaries/evidence/independent-model/audit_raw_profiles.py docs/grace-memory-host-choice-boundaries/evidence/raw-profile docs/grace-memory-host-choice-boundaries/evidence/source.nanz raw-audit.json
```

From `minzc`, run the saved `verify_selection.go` once for each successful workload and `verify_typed_payload.go` once per candidate. No native execution is needed for these audits.
