# wr16 ordered-byte address reuse prototype

Base a0e8d2bf plus frozen-rd16 source copies. Only two new prototype files; original21_hashmap_test.nanz unchanged, no default rule/API registration.

Full36persistentcaller compares read-only126680T980B with read+write123708T965B. Independenthost verifies all82bytes plus source-order perglobalbyte read/write counts; original/pre/postraw andactualZ80agree. See cost.json.gz for exactbinarySHA.

Raw133760casespervariant tests orderedstores, physicalwrap andcompletepartialmemoryafterfirst/secondtrap. Actual360allRAMcases verify exactstepwise twoStorebyteeventslow→high, noextra datawrites, completeRAMoutsidecodeandstack. VMheaptraps do notexist onactualallRAMtarget.

Reproduce froma0e8: copy frozen-rd16/minzc/pkg/hir files to checkout, apply prototype.patch fromrepo root, then minzc go test ./pkg/pipeline -run '^TestHashmapWR16Prototype' -v. OptionalMINZ_HASHMAP_WR16_ARTIFACTS exportsfullcaller artifacts; MINZ_HASHMAP_WR16_DISABLE=1 selectsread-onlyreference forProgramtest. Rawtesttakes~52secondsbecausecomplete65536Bmemorycomparedall65536indicesat2basesforbothvariants. No routineartifactswithoutexplicitenv. JSON/MIR/HIRdeterministicgzipmtime0.

EarlytargetharnessdraftassumedfixedregisterABI/ZXROMdefault; correctedfinaltargettestusesactualallocationparamlocations, allRAMSetROMEnd0 andnilSMCtracker. Thoseinitialharnessfailureswere notcompilerfailures. Finalsource preserves originalStorevaluesatoriginalorder, notwidewrites.
