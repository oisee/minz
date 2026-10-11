# rd16 address reuse — isolated measured prototype

Base ffdf877d. Original Nanz source is unchanged. No pending byte constant guard, Grace rule, public selection API, or compiler production edit is included.

Original whole36-call binary: 133994 T /984 B, SHA256 0390aa22f7b028216e55bca731f3ca1bb4e91ab06e70ddead41e756235034860.
Candidate:126680 T /980 B, SHA25631e686b05d328346cd092ee65a310c111de010c53c775d3c3cd8b92ccd016f11.
Measured savings7314 T (5.46%) and4 bytes. Both whole callers pass independent sequential host results, all82 storage bytes, and exact per-byte global reads/writes, plus pre/post raw VM.

The exact-source proof uses (idx*2 mod65536) being even, so its maximum is65534. Original logical base+1 cannot overflow. Physical arr+base addition and subsequent pointer+1 therefore agree modulo65536. Two byte loads remain low then high, without a wide load.

Raw oracle131612 cases per variant includes exhaustive all65536 indices at arr0/65535 and tiny heaps5/6/16 with first/second-read traps, exact ordered byte events and unchanged complete memory. VM function-name tags occupy5 initial bytes, so5 is the minimum tested heap. Target boundary test36 address/index pairs per variant verifies physical-wrap byte values, both read counters and no stores outside explicit code/stack; it does not claim emulator heap traps or independently capture target read order. Wholecaller target data traffic is independently checked.

Reproduce: use ffdf877d checkout; apply prototype.patch with git apply from the repository root; both new files plus existing hashmap_program_test.go are required. Run from minzc:
`MINZ_HASHMAP_RD16_ARTIFACTS=/tmp/rd16-new GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^TestHashmapRD16Prototype' -v`
`MINZ_HASHMAP_ARTIFACTS=/tmp/rd16-original GOCACHE=/tmp/minz-grace-gocache go test ./pkg/pipeline -run '^TestOriginalHashmapPersistentProgram$' -v`
Artifacts are emitted only when the environment variable is set. JSON/MIR/HIR gzip uses deterministic mtime0.

This loopfree helper has no LoopSiteID. Production followup requires a separate source-bound function identity and a strictly opt-in measured function-variant schema; current loop profile only binds workload compatibility, never function profitability.
