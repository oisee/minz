# Declarative row copying

`stdlib/hw/copy.nanz` implements @copy on the compile-time Nanz VM. Static source
and destination layouts may be linear/strided or ZX interleaved bitmap bytes.
The region declares source x/y, destination x/y, width and height. Row addresses
are computed before runtime; changing source pixels remain runtime data.

Both pointer loops and register-preserving LDIR blocks are selectable. LDIR uses
nonzero BC and preserves AF/BC/DE/HL; zero-size regions emit no stores. Nonempty
copies reject overlap of whole layout bounding extents, including gaps. This is
an intentionally conservative copy contract, not memmove. Layout bounds must
fit 16-bit address space. Caller owns writable destination and available stack
and keeps both layouts disjoint from active code/stack.

| Full linear raster → ZX bitmap | Bytes | T-states |
|---|---:|---:|
| Pointer, ordinary | 5183 | 411391 |
| Pointer, Grace Size | 5609 | 467623 |
| Preserving LDIR, all three modes | 3649 | 149962 |

The 6144-byte upload improves about 2.74× against ordinary pointer and uses 1534
fewer bytes. Counts include preparation, preservation and RET; ULA/IRQ excluded.
Grace Size worsens this large expanded pointer case; no automatic primitive
selection is claimed. Row-specialized LDIR itself has substantial setup code;
future grouped address walks can reduce it without changing copy semantics.

13 cases × 3 modes × 2 primitives × 2 changing source fills cover full upload,
page ends, 255/256 counts, end 65535, empty regions, ZX 7/8 and 63/64,reverse layout.
Raw/final VM checks apply to pointer only; native LDIR is checked directly against
an independent address oracle. Whole native RAM, preserved source, exact PC/SP
and LDIR primary registers/flags are verified. There are 27 malformed description
checks plus 10 dynamic live-value cases ×3 modes across two LDIR blocks.

Tests: TestHardwareCopyPointerMemoryNativeAndVM, TestHardwareCopyLDIRNative,
TestHardwareCopyRejectsInvalidDescriptions, TestHardwareCopyLDIRLiveValues.
MINZ_HW_COPY_EXPORT retains source, ASM/BIN and per-case/mode measurements.
