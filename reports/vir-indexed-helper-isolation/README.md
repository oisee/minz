# Direct native helper isolation for indexed-array probe

Rejected same_pair+word-CFG-width candidate exported without rewriting original program. Native helper probe seeds25 board bytes with40+i, calls actual board_set/board_get labels under their declared A/C/B ABI, then checks results and exact PC/SP. Baseline returns7,9,255 for(0,0),(3,2),(4,4). Candidate returns7,60,0. Both returnPC/SP are correct.

Candidate board_get saves x into IXH, computes5*y, then emits LD L,A; ADD A,L rather than adding saved x. Thus offset becomes10*y (20 instead13 at3,2), selecting seeded byte60. The zero case masks the defect. Candidate board_set ASM computes5*y+x separately; direct memory-store verification remains necessary before claiming it correct. Mixed caller alone is not the root of this mismatch.

All experimental compiler changes reverted. Actual baseline/candidate source, MIR, ASM and allocation retained, with standalone native probe. Next work must reconcile vreg identity and inserted per-instruction moves before enabling free same-pair candidates. Current full production gate remains running.
