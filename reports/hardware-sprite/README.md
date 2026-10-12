# Compile-time masked sprites

`stdlib/hw/sprite.nanz` takes a static destination layout, byte-aligned rectangle
and hexadecimal keep-mask/graphics rows. It computes row addresses and decodes
constant bytes on the Nanz compile-time VM. Runtime operation is exactly
`new = (old & keep_mask) | graphics`.

Transparent ff/00 bytes are omitted, opaque mask 00 uses direct stores, mixed
bytes perform read/AND/OR/write. Destination is linear/strided or mapped ZX.
Pointer form emits ordinary Nanz. Compiled form emits one closed ASM region per
nontransparent row, with precomputed address and INC HL progression; AF/HL are
preserved using four normal stack bytes. No IFF changes, SP borrowing or SMC.
Caller owns destination writable RAM disjoint from code/stack.

| Ordinary mode example | Pointer bytes / T | Compiled bytes / T |
|---|---:|---:|
| Mixed | 46 / 198 | 38 / 208 |
| Strided | 74 / 313 | 69 / 398 |
| Opaque | 21 / 90 | 19 / 120 |
| Fully transparent | 1 / 10 | 1 / 10 |

The compiled strategy saves size on these examples but loses time because it
preserves registers. This is an explicit choice, not an automatic selector or a
claimed universal speed win. Timing includes preservation and RET, excludes
external CALL and physical ULA/IRQ costs. Larger scenes need their own measures.

Tests compare two changing backgrounds with independent masks/address oracle,
whole native RAM, exact PC/SP, compiled primary register/flags preservation.
Pointer is also tested on raw/final MIR2 VM. Cases cover transparent/opaque/mixed,
ZX rows 7/8 and 63/64, linear boundaries/end 65535, empty regions, case-insensitive
hex, omitted primitive. Invalid descriptions check dimensions, hex, row lengths,
missing/duplicate fields and arity. Native/source/ASM/BIN/result packets retained.

Current limitations: byte-aligned, known position and mask/data. Dynamic-position
sprites, pixel shifts and clipping are not yet promised by this API. Those can
be added as specializations while preserving the explicit mask semantics.
