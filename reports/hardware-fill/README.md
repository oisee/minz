# Declarative byte fills on Nanz

Implemented in research branch `feat/grace-hir-vm` on October 12, 2026.
This follows the published hardware-array design with one small reusable operation.

`import hw.fill` provides a genuine Nanz metafunction. Its static block specifies
linear or ZX bitmap byte layout, region, traversal and value. It validates bounds
and computes row addresses on MIR2 VM at compile time, then emits ordinary Nanz
pointer loops. Runtime stores are checked against an independent physical-address
oracle on raw/final VM and assembled Z80.

12 positive scenarios × ordinary/Grace Size/Grace Speed pass, with whole RAM
comparison and exact native return PC/SP. Scenarios cover zero regions, 255/256
counter transition, page edges, address 65535, ZX rows 7/8 and 63/64, full screen.
20 invalid descriptions are rejected, including signed values, missing/duplicate
fields, wrong arity and overflowing numeric input.

The library exposed a missing type declaration for `node_arg_int`. The host now
has a declared i32 result and rejects numbers outside signed 32-bit range. An
assignment/bounds-check regression covers the difference between a real typed
numeric argument and an undeclared call that happened to return a VM value.

This is a correctness baseline, not a speed claim. Full 6144-byte bitmap fill uses
30 bytes and 835669 T in the focused native test; 255 bytes use 13 bytes/10487 T,
256 bytes use 30 bytes/34901 T. Counts include initialization and return. The word
counter path is costly; a later measured LDI/LDIR alternative can reuse these
contracts and independent oracles. No SP borrowing, IFF changes or SMC is emitted.

Library: ../../stdlib/hw/README.md. Example: source.nanz.
Test: TestHardwareFillMemoryNativeAndVM, TestHardwareFillRejectsInvalidDescriptions.
Native artifacts are retained with their source, ASM, binary and measured result.
