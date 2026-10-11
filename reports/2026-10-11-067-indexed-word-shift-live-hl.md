# Indexed word shifts preserve live HL

The unchanged FAT12 write_fat12 odd case (cluster 9, value 3445) computes keep = raw & 15 and shifted = value << 4. Native code placed keep in HL and the shift destination in IX, then staged the shift through HL without saving keep. The low output byte was 80 instead of the independently expected 90; raw and final VM agreed with the packed-nibble oracle.

Constant 16-bit IX/IY destination shifts now save independently live HL (including live H/L) before source staging and restore it after the indexed result is written. Ordinary HL destinations and variable shifts are unchanged. The additional PUSH/POP preserves flags and balances the stack.

A separate assembled native regression covers 180 cases: IX/IY × SHL/SHR/SAR × counts 0, 1, 4, 8, 15 × six word boundaries, retaining a nonzero HL value for a later OR. The previous implementation fails even at count zero; the corrected implementation passes, including SP. Full MIR2 package passes.

All 36 original prospective FAT12/FAT16 calls now pass in default mode, with raw/final VM, independent packed semantics, actual two-byte observations, full 64 KiB state, code and SP checks. GraceSize still fails read_fat12: BIT 0,C is followed by flag-changing AND instructions before RET Z. This is a separate required correctness fix; this packet does not claim all packed modes pass or a selected heldout winner. Exact artifacts retain the failing GraceSize image separately from historical first-fix evidence.

Artifacts: reports/fat-packed-prospective/shift-proof and shift-fixed. Prospective freezes and unchanged original source are retained. Existing public controlled selection still refuses unreachable disk externs; no admission relaxation is included.
