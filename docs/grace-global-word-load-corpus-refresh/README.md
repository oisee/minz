# Fresh corpus measurements

The four unchanged persistent workloads were measured with the frozen production executable `dba34fdc489d4fa00f985bff82952076f8de8f68488999dfe98364af242c9ca5`. Source SHA is unchanged; 50 calls per candidate family. All eight CLI runs exited zero.

| Corpus | Original T | Arena T | Append T | Both T | Speed/size choice |
|---|---:|---:|---:|---:|---|
| short-reuse | 330515 | 331075 | 327008 | 327568 | append |
| long-atoms | 4110605 | 4110877 | 3968942 | 3969214 | append |
| nested-mixed | 1297899 | 1299550 | 1281762 | 1283413 | append |
| fresh-dominant | 789462 | 789510 | 699845 | 699893 | append |

Binary sizes for masks 0–3 are 2000, 2121, 1978 and 2099 bytes. Each objective measures all four complete callers. Arena adds 560/272/1651/48 T to append on these workloads; previous executable `1bd` selected both for speed. These are separate compiler-context measurements, not additive estimates.

Fresh input profiles bind the current executable and all 11 original source sites. Portable host audit checks all raw/native full states and ordered resource events; Go auditors check typed precompile proof, source/actual sites, sidecar/frame/allocation/MIR hashes and selected bytes. Independent final reviews e67a77a6 (host/profile) and d39ae34b (native/proof) pass and are copied beside the evidence.

Large JSON, MIR and ASM files are gzip with mtime zero; preserved-bytes.json binds every original byte sequence. Target/selected binaries are retained exactly. No compiler or runtime changes are part of this artifact-only branch.
