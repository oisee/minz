# Mixed byte/word adapter copies preserve live A

Cross-width adapter emission previously delegated to generic register moves, bypassing the AF preservation used by byte-only index-prefix copies. Native H→IX and IX→H (and related IY/low-half cases) reproduced live A corruption. Zero-extension now performs a safe byte copy into the low half before clearing the high half; truncation copies the low half through the same safe byte emitter. Same-pair and byte/word semantics are unchanged.

Native matrix executes all 11 byte locations × 5 word pairs × 2 directions: 110 assembled variants. Each zero-extension case covers all 256 source byte values; each truncation case covers all 65536 source word values (odd multipliers generate permutations of their domains). Total 3,618,560 native executions independently compare complete exposed register state, flags, PC/SP. Other register values are derived, not a Cartesian domain. No proof of arbitrary parallel maps or all memory footprints claimed.

110 exact ASM/BIN pairs retained under emitted/. Native matrix PASS 0.382s. Broad actual-Z3 assert/codegen/indexed/island/adapter family PASS 6.897s before optional artifact export was added; final native rerun includes export. No new size/speed benefit claimed. Research full 7bfe4476 and production cbf01110 were already live and predate this fix; neither restarted.
