# Effect-safe invariant address experiment in MIR Grace

A general load fusion was investigated against unchanged FAT12 `ld_word`. It was not landed: an ordinary u16 VM load validates the complete range before byte zero, whereas two u8 loads may observe byte zero then trap on byte one. Reusing ordinary u16 would change this prefix. Removing ext/shift operations only in native code also changes original byte lifetimes and requires allocator support. Both gaps are retained as requirements, without introducing weakened memory semantics.

Instead, `HoistInvariantAddresses` proves a bounded natural loop, unique unconditional preheader and single latch using reachable dominators and predecessor closure. A loop header parameter is invariant only when its preheader argument is available and every backedge passes that same parameter unchanged. Only a modular 16-bit Add/PtrAdd address DAG, plus its pure constants/moves/zero extensions, is moved. The DAG must actually feed a memory address. Loads/stores remain identical instructions at the original positions. Globals are not read early. Inline assembly, calls, indirect calls and ports reject the loop; hard-class instructions are not moved. This conservatively avoids hard-register and cross-call save costs.

The original source `examples/nanz/grace_intern_checked.nanz` runs all 30 existing independent state/traffic workloads, including repeated interning, mismatches, maximum 255-byte strings, numeric wrap, persistent counter wrap and unread guard bytes. Its `region_eq` offset+1 computation is hoisted by facts, without checking the function's name. Actual production `CompileHIRSteps` compares modes after all other passes:

| Mode | Native T-states | Whole BIN bytes | New hoists |
|---|---:|---:|---:|
| ordinary default | 1 637 997 | 661 | 0 |
| MIR Grace + OptSize | 1 637 997 | 661 | 0 |
| MIR Grace speed | **1 611 362** | 669 | 1 |

Benefit is **26 635 T (1.63%) for 8 bytes**. Pure speed permits this exchange. Default and Size do not run the new pass. No optimum or universal profitability claim is made. This is an opt-in migration experiment, not completed automatic profile selection.

`RunGracePassesWithOptions` carries OptimizeForSize and an explicit test/research disable. Existing `RunGracePasses` callers keep the old behavior. The pipeline change is only its two MIR Grace calls, passing the real OptSize. `CompileHIRWithOptions` previously ignored UseGrace and is not changed in this commit; parent integration will address that separate API gap.

A broad call-loop prototype passed semantics but worsened unchanged self-Lanz original training from 1 016 414 to 1 115 064 T (+89 bytes). That result is saved in `rejected-call-loop`; call-loop hoisting is rejected in the delivered implementation. It must not be reintroduced merely because arithmetic is invariant.

Semantic regressions cover ordinary volatile byte-load callbacks, aliased store followed by load, unchanged denied-access callbacks, zero-trip no access, modulo-65536 wrap, truncated-heap low-success/high-trap prefix, variant-base rejection, mutating-call rejection and asm rejection. Original memory instruction objects and metadata are untouched. A real-source native `region_eq` check at arena=ffff exercises 0/1/3 bytes in speed and Size, and verifies return PC/SP through the existing strict harness. The independent Go interning oracle checks all return values, entire arenas and globals, every resource read/write count. Portable `audit.py` repeats those state/checksum/count checks on all three saved native outputs without compiler imports.

Validation: full MIR2 package passed in 8.006 s. Focused native program and wrap tests pass; frozen executable retained at `/home/alice/dev/minz-artifacts/grace-checkpoints/invariant-address.test`, SHA256 `d9b3834df50930139fbc62fbd3f441855bd785de7ded29820a1125317f03d4a2`. Run from `minzc/pkg/pipeline` with `-test.run '^TestInvariantAddress(RealInternProgram|NativeWrap)$' -test.v`. All artifacts retain source/workload, ASM/BIN, MIR, allocation, raw/native observations and hashes. No whole slow pipeline suite was run here.
