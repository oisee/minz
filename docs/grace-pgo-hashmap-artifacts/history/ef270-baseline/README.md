# Original persistent hash map actual-program baseline

Authoritative compiler/worktree ef2703333843c3261e1f26b51b66ffde5dffa295, isolated /tmp/minz-hashmap-program-baseline. Original complete examples/nanz/21_hashmap_test.nanz is unchanged; only new pipeline/hashmap_program_test.go creates a controlled persistent workload and independent host. No external calls, resources, fake-pure hosts, source ports or optimization kernels introduced.

36 ordered original-function calls use map_clear/map_put/map_get/map_has with persistent globals. Corpus includes empty miss, colliding keys initially hashing to slot15 and wrapping to0, overwrite,16filledslots/17thinsertfailure, overwritewhilefull, missingwhilefull, zero/65535keys and values, sentinelquirk(map_has returns0 when existingvalue65535), clearandrefill. All8originalMIRsourceassertions remain sourcehistory; this test deliberately uses AssertMode:none and independentlychecks its own36operation workload.

Independent Go host implements EXACTsource hash `(key xor ((key+37) mod65536)) &15`,16probe cap, littleendian2byte keys/values, read-beforeoverwrite, newslotused/key/value writes followedbycountread+write, clearcountwrite and16usedwrites, full-tablefailure. It compares everyreturn, complete80Bstorage+2Bcount snapshot and every declaredglobalbyte read/write count on original/precompile/postcompile raw and actual assembledZ80. Stack/instructionfetch/initialization/snapshotreads are excluded from data-access evidence. Host source defines read/store ordering but exported snapshots aggregate bytecounts; exactevent-sequence targettrace is not claimed.

Measured **138,440 T /1,013 B**,36calls, no eligible region sites and no optimization choice. The existing explicitregionAPI installs emptychoices, seedsoriginalordinarylooplineage and disablesotherHIRGrace rules; independentcompiledproof normalizes any supportedtail split and preserveswholeprogram. Actual T exclude callbootstrap by harness. SHA in cost.json.gz binds savedmachinebinary and targetrun. Original and stage-codegeneration outputs unchanged between the two baselineexports (secondaddedrawMIRartifactonly).

Reproduce from isolated/repository minzc with savednewtest applied:

```sh
MINZ_HASHMAP_ARTIFACTS=/tmp/hashmap-baseline-repro go test ./pkg/pipeline -run '^TestOriginalHashmapPersistentProgram$' -count=1 -v
```

Go1.26.8 used with GOCACHE=/tmp/minz-grace-gocache. ExportedlargeJSON/MIR/HIR gzip uses mtime0/emptyfilename. This is baseline evidence, no profitability or optimizedPGO claim. Nextstep may attribute actualPC/function costs and select a narrow sourceproof for zero-fill or boundedwrappedprobe loops only after measurement.
