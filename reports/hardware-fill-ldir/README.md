# Register-preserving LDIR fills

`@fill` now accepts explicit `via "ldir"`; the default remains `"pointer"`.
The same compile-time layout, region and bounds validation generates a seeded
native fill. Empty regions emit no code. A singleton emits one store; larger
spans seed one byte and propagate it using LDIR with BC=length-1.

The ASM block preserves AF/BC/DE/HL, IX/IY, and restores SP. It does not change
IFF or use alternate registers. It uses at most eight bytes of normal stack,
which must be disjoint from the destination and available to the caller.
Initial use of clob-all ASM failed the live-value test; that version was not
shipped. The explicit saves/restores are included in all final timing below.

| Workload | Pointer T | Preserving LDIR T | Pointer / LDIR bytes |
|---|---:|---:|---:|
| 1 byte | 73 | 51 | 13 / 8 |
| 255 bytes | 10487 | 5463 | 13 / 22 |
| 256 bytes | 34901 | 5484 | 30 / 22 |
| 6144-byte ZX bitmap | 835669 | 129132 | 30 / 22 |

Counts include setup, register preservation and RET; exclude external CALL,
interrupts and ULA contention. Full bitmap is approximately 6.47× faster with
smaller code. Short fills have a different size tradeoff; there is no automatic
selection or universal speed claim.

13 scenarios × 3 modes pass against an independent physical-address oracle,
including zero/one/two bytes, page/end boundaries, mapped ZX rows and full screen.
All native RAM is checked except precisely declared spill bytes and the eight-byte
stack interval used by saves. Primary register/flags preservation and PC/SP are
checked. A separate 10-input × 3-mode test computes a live word before two fill
blocks and consumes it afterwards. The original pointer VM/native matrix and 23
negative description tests also pass.

VM cannot execute this ASM; native results are compared with the independent
model, not with skipped ASM VM execution. Raw/final VM checks remain on the
pointer variant. Source, ASM/BIN and final per-case timing are saved in native/.

Implementation: ../../stdlib/hw/fill.nanz. Tests: TestHardwareFillLDIRNative,
TestHardwareFillLDIRLiveValues, TestHardwareFillMemoryNativeAndVM.
