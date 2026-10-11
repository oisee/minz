# First native string fixture: saved-only diagnosis

Read the saved first-run ASM/BIN/output/registers in `grace-checkpoints/string-representation-evidence/actual`. No native execution repeated.

Three separate causes must not be conflated:

1. HALT termination oracle: bootstrap `test_done` labels DI at 0x0106 and HALT at 0x0107. Remogatto's halted PC stays at the HALT opcode. Actual PC 0x0107/SP 0xF000 with no run error is normal, so expected `done+2` is a fixture mistake.
2. Zero `answer`/`first`: `NewRemogattoZ80` uses `NewMemory`, whose default ROM boundary is 0x4000. Fixture globals lie below 0x4000 because the image loads at 0x0100. CPU writes there are intentionally ignored, while `GetMemory` reads the same backing store; it is not a stale snapshot. Use an explicit all-RAM fixture or move the image/globals into RAM. No generic store-codegen failure is established by these zeros.
3. Genuine S/L output error: the byte load is allocated to H (S) or D (L), and the ClassAcc move shares that physical location after coalescing. The move is consequently a no-op. The builtin `@mir.io.print.char` bypasses ordinary call argument setup and emits `OUT (0x23),A` directly. A still holds the live count/scratch value, explaining S output `[2,1]` rather than `AB` and L output `[0,0]`. Saved MIR includes `%r12 = move %r11 [acc]`, but ClassAcc is not a guaranteed physical A ABI after allocation/coalescing.

The narrow repair must materialize the actual allocated argument into A at the builtin boundary and preserve a different live A value through output. Merely inserting `LD A,H` without saving the live S count would fix one byte and corrupt subsequent iterations. Reuse existing physical move/call-save facilities with explicit builtin clobber and argument contracts where feasible. Keep the failed run and its artifacts; only rerun after the code and corrected fixture are frozen and reviewed.
