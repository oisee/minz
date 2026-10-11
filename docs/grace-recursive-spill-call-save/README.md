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
the forced-allocation native fixture by default. Native execution remains
disabled unless `MINZ_RECURSIVE_SPILL_NATIVE=1`; this checkpoint has not executed
the native oracle. Direct/mutual/indirect and multi-return coverage must be
completed before production integration.

Compile/VM/assembly focused check: PASS 0.015 s. An initial compile-only fixture
error (Verify returns error; Registers exposes A rather than AF) was corrected
before this check; no native execution occurred.
