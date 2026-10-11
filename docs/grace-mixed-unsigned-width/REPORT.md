# Primitive unsigned width: word shortcut no longer truncates wider operand

Base `fcba63cd`, including `48e5fe6d`. Only production change: eleven lines
in Nanz `resultTy`, before the historical u16 shortcut. Unsigned non-shift
primitive arithmetic selects the wider integer operand, including u24/u32.

## Evidence and policy boundary

Current `resultTy` explicitly states that the widest integer operand determines
operation width. The accepted `48e5fe6d` implemented that rule for a wider signed
RHS but placed it after a u16 shortcut. Therefore `u16(0)+u32(65536)` selected
u16, yielding zero already in raw MIR VM. u24 has the same defect; OR can instead
preserve the high bits in VM while losing them in native code. `before.txt`
contains actual before-fix source results/ASM, not a hypothetical type argument.

The independent expectation embeds both unsigned operands into max(widths),
performs the operation and reduces modulo 2^width. This documents the current
accepted implementation rule; it does not claim the Language Book specifies a
complete implicit-conversion policy. Book v7 §2.9 specifies explicit casts and
§3.1 defines widths. Older `Nanz_Language_Book.md:333` explicitly says there is
no implicit widening. That historical statement and current accepted compiler
behavior need a separate language specification reconciliation.

No new C promotion policy is introduced. Mixed signedness, equal-width signed
choices, shifts, pointers, operator overload dispatch and comparisons keep their
existing implementation. Their regression controls are preservation checks,
not an endorsement of every historical behavior. In particular u8<<u16 still
has a count-driven u16 result; that unresolved policy is not changed here.

## Independent executable proof

`TestMixedUnsignedWidthBothOrders` parses ten complete sources: u16 mixed with
u24/u32 under +,-,&,|,^, both operand orders. Eight boundary inputs include
65536, 0xffffff, 0xffffffff, subtraction underflow and equal operands. Fresh raw
VM, final VM and assembled native execute the original zero-argument compute
entry. Both full-width result globals are checked against host unsigned models.
Native guards cover all remaining RAM, exact typed compiler spill ranges,
bounded actual stack, original executable bytes and balanced PC/SP return.
Ordinary, Grace Speed, Grace Size: 240 workload cases, two results per case.
Every complete source/mode ASM/BIN/MIR is retained; no corpus count is increased.

Parser test enumerates all sixteen unsigned operand pairs across eight
primitive operators (128 type cases), with mixedsigned/shift/pointer/overload
controls. Existing signed-literal regression remains in the focused gate.

## Separate comparison audit, unchanged production

Unsigned narrow/wide both-order comparison at 65535 and 65536 currently agrees
raw/final/native: [1,1,1] including explicit widened control. genCmp32 already
pads missing unsigned operand bytes with zero.

Signed i16(0xffff) versus i32(0) exposes a distinct issue:
`narrow<wide`, `wide>narrow`, explicit `i32(narrow)<wide`:
raw/final [1,0,1], native [0,0,1]. lowerCond selects SrcTy/signedness from lhs;
genCmp32 pads the narrow operand with zero instead of sign extension. Book v7
§3.2 explains same-type signed comparison, but does not prescribe a common type
for mixed widths/signedness. Exact audit programs and generated ASM are kept in
`comparison-audit*` and `signed-comparison-audit*`; no expectations were edited
and no mixed-comparison compiler fix is piggybacked on this unsigned change.

Terminal gates: full Nanz PASS14.868s; full HIR PASS29.120s; focused mixed-width
and existing signed-literal native regression PASS1.811s. No full slow pipeline rerun.
