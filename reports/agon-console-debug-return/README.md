# Console Agon programs: exact MOS return contracts

Both unchanged console examples in ordinary/Grace Size/Grace Speed, same public CLI BINs as full-VDP console stream replay. Real eZ80/MOS/fullVDP execute; debugger captures entry/preRET/afterRET registers and24 caller-frame bytes. Exact originalSP/IY/IX and frame must match, HL0, ADL1; RET must reach the little-endian saved returnPC and increaseSP by3. Repeated normal SDL A prevents pauses from losing keyboard input.

This is fixed early-exit return proof, not fullRAM, independent framebuffer or hardware execution. Console output is checked in a separate fullVDP stream matrix on identical BINs. Run scripts/agon_debug_return_replay.py --programs console with retained matrix and tool paths. Commands/transcripts/source/BIN/hashes retained; assertions use entry snapshots rather than fixed firmware frame values.
