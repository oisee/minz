# Z80 locations

`z80spec` is an independent description of storage, with no compiler or emulator
imports. No production allocator uses it in this slice. `Locations` returns a
copy; `Lookup`, `Width`, `Overlaps`, `Aliases`, `Contains`, `Halves` (high, low), and `Parent` operate on
case-sensitive canonical names. Unknown names do not overlap and have width zero.
Lookup uses a name-to-index map, including for overlap queries. `Aliases` returns
all overlapping locations (including self) in table order. `Contains(outer, inner)`
is directional and reflexive for known names; unknown names are never contained.
Overlap describes shared physical storage, not instruction-prefix compatibility.

The table contains the primary bytes A/B/C/D/E/H/L/F, BC/DE/HL/AF, SP, I/R/PC, IX/IY and
all four index halves; shadow bytes and pairs; tsmc0–7, mem0–3 and stk0–3; generic
MIR2 `mem`/`stack` sentinels; GPU `gpu_mem`; and BC32/DE32/HL32. DWord composites
contain the main and shadow pair, and overlap both pairs and all four bytes.
`Halves`/`Parent` describe sixteen-bit pair/byte containment only. SP has no
modelled halves and has its own StackPointer kind. I is Special8; R is Refresh
(eight bits stored, with a seven-bit incrementing counter and preserved top bit);
PC is ProgramCounter. F is physically eight bits, even though IR flag values are
one-bit booleans. Independent byte-storage masks suffice here because each
modelled overlap covers entire bytes; named flag bits are deferred to S1.

## Namespace views

| Namespace | Mapping | Locations without a counterpart |
| --- | --- | --- |
| LIR | 0–36; canonical names unchanged | I/R/PC, stk0–3, generic mem/stack, gpu_mem, AF, shadow pairs, F', DWord composites |
| VIR | 0–40; `_vir_mem0`–`_vir_mem3` become mem0–3 | I/R/PC, generic mem/stack, gpu_mem, AF, shadow pairs, F', DWord composites |
| GPU | 0–9 = A–L, BC/DE/HL; 10–13 = IXH/IXL/IYH/IYL; 14 = gpu_mem | I/R/PC, SP, IX/IY, F, all shadow/SMC/numbered spill/stack locations, generic MIR2 sentinels, AF, DWord composites |
| MIR2 | `mir2loc.From`/`To`, preserving Kind and Name; LocDWord BC/DE/HL become BC32/DE32/HL32 | I/R/PC, SP, AF, shadow pairs, F', all SMC/numbered spill/stack slots, gpu_mem |

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
edges); all other edges agree. Every returned alias is checked, even outside Z80PhysLocs,
and every known-gap key and target must remain in Z80PhysLocs. The width/kind
disagreements above are also enforced by assertions.

The emulator cross-check executes LD, PUSH/POP, EXX and EX AF,AF' through
RemogattoZ80's FUSE-verified core. It checks pair-to-half instruction reads,
half-to-pair instruction writes, AF stack byte order, and preservation of both
shadow banks for BC/DE/HL/AF. LD I,A / LD A,I and LD R,A / LD A,R check the
special registers, including R's seven-bit rollover and preserved top bit.
Instruction execution also checks PC advancement. Wrapper getters only observe
instruction results; they do not establish the containment relation themselves.
