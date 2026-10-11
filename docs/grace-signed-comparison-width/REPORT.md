# Native signed comparison follows MIR SrcTy

Base `53863716`. Production changes only `genCmp32` in z80codegen_cmp.go.

Original mixed source has i16 narrow=0xffff, i32 wide=0. Raw/final VM produce
[narrow<wide=1, wide>narrow=0, i32(narrow)<wide=1], while native produced [0,0,1].
The previous packet retained that diagnostic. This packet freezes its own full
before-fix matrix/source ASM in `before.txt`, including i8 byte-home refusal.

## Actual contract, not a new language promotion policy

VM `OpCmp` calculates `signExtend(value, inst.SrcTy.Width())` for each operand
when evaluating CmpLt/Le/Gt/Ge. The left shift then arithmetic right shift first
truncates higher bits at SrcTy width, then interprets that sign bit. It does not
first sign-extend a narrow operand from its own type when SrcTy is wider.

The backend now normalizes both snapshots exactly the same way: zero-pad
missing physical operand bytes below SrcTy width, discard original bytes above
SrcTy width, and fill higher comparison bytes from the SrcTy sign bit. This
preserves the current asymmetric source behavior. The inverse comparison stays
false until the source explicitly widens narrow, or a separate language decision
changes frontend common-type selection. Book v7 §3.2 describes same-type signed
comparison; general mixed-width policy remains unspecified. No frontend edits.

Eq/Ne compare complete raw values in VM. Unsigned comparison also retains raw
unsigned values. Neither receives the signed truncation/fill transformation.

Wide comparison already snapshots both operands before writing BC/IX scratch.
It previously refused a narrow byte register when paired with wide: supported
main/index byte homes now enter the same snapshot. If the byte is in A, it is
read from original stacked AF even when it is the second operand; scratch A
cannot replace the argument accidentally. Existing pair/memory paths remain.

## Independent checks

`TestSignedMixedComparisonMatchesSourceWidth`: eighteen complete sources,
i8/i16/i24 versus i32 under <,<=,>,>=,==,!=. Nine bitpattern inputs include sign
boundaries, truncated wide high bits, equal raw values and 0xffffffff. Three
original entries in each source body compute both operand orders and explicit
widened control. Independent host formula models SrcTy exactly, including
raw-value equality, instead of assuming both orders mathematically equivalent.
Fresh raw VM, final VM, assembled native under ordinary/Grace Speed/Grace Size:
486 workload invocations with all three result bytes; remaining RAM, exact typed
spill storage, bounded actual stack, code bytes and PC/SP checked. All complete
sources and 54 mode ASM/BIN/MIR sets retained; no corpus count inflation.

`TestNativeMixedComparisonSrcTyAndLiveHomes`: SrcTy8/16/24/32 × operand8/16/24/32
× signed four relational conditions/Eq/Ne/Ult/Uge =128 type/condition combinations ×both orders =256 physical configurations.
Eleven × seven independent input bitpatterns each: 19712 native calls. Direct
emitter proof checks original A (including byte operand in A), HL/DE/BC main and
shadow banks, AF shadow, IX/IY, exact RAM, code and balanced PC/SP. Comparison
flags are captured through the existing complete predicate materializer. The
full existing wide-comparison suite separately retains carry/Z/live-home checks.

This fixes execution agreement with the current MIR semantics; it does not
claim a mathematically symmetric implicit signed-promotion policy.

Terminal full MIR2 suite PASS73.735s (existing full wide comparisons included).
Source matrix PASS3.569s; final both-order physical matrix PASS in physical.txt.
No full slow pipeline restarted.
