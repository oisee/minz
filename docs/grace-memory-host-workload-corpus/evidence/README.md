# Persistent SelfLanz workload corpus

All measurements use the unchanged original source and the same authoritative CLI executable (`1bd95516f7b1ef1ba56c17822104936e2b73fe8bde5fe7368dd128811009f041`). Four ASCII workloads contain 50 persistent calls. Each goal measures all four candidates; source, workload, profile, proof, assembly and binary are preserved.

| Corpus | Calls | Original T | Arena T | Append T | Both T | Speed reduction |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| short-reuse | 12 | 355595 | 354055 | 351368 | 349828 | 1.62% |
| long-atoms | 9 | 4401641 | 4400893 | 4217093 | 4216345 | 4.21% |
| nested-mixed | 9 | 1419255 | 1411486 | 1400553 | 1392784 | 1.87% |
| fresh-dominant | 20 | 849806 | 849674 | 734374 | 734242 | 13.60% |

Speed selects both rewrites in all four corpora; size selects append hoisting. Actual binary sizes are 2091, 2212, 2076 and 2197 bytes for original, arena, append and both. This corpus has no winner crossover; the gain varies with persistent lookup and copying traffic. Costs are measured for whole workloads and are not inferred by adding individual gains.

The independent Python model checks results, all ten global values, complete declared memory regions, ordered host events, typed failures, and all eleven source-loop counts per invocation. The Go auditors check typed precompile proofs, source-site origins, allocation and MIR diagnostic hashes, native access/frame records, exact assembly/binary bytes, call counts, summed costs and goal selection. All 32 candidate payloads and eight selections passed again after compression. Internal global read traffic is outside the resource-event equivalence claim. The native assembly includes closed memory-host adapters and is a bounded workload artifact.

Large payloads use gzip with mtime zero; `preserved-payloads.json` records original byte hashes. Readable original source is `source.nanz`; the portable auditors accept raw or gzip files. The raw-profile review harness has a separate Go process identity and is used only for observability; every CLI invocation collected and replayed its own profile with the authoritative executable.

Run the Python audit from the repository root:

```sh
python3 docs/grace-memory-host-workload-corpus/evidence/independent-model/audit_corpus_pack.py docs/grace-memory-host-workload-corpus/evidence --source docs/grace-memory-host-workload-corpus/evidence/source.nanz --out audit.json
```

Run `verify_typed_payload.go` for each candidate and `verify_selection.go` once for each corpus from `minzc`, using `go run` and the evidence path. No additional CPU execution is needed.
