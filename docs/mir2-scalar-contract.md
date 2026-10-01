# MIR2 scalar oracle: admitted byte contract

Status: implemented test slice, 2026-10-01. This contract describes the MIR2
VM and `FoldConstants` for the operations below. It is not a claim about native
backends or all MIR2 operations.

Values are stored as bit patterns. An `i8` value uses the same eight bits as
`u8`; signed operations interpret bit 7 as the sign bit. Results are masked to
the destination width. For `CmpLt`, `CmpLe`, `CmpGt` and `CmpGe`, `SrcTy` gives
the operand width for two's-complement interpretation; the result is `0` or
`1`. `CmpUlt` compares the unsigned bit patterns.

For `u8` `Shl` and `Shr`, the count is an unsigned byte and is **not** reduced
modulo 64. A count of 8 or more yields zero. For `i8` `Sar`, the left operand
is sign-extended before shifting, so a count of 8 or more yields `0xff` for a
negative operand and zero otherwise. `i8` `SDiv` divides signed values with
truncation toward zero, traps on a zero divisor, then masks the quotient to
eight bits; `-128 / -1` therefore stores `0x80`.

The test oracle in `minzc/pkg/mir2/scalar_oracle_test.go` uses Go byte and
signed-byte conversions, rather than MIR2 VM helpers or constant-folding code.
Each admitted input is checked against the raw VM and the VM after folding.
A deliberately corrupted folded comparison is a negative control.

| Operation | Type | Inputs checked | Observation |
| --- | --- | ---: | --- |
| `CmpLt` | `i8` | all 65,536 byte pairs | boolean return |
| `CmpLe`, `CmpGt`, `CmpGe`, `CmpUlt` | `i8` operand bits | 36 boundary pairs each | boolean return |
| `Shl`, `Shr` | `u8` | all 65,536 byte pairs each | byte return |
| `Sar`, `SDiv` | `i8` | all 65,536 byte pairs each | byte return or divide-by-zero trap |

The gate says nothing yet about wider types, other operations, memory or I/O,
loops, nonconstant propagation, or native code generation. Those remain
unknown in this oracle, even when other tests exercise them. A raw/folded VM
match alone is not counted as independent evidence.

Normalizing constants also exposed a frontend defect in the Z3 smoke test:
Nanz had typed `131072` as `u16`, so the previous folder accidentally moved
its untruncated immediate into a `u32` capacity argument. Nanz now infers
`u24`/`u32` for larger positive literals. The Z3 demo and fixture smoke tests
exercise that integration path; they are not part of the byte-oracle claim.
