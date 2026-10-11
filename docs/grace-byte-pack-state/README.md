# Byte packing: implicit flag consumers and current physical state

Base 91e2a9e1. A confirmed counterexample required tightening the new production
byte packing rule. A word OR can directly feed TermCondRet's condition, which
uses NZ flags even though its result is allocated to HL rather than F. Starting
with Z=1, low=1/high=0, the packed LD L,C left Z unchanged and RET Z returned 0
instead of 1. SP was balanced. native-before.txt records the actual assembled
failure; the restored baseline OR sequence sets NZ and passes after the fix.

Packing now refuses any result forwarded into implicit BrIf/CondRet conditions
or flag-return slots. Forwarding follows MIR OpMove and ordered block argument
edges to their parameter identities, including cross-block paths. This is a
semantic terminator check, in addition to live physical F and pending predicates.
TermBrIf2 emits a new CP itself and does not consume prior OR flags. Explicit
OpCmp consumers likewise get their own flags. This fix preserves baseline
emission rather than claiming to fix all generic word truthiness semantics.

Physical state audit:

- genDisjointByteOr reads current g.loc mappings, including physOverride.
  A relocated low operand in DE emits LD L,E; a high operand relocated away
  from the result pair and spilled low input are refused.
- A live pending accumulator remains A; packing writes only the primary pair's
  low byte and does not create an A scratch or alter pendingAccReg. Native
  snapshot controls confirm A=99 and SP preserved while packing both normal
  and overridden sources.
- Earlier byte accesses, ext and shift definitions are not deleted or moved.
  Existing producer scratch/call save handling remains responsible for their
  live values; packing neither extends lifetime nor creates a new interference.
- An ordinary source keeps both operands across perturb(lo), then packs and
  adds its return value. A second source packs then compares against zero.
  256 low inputs × five high values × two functions = 2,560 independent VM/
  native result-and-SP cases pass. These are bounded producer/call/condition
  evidence, not proof for arbitrary opaque asm or invalid allocations.

Full MIR2 PASS (10.178 s), pkg/... cmd/... compile PASS. Exact unchanged FAT12
helper still passes exhaustive 131,072 word/address cases in both native
versions plus VM, ordered reads and denied-second-byte prefixes. Its benefit
remains 98 to 78 T and 21 to 16 bytes. Original memory-fusion boundary is unchanged;
this remains packing of computed words, not widened access semantics.

Next useful bounded ISA/ABI work is production word boolean/truthiness handling:
the direct OR condition baseline observes flags of the last low-byte OR, not
necessarily the full word. A high-only word should be included before extending
this test into an all-word language guarantee. That issue predates packing and
is not concealed by the preserved-baseline counterexample used here.
