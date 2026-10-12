# Original CP/M and TUI Tetris computational mechanics

Two exact original modules are retained without source edits: `examples/nanz/tetris_cpm.nanz` and `examples/nanz/tetris_tui.nanz`. `freeze.json` pins each SHA256, imported production TUI dependency, geometry, and independent workload domains before execution. This packet does not cover `examples/zx/tetris.nanz` or archived ex34 by association.

The independent model uses explicit source-published tetromino cell data (seven types × four rotations × four ordered cells), frozen before compilation. These are data authority for these games, not data read from generated HIR/native tables. Four occupied lattice cells define each shape; source enumeration is kept because collision short circuit and lock writes observe cell order. Model arithmetic uses byte/word modular truncation; Book v7 §3.1 numeric widths, §2.6 pointers, §2.5 ordinary `for i in 0..n` syntax and §6.2 distinct countdown `range` semantics are referenced. Ordinary for visits ascending half-open indices; it is not countdown range.

Twelve original entries per source/mode:

- `rng_next` and `random_piece`: all 65,536 initial LFSR states each; independent right-shift/XOR 0xB400 recurrence and biased 7→0 choice mapping.
- `piece_dx`, `piece_dy`: all 112 valid geometry entries each.
- `board_set`, `board_get`: all 200 valid cells × all 256 byte values each, with independent row-major address and exact single read/write.
- `score_lines`: all 256 cl values × eight frozen score/lines/speed tuples, including word wrap and 0/1/2/3/255 speed. Values above four still update line count exactly as original source, without inventing scoring policy.
- `can_place`, `lock_piece`, `try_rotate`, `ghost_y`: seven types × four rotations × eight boundary positions × three independent boards, 672 calls each. Includes empty, sparse obstacles, bottom wall, and byte coordinate wrap at 254/255. Rotation model follows the original ordered no-kick/+x/−x/−y attempts, not a substituted standard SRS implementation.
- `clear_lines`: 24 boards with isolated/top/bottom/alternating/all full rows. Independent bottom-up row compaction, exact copied bytes and read/write order; covers four and twenty line deletion.

Per original source and compilation mode this is 238,456 actual entry calls. Ordinary/Grace Size/Grace Speed parse and emit the entire original module. Terminal/device functions and imported ASM stay in the full image, but instruction fetch is restricted to the twelve computational bodies and every opaque/indirect/foreign call in this closure is refused. The actual allocated ABI initializes entry registers; no constant driver replaces arguments.

Raw and final VM compare returns, exact ordered board reads/writes, every global byte, and the whole 64 KiB heap. Native compares the same independent result, complete board state/order, all 64 KiB outside exact private spill bytes and actual stack scratch, original return PC/SP, and unchanged full code image. Positions wrap modulo 256 before game bounds checks; board allocation remains its original global location, so this is not a relocated board crossing address 65535 proof. VM budgets are cumulative counters adjusted per call (five million instructions), native bounded to two million steps per call.

A genuine production bug was found: four full bottom rows gave raw/model 4 but final MIR 1 because preallocation coalescing merged a still-live outer row with a mutable inner counter. The standalone production correction and before evidence are separate in `reports/tetris-precoalesce` (167b66d3). No oracle or source was changed to hide it.

This is a bounded computational-entry proof. `spawn_piece`, `hold_piece`, composed `do_lock_and_clear`, rendering, input, game loop/main, BDOS/VT100 and external TUI runtime are not executed here. Complete interactive games are not claimed. No PGO improvement or runtime speed conclusion is inferred from a corrected wrong-code baseline.

Terminal full family result: **PASS 274.043 s**, all six source/mode combinations, **1,430,736 native calls and 2,861,472 raw/final VM calls**. Independent model controls PASS0.009s. Full-module native BIN lengths: CP/M4660ordinary/4663GraceSize/Speed; TUI4141ordinary/4144GraceSize/Speed. These are final image sizes, not a controlled speed/size improvement experiment. All per-entry counts and original SHA values are retained in six summary.json files.
