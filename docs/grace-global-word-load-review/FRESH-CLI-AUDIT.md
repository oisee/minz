# Fresh production CLI profile audit

All sixteen saved raw/native candidate records from four fresh dba34fdc CLI
runs pass the independent original-source model: invocation results, all ten
global values, entire resource snapshots and ordered host events. All four
profiles' per-invocation and cumulative eleven-site counts match the independent
source counter oracle. Full workload region_match is 49 Entries / 59 bodies;
append is 13 / 286. Small workload region and append each have 1 / 0, admitted
as observed zero-body loops.

| Run | Four actual T-state costs (original / arena / append / both) | Selection |
|---|---|---|
| full speed | 1135370 / 1135211 / 1079795 / 1079636 | both, 1079636 T |
| full size | same | append, 1978 B |
| small collect | 604 / 583 / 663 / 642 | arena, 583 T |
| small supplied replay | same | arena, 583 T |

Binaries are 2000 / 2121 / 1978 / 2099 bytes. The fresh backend's incremental
arena gain on the full workload is now 159 T, not the previous backend's 3099 T.
The smaller direct-load code changes measured interactions; no old coefficient
or profile is reused as evidence for this executable.

Small collected and same-executable supplied-profile replay selections are
exactly equal, including all candidates/profile/association/proof payloads.
Current executable context is dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5.
Checks bind each candidate's typed precompile original/actual compatibility and
function maps, the two source/actual site paths/origins, compact sidecar/frame/
allocation and MIR hashes, embedded/saved BIN/ASM, objective minima and exact
selection-file SHA in each CLI decision. The owner's Go typed audit supplies
schema serialization checking; this portable independent source/count model
adds no native execution. Postcompile source-loop lineage is not inferred.

Reproduce without CPU execution:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 docs/grace-global-word-load-review/audit_fresh_cli.py "$EVIDENCE" --source "$SOURCE" --out fresh-cli-audit.json
```

Use the saved global-word-load-cli-evidence pack and unchanged SelfLanz source;
lossless gzip payloads are supported. Production integration/gates remain with
the parent. This review branch changed only its artifact directory.
