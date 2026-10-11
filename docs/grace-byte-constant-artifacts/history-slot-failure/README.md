# Isolated masked-slot narrowing prototype: target failure preserved

Base1d4d3d10, originalcomplete21_hashmap_test.nanz unchanged. Onlytwo new manualprototype/test files, no Grace rule or PGO API/defaultactivation. Exacttyped map_put/get source body: slotu16initializedhash&15, soleupdate(slot+1)&15, alloldslot reads replacedbyu16castofgeneratedu8 slot; probeindex and sourceorderedcalls/stores/reads/earlyreturns unchanged. HelpersABI/globalstorage/parammetadata and reservedbindingnames checked. This is experimental boundedshape, not an independentlyexportedproductionproof.

Original/pre/postraw36persistentcaller agree with independentfull82Bstate/returns/everyglobalbytecount. Independentrawtable22,032calls pass:16seedkeys,17empty/matchpositions includingfull, used1/2/255, count0/16/65535, get/has/put. Exactsourcehashstarts5/7/11/13/15, notarbitrary16startslots; wrapprobechainsvisitallslots. Sourcehash/purity not modified or inferred; no hostextern.

ActualassembledZ80 FAILS at invocation3 map_put(13,200), after key5 insertedslot15: map_keys[0] remains0 instead13. Emitted map_put tail computes ADD A,1 then AND C whereCcontainsnewslot, notconstant15; slot15→16wrong. FullmachineASM/bin/rawMIR/optMIR/source/raw and failurediagnostics preserved. No performanceoroptimizationclaim possible untilbackendfix passesactualcaller. Parent/backendagent investigate separately; thisprototype doesnotcontainbackendedits.

Reproduce in isolatedminzc:

```sh
MINZ_HASHMAP_SLOT_ARTIFACTS=/tmp/slot-repro go test ./pkg/pipeline -run '^TestHashmapSlotPrototypePersistentProgram$' -count=1 -v
go test ./pkg/pipeline -run '^TestHashmapSlotPrototypeRawTables$' -count=1 -v
```

Exportoptionalonly. Savedlargefiles deterministicgzipmtime0/emptyfilename. target.json.gz is null because the productionharness refuses at firstmismatch; do not interpret it as completedtargetevidence. Originalexpectedstate remainsindependent, neveradjustedtofailure. testartifactwriteruseslocalerrors so itdoesnoterase executionfailure.
