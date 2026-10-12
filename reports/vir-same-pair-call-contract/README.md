# Same-pair candidate with real transfer and CALL contracts

A 16-bit move whose operands occupy the same pair is now a zero-byte, zero-T candidate. Transfer lookup rejects tied patterns for different registers, so this candidate cannot erase real copies. Double16ThenDec now emits ADD HL,HL; DEC HL; RET (3 bytes instead of6). Existing DEC HL quality expectation passes; native arithmetic passes all65536 word inputs with exact returnPC/SP.

Two correctness contracts were necessary to admit it:

* Index-half prefix-conflict transfers internally use A. Those move patterns now preserve AF and charge21 additional T/2 bytes in all emission paths; the scratch save is part of the operation, not a post-hoc free fixup.
* VIR call liveness assumes IX/IY and halves survive CALL, while generated callees freely allocate these pairs. The CALL pattern now pushes both pairs and restores them, costing75T/11B including the17T/3B CALL. This is a conservative implementation of the existing preservation contract, with real overhead in solver costs. A narrower callee-derived save policy remains a possible optimization.

Without scratch preservation, standalone board stores fail; without CALL preservation, the old mixed scenario returns152 instead16. Final tests: all25 cells×256 values independently verify6400 stores/6400 reads, untouched other cells/adjacent bytes and exactPC/SP; chained calls cover256 inputs; indexed mixed calls cover256 input values with a prior read live through a second getter. All PASS. Broad asserts/codegen/table/indexed/island/word-return family PASS3.432s, before adding the final mixed full-domain test; final focused set PASS3.884s. Original source/scenario assertions and quality expectations are not weakened.

Immutable full research VIR run from the preceding index-alias checkpoint is still running, as is production pipeline cbf01110. Neither covers this later change. No full-current-VIR or universal arbitrary-CFG PASS claimed. The production MIR2→Z80 path is separate.
