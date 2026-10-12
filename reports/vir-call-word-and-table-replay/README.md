# Full index-word CALL preservation and production table replay

TestIndexCallPreservationFullWordNative executes an actual solved/assembled call to an independent callee that overwrites both IX and IY. Across65536 values x, initial IX=x and IY=~x are preserved with exact returnPC/SP. Each index word independently covers its entire domain; this is not the65536² Cartesian domain. ASM/BIN and terminal log retained.

After shared VIR descriptor/alias/transfer changes, production saved-table readmission (no solver) still yields44→20B and all256 caller60928→36352T,238→142T per call, for all six PFCCO/Grace combinations. Negative tamper, relocation and registry checks pass. Its broad run skips the optional generation matrix; that matrix was separately rerun with actual VIR_TEST_Z3 and all12 combinations PASS5.528s. No claim of new heldout gains or hardware execution.

Corpus Git retention audit:194 sources,85 manifests,4486 payload entries,0 failures. That is source/evidence preservation, not fresh execution of all corpus paths. Full production cbf01110 and earlier immutable full VIR index-alias checkpoint still running; current late fixes remain outside those executables.
