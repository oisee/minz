# Standalone upstream fix: simultaneous unsigned division workspace assignment

Base: fresh GitHub main `4743cb7c7712a0d76413ddf041d93dbfa604df06`, remote
main independently reverified unchanged on 11 October 2026. Donor `1fb9bada`
is mixed; this patch extracts only `genDivMod8` unsigned B/C setup. No pointer,
VM arithmetic, Grace, PGO or other divisor algorithms are transplanted.

The 8-bit restoring divider requires dividend in B and divisor in C. When the
allocator supplies dividend in C and divisor in B, sequential `LD B,C; LD C,B`
destroys the original divisor. On unchanged main the new native regression fails
immediately: `0 / 1 = 255`; the mod subtest also fails. Raw VM quotient/remainder
matches the independent host model and isolates the failure to native setup.

The patch assigns both operands simultaneously through main's existing
`emitParallelCopy`, preserving the existing pair-low-byte/spill handling. Signed
setup remains unchanged. The native swap becomes `LD A,C; LD C,B; LD B,A`.

After the patch, raw VM and assembled native execution match independent integer
quotient/remainder for all 256 dividends × 255 nonzero divisors, for both div and
mod (130,560 native cases). Native checks include actual call/return ABI, finite
200-instruction budget, terminal PC, restored SP, unrelated HL and E values,
guarded output and complete unchanged program image. A separate live-D test
consumes D after the operation for 256 values × eight boundary operand pairs ×
both operations (4,096 cases), proving the existing scratch-save wrapper still
protects a genuinely live division workspace byte.

Zero is explicitly excluded from valid native quotient/remainder evidence. The
Book v7 demonstrates quotient/remainder (`divmod(10,3)==(3,1)`, §7.4 (tuple assertions)) but does
not declare native zero-divisor trapping. Current MIR VM `OpDiv`/`OpMod` explicitly
error on zero; tests verify all 256 dividends error under both VM operations.
The native restoring algorithm has no trap/zero check. This pre-existing target
semantic difference is not repaired or certified by this narrow workspace fix;
no fabricated zero-divisor oracle is used to green the tests.

Focused exhaustive/live-D tests PASS, 0.193 s. Full MIR2 PASS, 3.761 s, and existing native
pipeline spilled-call/tuple-return/extern ABI smoke PASS, 0.026 s. Logs are retained alongside
before/after evidence and exact assembled div/mod ASM/BIN, with SHA256 manifest.
No full CLI, showcase or slow pipeline repeat. This pushed review branch
`fix/upstream-div8-workspace` does not merge main; the register-identity PR is
independent and is not included here.
