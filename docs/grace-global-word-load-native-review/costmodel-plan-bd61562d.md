# Minimal global word-load backend experiment

No source rewrite, new HIR certificate, broad peephole or pointer optimization.
Extend the existing exact direct-global u16 OpLoad matcher/emitter from HL to
DE and BC. Required existing premises remain: width exactly 16, actual
OpAddrOf of a known module global in the same block, address register used
solely by that load, destination exactly HL/DE/BC. Match and emission must agree
before suppressing the AddrOf. Keep all other pointers, offsets, multiple uses,
spills, IX/IY and unresolved addresses on existing paths in this first slice.

Current cur has two confirmed 36-T sequences:
LD HL,symbol (10), LD E,(HL) (7), INC HL (6), LD D,(HL) (7), DEC HL (6).
Replace with LD DE,(symbol), ED5Bnnnn (20 T, 4 B); BC uses ED4Bnnnn.
Original six-byte sequence becomes four bytes. The recorded executions are
1469 for pos and 1444 for src_buf. Arithmetic (1469+1444)*16 = 46608 T is a
local instruction-count estimate only. Whole-program speed, sizes, changed
allocation or other eligible sites must be measured, not inferred.

Legality: LD rr,(nn) reads low byte at nn then high byte at nn+1, matching
original little-endian ordered byte reads. It changes only destination rr;
A, F, HL and other registers are preserved. Original address register material-
ization can be removed because it has no other IR use. No asynchronous memory
or arbitrary external callback is introduced between the source OpLoad bytes.
Globals are compiler-owned complete storage; this is not a wide resource-read
substitution with invented partial-trap semantics. Numeric/resource pointer
loads remain unchanged.

Native permission emission must use existing emitNativeMemoryf(inst,0,2) for
the same MIR-rooted global symbol/offset/width. The closed resolver already
recognizes ED4B/ED5B word-read opcode/address/width; verify actual native runtime
consumes the two ordered byte permissions and reads exactly the rooted bytes.
A direct opcode with numeric wrong address, wrong direction/width, or a stale
symbol sidecar must still refuse before access. No permissions widening needed.

Generator cache/liveness: invalidate only the written destination pair and its
byte aliases; keep A/F/HL cache knowledge consistent with actual unchanged
registers. If the address pointer was allocated in that same pair, useCount1
makes its old physical value dead; load defines the destination directly.
Do not retain stale value cache for destination bytes or suppress a separately
needed AddrOf used by another load/call. Known globals with unsupported size or
symbol collisions must use existing verified storage constraints.

Meaningful machine tests: source/raw/optimized MIR expectations for two known
u16 globals forced into DE and BC, plus a live A/flags/HL value across the load;
exact low-then-high data traffic, no third read/write, little-endian results,
whole RAM unchanged apart from declared stack; actual ED opcode at rooted
symbol address. Test multiple-use AddrOf and numeric-pointer cases remain
unoptimized. Byte boundaries/values 0,255,256,65535 cover value semantics.
Internal forced-allocation tests can compare pre/post registers/AF exactly;
source tests must actually execute MIR assertions and target binary.

Bounded actual caller experiment after design/source review: unchanged original
SelfLanz thirty-call workload, existing closed runner/full independent host
state/events and native sidecar/allocation binding, original plus already
selected both-transform variant if appropriate. New compiler executable must
collect fresh profiles; old supplied profiles cannot be relabeled current.
Preserve old measured PC/native evidence and explicit before/after binaries.
Native cost oracle is actual CPU T-states and step counts, not the local estimate.
No CPU execution has occurred in this independent design review.
