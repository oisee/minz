# Reusable affine texture walk verification

The library @texture_walk generates affine 8.8 sampling kernels from destination,
mutable texture page, runtime setup-symbol names and static unroll factor.
60 compiled variants (five layouts and all dividing unroll factors × 3 modes)
execute 480 independent runtime setup/texture mutations; exact output, all memory
outside destination and audited stack, and return PC/SP verified. Renamed setup
symbols, relocated output/page, singleton and final 0xff00 texture page are included.
Invalid names, dimensions, alignment, overlap, missing fields and arity reject.

| Unroll | Minimal module bytes ordinary | Kernel T ordinary |
|---|---:|---:|
| 1 |143|323907|
| 2 |219|315459|
| 4 |371|311235|
| 8 |675|309123|
| 16 |1283|308067|
| 32 |2499|307539|

Size mode outlines RLCA groups into __rotate_4 at factors 8/16/32. It saves
8/24/56 bytes but costs 350595/349539/349011 T. The test checks exact helper machine
bytes and instruction-observed minimum SP: 8 bytes usual, 10 bytes outlined. Thus
Size's extra stack/timing cost is measured, not inferred away. Direct kernel
counts exclude wrapper/setup; same model covers eight changing runtime datasets.

Dedicated and library demo actual main tests separately cover ROM IRQ, screen and
ULA. Library demo source lives at ../zx-rotozoomer-library/source.nanz. These
operation tests use native independent math; ASM is not a VM oracle.

TestHardwareTextureWalkNative / TestHardwareTextureWalkRejectsInvalidDescriptions.
MINZ_HW_TEXTURE_EXPORT retains source-derived ASM/BIN and result JSON. Exact test
source is retained as test-source.go. This report does not claim main integration.
