# Native packing of disjoint byte lanes

Base d970ed74. This is a useful production migration of the arithmetic portion
of the former load16-combining pattern. It does not combine memory operations.

The unchanged `stdlib/fs/fat12.minz:ld_word` reads lo, extends it, computes ptr+1,
reads hi, extends and shifts it by 8, then ORs the two words. Existing code
emitted six instructions for that final OR. Their operands have proven
nonoverlapping lanes: lhs high byte only, rhs low byte only. When lhs and the
result already occupy the same primary pair, packing requires just a copy of
rhs low byte to result low byte.

| Real helper | T-states including RET | Bytes |
|---|---:|---:|
| Base production | 98 | 21 |
| Selected production | 78 | 16 |

Saved baseline.a80 was independently compared byte-for-byte to production
compilation with the exact base ALU emitter restored; original-baseline.txt
records PASS. Selected ASM/BIN and original helper source are retained. No
source rewrite, widened load, instruction deletion or extended byte operand
lifetime occurs. Every earlier MIR instruction and emitted byte access remains.

The matcher is restricted to same-block u8→u16 extensions and an exact
unsigned shift by constant 8. Repeated definitions are refused. Both physical
inputs must be disjoint BC/DE/HL primary pairs; high input must already be in
the result pair. Result-overlapping live values/pair halves and live/pending
flags refuse the match. Index pairs, memory spills and unproved lane shapes use
the original emitter. The replacement has no scratch, accesses or stack moves.
It writes only the low result byte and preserves the proven high result byte.
Hard register constraints remain satisfied because allocation is not changed.

## Native and effect oracle

All 65,536 possible words at 0x8000 and 0xffff are executed on both original and
selected native helpers: 131,072 cases per native version. The ordinary MIR VM
also matches the independent little-endian word result. Pointer wrap reads
0xffff then 0x0000. The bounded target memory accessor checks each original
byte event, including order and exact values, and exact SP/return completion.
Denying the second byte must retain precisely the successful first-byte read
prefix for both versions; these refusal controls also pass at both addresses.

These tests preserve memory behavior by leaving all memory operations in place.
They do not reconcile VM widened-access prevalidation with native partial
access, or prove general byte load/store16 fusion safe. That migration remains
open. Accessing volatile/MMIO or interleaved effects is not moved by this rule.

Full MIR2 PASS, focused native/refusal controls PASS, supported pkg/... cmd/...
compile-only PASS. The new exhaustive native regression takes about 2–3 s;
actual duration is in focused-native.txt. SHA256 binds the saved artifacts.
The measured result is this helper and workload, not a whole FAT filesystem
performance claim. Default production benefits; compiler policies are unchanged.
