# Grace conditional returns retain comparison flags

After the independent indexed-shift repair, original FAT read_fat12 still miscompiled under GraceSize: BIT 0,C checked cluster parity, then a hoisted raw & 4095 emitted AND instructions before RET Z. Cluster 4 returned 3531 instead of 3263. Raw and final VM preserved the intended condition; native flags did not.

The Grace condRetSink action omitted hoistedConditionFlagsSafe, already enforced by the Go CondRetSink pass. It now uses that same legality check before any provenance or CFG mutation. Flag-changing pure instructions remain in their branch; the supported matching subtraction/carry-fusion case remains admitted. No loads/stores or packed-nibble source semantics change.

A focused independent VM and assembled Z80 regression checks all 256 inputs for BIT/parity, equality/Z, and unsigned carry conditions: 768 cases with a later masked-word return and balanced SP. The old action reproduces rejection-test failure. Another 512 assembled BC/DE memory-bit cases prove unencodable BIT-memory patterns retain the original load/mask fallback. The existing positive Grace test formerly expected an unsafe reversed subtraction; its operands now match the documented legal carry fusion, while new negative cases test rejection directly.

All 108 prospectively frozen original FAT12/FAT16 read/write calls pass under default, GraceSize, and GraceSpeed. Checks include independent packed semantics, raw/final VM, actual byte read/write counts and order, full 64 KiB memory/code, FFFF wrap and SP. Full MIR2 package passes. These are correctness results, not a measured selector winner: controlled ProfileWorkload admission still rejects unreachable disk extern declarations until its separate conservative closure implementation lands.

Exact original source/freezes and previous failing native images are retained. Artifacts: reports/fat-packed-prospective/flags-proof and flags-fixed; dated share /mnt/safe/minz/grace/2026-10-11/fat-packed-prospective.
