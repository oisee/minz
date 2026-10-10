# Complete-source program evidence

Current verification supersedes historical discovery logs. Five unchanged original files run complete ordered workloads through raw profiling, both common Grace variants, full-module assembly and persistent Z80 replay. Independent host return/memory oracles cover all calls.

| Source | Calls | T-states | Module bytes |
|---|---:|---:|---:|
| arena allocator |15|14300|1193|
| state machine |1032|85005|99|
| Option |86|31447|234|
| Result |87|17490|106|
| recursion |44|329000|129|

Both common-rule candidates tie for all five sources; original wins. This corpus does not exercise eligible pointer-walk rewrites and does not establish a speedup. There are 1264 distinct workload calls. State machine covers every byte input for all four states; Option/Result use 81 pairs plus source test wrappers; recursion includes 0/1/2/3/5/7/10/16/32/127/255 with byte overflow. Arena includes all six original tests and an ordered frame allocation sequence with complete independently encoded global snapshots.

Arena historical failures led to fixes for overlapping word loads and high-byte zero-extension. `initial-discovery-tests.txt`, `shared-clone-change-tests.txt`, old tests/repeat-tests retain failure history. `verified-tests.txt` and current candidate/evidence files demonstrate the corrected result. No calls or original source functions were removed. Assertions/source sandbox blocks remain in the parsed input.

Run from minzc with the Go 1.26.8 toolchain and GOCACHE=/tmp/minz-grace-gocache:

```
MINZ_PGO_PROGRAM_ARTIFACT_DIR=/tmp/minz-grace-pgo-resume/docs/grace-pgo-program-artifacts go test ./pkg/pipeline -run '^TestProfileCompleteSourcePrograms$' -count=1 -v
```

Each directory saves source/manifest, workload, profile/run, independent oracle, both full candidate compilations, costs and results, selected binary and replay. These results cover declared workloads, no full SHA or external I/O claim.
