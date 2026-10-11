# Recursive spill caller-save draft

Compiler spill labels have one static home per function and register. A nested
activation can overwrite a value that its caller still needs. This isolated
draft snapshots caller-owned live byte/word homes on the activation stack before
register saves and argument copies. It restores them after result pickup. Result
definitions and dynamically relocated homes are excluded.

`EX (SP),HL` swaps the snapshot with the original HL value, so snapshotting and
restoration preserve HL, A, flags, and the remaining registers. The existing
register-save/result frame offsets remain unchanged. Calls with no live memory
homes emit no additional instructions. Builtins and tail calls are unchanged.

This does not expand the closed memory-host native frame certificate. Its
decoder may reject the new stack exchange form; no numeric stack whitelist is
introduced. The draft handles only one/two-byte Z80 homes.

`TestRecursiveCallSpill` verifies typed MIR and independent raw VM triangular
sums at depths 0, 1, 3, and 7 for byte and word homes. It compiles and assembles
the forced-allocation native fixture by default. Native execution is gated by
`MINZ_RECURSIVE_SPILL_NATIVE=1`. The corrected eight direct cases passed once,
followed by three typed mutual/indirect reentry cases and one modeled post-pickup
register/flag preservation case. The latter does not exercise actual multi-return
pickup. Conditional calls have scanner coverage, rather than native execution.

The TSMC scanner retains data homes for values live across direct, indirect,
conditional calls and opaque assembly. New call results and last-use arguments
remain eligible for the existing TSMC optimization. Actual current named homes
include generated spill overrides; native widths use `z80SpillBytes`.

The first native attempt stopped at depth zero because the fixture expected the
HALT PC to advance; this default emulator mode leaves PC at the opcode. Its
results and SP were correct. Both failed cases, that executable hash, and logs
remain in the packet. The corrected oracle and subsequent reentry run use
separately preserved executables. No runtime algorithm failure was observed.

Saved results are in `evidence/`: full assembly, binary, source MIR and state for
the direct fixtures, complete allocation/symbol records for typed reentry, and
the modeled result-preservation assembly/state. Each text copy is deterministic
gzip. `manifest.json` records both uncompressed and stored hashes. Original ELF
files remain in the permanent checkpoint directory; their hashes are recorded.

Actual direct eight: PASS 0.01 s. Additional reentry/result four: PASS 0.01 s.
All observed SP/PC values, independent triangular sums and flags/registers agree.
This is ordinary Z80 emulator validation, not a closed memory-host frame or CP/M
certificate. `EX (SP),HL` requires a separate closed native decoder extension if
that runner encounters these snapshots.

Compile/VM/assembly focused check: PASS 0.015 s. An initial compile-only fixture
error (Verify returns error; Registers exposes A rather than AF) was corrected
before this check; no native execution occurred.
