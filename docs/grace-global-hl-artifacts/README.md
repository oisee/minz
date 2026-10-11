# Live HL lost by direct constant global-byte store

Base dbac514b, isolated /tmp/minz-global-hl-fix. No bool backend changes mixed in. Original source once/effect is unchanged in before/after. Raw and optimized VM implement(v&255)+256; actualZ80before failsall12(value0/1/255/256/511/65535 ×HIRGraceoff/on), replacing v with the global symbol address. Preserved before.txt and full old failurehistory.

Cause: scanGlobalFieldPatterns eliminates explicitAddrOf usedonlybyStore; constantdirectbyte-storecodegen introducesimplicitLDHL,symbol. Registerallocation/liveness didn'tmodelthat scratch, so liveincomingHLwasoverwritten before forwardingtocalleeDE. Localfixonlyconstantdirectstorepath checks actual liveafterlocations HL/H/L, PUSHHL beforeaddressload andPOPHLafterbytewrite. DeadHLkeepsprevious20Tpath; A/flagsunchanged.

Source12VM/rawopt/actualcasesPASS0.069s; machineHL/H/Llive andDEreference(deadHL) fourcasesPASS0.009s checksfullHL/A/carry/storevalue andnoextraPUSHforHLdead. No generalgatesrun. Sourceeffect incrementscounterexactlyonce, returnedbyte+counter256 independentlychecksargumentandglobalmutation.

Reproduce dbaccheckout applyingfix.patch.gz afterdecompress; fromminzc: go test ./pkg/pipeline -run '^TestGlobalByteStorePreservesLiveHL$' -v; go test ./pkg/mir2 -run '^TestZ80DirectConstantStorePreservesHLBytes$' -v. Exporthelper optionalMINZ_GLOBAL_HL_EXPORT receivesdir, gorumfromminzc. Before/after actualASM/BIN/HIR/rawoptMIR+SHA saved. Fulltestsourceandproductioncopyhere. Gzipmtime0 deterministic. This is a scratch preservation correctness fix, no standalone speedclaim.

Rebased finalproduction on d6e3a8d364976c261b13e3b37f515afc7f444de0; exact3-fileSHAinventory unchanged. Directedsource12PASS0.077s andmachine4PASS0.009s. Before/afterartifacts retained historicaldbacmeasurement. Finalpatchrelativeboolcommit.

Parent archive stores historical program.a80 losslessly as program.a80.gz. isolated-history-manifest.json names original uncompressed paths for provenance only; current manifest.json verifies saved files. Parent broad gates and fresh measurements remain pending until reported.
