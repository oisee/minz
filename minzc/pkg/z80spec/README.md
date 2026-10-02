# Z80 locations

`z80spec` is an independent description of storage, with no compiler or emulator
imports. No production allocator uses it in this slice. `Locations` returns a
copy; `Lookup`, `Width`, `Overlaps`, `Halves` (high, low), and `Parent` operate on
case-sensitive canonical names. Unknown names do not overlap and have width zero.
Overlap is physical containment, not instruction-prefix compatibility.

The table contains the primary bytes A/B/C/D/E/H/L/F, BC/DE/HL/AF, SP, IX/IY and
all four index halves; shadow bytes and pairs; tsmc0–7, mem0–3 and stk0–3; generic
MIR2 `mem`/`stack` sentinels; GPU `gpu_mem`; and BC32/DE32/HL32. DWord composites
contain the main and shadow pair, and overlap both pairs and all four bytes.
`Halves`/`Parent` describe sixteen-bit pair/byte containment only. SP has no
modelled halves. F is physically eight bits, even though IR flag values are
one-bit booleans. Independent byte-storage masks suffice here because each
modelled overlap covers entire bytes; the package does not model individual flags.

## Namespace views

| Namespace | Mapping | Locations without a counterpart |
| --- | --- | --- |
| LIR | 0–36; canonical names unchanged | stk0–3, generic mem/stack, gpu_mem, AF, shadow pairs, F', DWord composites |
| VIR | 0–40; `_vir_mem0`–`_vir_mem3` become mem0–3 | generic mem/stack, gpu_mem, AF, shadow pairs, F', DWord composites |
| GPU | 0–9 = A–L, BC/DE/HL; 10–13 = IXH/IXL/IYH/IYL; 14 = gpu_mem | SP, IX/IY, F, all shadow/SMC/numbered spill/stack locations, generic MIR2 sentinels, AF, DWord composites |
| MIR2 | `mir2loc.From`/`To`, preserving Kind and Name; LocDWord BC/DE/HL become BC32/DE32/HL32 | SP, AF, shadow pairs, F', all SMC/numbered spill/stack slots, gpu_mem |

LIR and VIR indices 0–32 agree. At 33–36 only spelling differs. VIR adds stk0–3
at 37–40. GPU's production reverse table returns -1 for its spill sentinel;
its forward table omits the sentinel. It is not mem0. Similarly, MIR2's generic
mem/stack allocation candidates do not identify any particular numbered slot.
These abstract locations have distinct storage, nominal width 16, and no inferred
alias edges. Dynamic MIR2 memory addresses, stack frame offsets and nonzero index
offsets are outside this finite table and are rejected by the adapter.

The MIR2 adapter lives in a child package to permit a future MIR2 dependency on
the base spec without a cycle. The base index converters likewise do not import
LIR or VIR. External tests import all three IRs. Tests read the unexported VIR
pair/GPU initializers with Go's AST parser rather than changing production code
or using unsafe linkage; changes to initializer structure must update the tests.

## Complete current disagreements

Alias rows exclude self-overlap (the allocators already treat equal locations as
conflicting). Each arrow is directed. Listed gaps are asserted explicitly: a
new gap or a fixed known gap fails the tests until this inventory is updated.

| Namespace | Location | Difference from spec |
| --- | --- | --- |
| LIR Alias | B | Missing B → BC |
| LIR Alias | C | Missing C → BC |
| LIR Alias | BC | Missing BC → B, C |
| LIR Alias | D | Missing D → DE |
| LIR Alias | E | Missing E → DE |
| LIR Alias | DE | Missing DE → D, E |
| LIR Alias | H | Missing H → HL |
| LIR Alias | L | Missing L → HL |
| LIR Alias | HL | Missing HL → H, L |
| LIR Alias | IX | Missing IX → IXH, IXL |
| LIR Alias | IXH | Missing IXH → IX |
| LIR Alias | IXL | Missing IXL → IX |
| LIR Alias | IY | Missing IY → IYH, IYL |
| LIR Alias | IYH | Missing IYH → IY |
| LIR Alias | IYL | Missing IYL → IY |
| VIR pairAliases | IX | Missing IX = IXH + IXL |
| VIR pairAliases | IY | Missing IY = IYH + IYL |
| MIR2 physicalAliases | IX | Missing IX → IXH, IXL |
| MIR2 physicalAliases | IXH | Missing IXH → IX |
| MIR2 physicalAliases | IXL | Missing IXL → IX |
| MIR2 physicalAliases | IY | Missing IY → IYH, IYL |
| MIR2 physicalAliases | IYH | Missing IYH → IY |
| MIR2 physicalAliases | IYL | Missing IYL → IY |
| MIR2 physicalAliases | B' | Missing B' → BC32 (reverse exists) |
| MIR2 physicalAliases | C' | Missing C' → BC32 (reverse exists) |
| MIR2 physicalAliases | D' | Missing D' → DE32 (reverse exists) |
| MIR2 physicalAliases | E' | Missing E' → DE32 (reverse exists) |
| MIR2 physicalAliases | H' | Missing H' → HL32 (reverse exists) |
| MIR2 physicalAliases | L' | Missing L' → HL32 (reverse exists) |
| LIR/VIR width | F | IR boolean width 1; physical width 8 |
| LIR/VIR kind | HL | LocIndex models pointer use; spec Pair16 models storage |
| LIR kind | IXH, IXL, IYH, IYL | LocIndex; spec IndexHalf (VIR already distinguishes these) |
| VIR spelling | mem0–3 | `_vir_mem0`–`_vir_mem3`; canonical mem0–3 |
| GPU spill mapping | 14 | Reverse VIR index -1, omitted forward; spec independent gpu_mem |

All 37 LIR Alias fields are empty. Fifteen locations therefore lack a total of
20 directed containment edges; the other 22 locations have no non-self overlaps
within that namespace. VIR's three declared BC/DE/HL aliases agree exactly.
MIR2 has 14 missing directed edges (eight index edges and six shadow-to-DWord
edges); all other edges among Z80PhysLocs agree.

The emulator cross-check uses RemogattoZ80's FUSE-verified core via SetRegisters,
GetRegisters and SetRegister8 for BC/DE/HL/IX/IY/AF, including both half writes
and preservation of the opposite half loaded through a pair write. Its public
wrapper does not expose shadow-byte setters/getters, so shadow containment is
covered by spec properties and the DWord comparison, not this emulator check.
