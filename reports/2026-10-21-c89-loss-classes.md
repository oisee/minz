# MinZ C vs SDCC 4.2: loss classes on the 12-case main suite (2026-10-21)

**Input:** the B1 matrix (PR #224; MinZ `fa25ad366` compiler, SDCC 4.2.0 size/speed), main suite
only. The heldout suite was not opened for this analysis.
**Method:** per-iteration cost taken from the slope of T-states over `n` in each case's
`measurements.json`; the loss of each case (MinZ default minus best SDCC profile) is split across
classes in proportion to hand-counted Z80 T-states of the hot loop. `final.mir` was used to tell
IR-level causes from codegen causes. Numbers are approximate attributions, not separate runs.

## Losing cases (MinZ default vs best SDCC)

| Case | T-states lost | Bytes lost | Note |
|---|---:|---:|---|
| crc8 | 214.5M | +53 | Grace: 206M |
| reverse_loop11 | 33.0M | +39 | |
| strlen_bytes | 16.1M | +17 | |
| memset_fill | 11.1M | +10 | |
| mul8x8 | 1.82M | +34 | |
| bitwise8reg | 1.08M | +117 | SDCC upstream kernel; Grace 0.95M / +101 |
| gcd | 0.86M | +19 | Grace **wins** by 1.03M T, still +11 B |
| fat12_decode | 4.6K | +18 | |
| **Total** | **~278M** | | |

MinZ wins copy_bytes (−32 B, 2.7× faster), sum_array (−12 B, 1.3× faster) and abs_diff
(−7 B, −16 % T); SDCC cannot compile struct_roundtrip.

## Classes, sorted by loss

| # | Class | Cases | ≈T lost | ≈B lost | Layer | Size |
|---|---|---|---:|---:|---|---|
| 1 | u8 arithmetic left in i16 after C promotion (`and/or/xor/shl/sar` of `ext`, `ext` with a constant) | crc8 108M, bitwise8reg 0.9M, mul8x8 0.26M | ~109M | ~135 | MIR pass | S–M |
| 2 | Compare lowering: `ext u8` vs constant compared as 16-bit SBC; signed i16 compare turned into a carry value through `JP M/PE/PO`+`SCF/CCF` | crc8 62M, reverse 12.4M, strlen 6.6M, mul8x8 1.2M, gcd 0.1M | ~82M | ~75 | MIR narrowing + codegen | S |
| 3 | 16-bit moves via PUSH/POP; `ptr_add` hard-wired to DE; zero-extension via PUSH AF; ±1 through HL copies | reverse 17.7M, crc8 9.5M, strlen 9.5M, memset 6.4M, fat12 | ~43M | ~30 | codegen | S |
| 4 | Loop-carried value kept in A, so every ALU/CMP/byte load saves and restores it | crc8 29M, memset 2.6M, mul8x8 0.3M | ~32M | ~25 | regalloc (PBQP costs) | M–L |
| 5 | Dead constant materialisation (`LD HL,1` overwritten; `LD HL,-1; ADD HL,DE` for `DEC DE`) | strlen 1.6M, reverse, bitwise8reg | ~2M | ~10 | peephole | S |
| 6 | Duplicate compare instead of reusing flags (default only; Grace already fixes it) | gcd 0.77M | 0.8M | 6 | MIR / Grace | S |
| 7 | u16 parameter assigned to IY plus PUSH/POP call glue | fat12 | 5K | ~10 | PFCCO | M |

About 8M T stays unattributed (reloading `n` each iteration, JR vs JP).

**Related opportunity (not a loss class by itself):** `StrengthReduceCountedPointers`
(`minzc/pkg/mir2/counted_pointer_walk.go`) only accepts a parameter base and bound, so loops over
*global* arrays with a global bound (memset, strlen, crc8, reverse_loop11) never get pointer
induction, which is what makes copy_bytes and sum_array beat SDCC.

## Where each fix likely lives

1. `mir2/narrow_byte_arithmetic.go` (`narrowTruncatedByteArithmetic`, called from
   `constprop.go`): today only add/sub with both operands `ext`. Generalise to demanded-low-byte
   narrowing: `trunc8(op16(x,y)) → op8(trunc8 x, trunc8 y)` for add/sub/and/or/xor/shl, byte
   constants as operands, through chains; `sar(ext_u8 x,k) → shr8`.
2. `mir2/narrow_byte_compare.go` (`narrowZeroExtendedByteComparisons`): accept a constant in
   [0,255] as the other side, fold out-of-range constants. Codegen `z80codegen_cmp.go`
   (`normalizeSignedCmp`, `genCmp16`): branch directly on sign for `x>=0`/`x<0`, `LD A,H; OR L`
   for ==/!= 0.
3. `z80codegen_move.go` (`emitMov` PUSH/POP fallback, `genUnsignedExtBytes`),
   `z80codegen_inst.go` (`OpPtrAdd` offset pair), ±1 → `INC rr`/`DEC rr`.
4. PBQP costs (`pbqp_affinity.go`, `pbqp_memory_cost.go`): penalise A for loop-carried values live
   across ALU/CMP sites.

## Order of work

Smallest risk × largest loss first, one class per PR, each under the full gate (failing-first test,
main-suite matrix before/after with no new loss, heldout not worse, direct-call fuzz 20000/20000,
folded 1000/1000, assert_matrix 0 unexpected, critic GO):

1. Class 1, byte narrowing of bitwise/shift ops.
2. Class 2, MIR part (constant-side compare narrowing), then the codegen part.
3. Class 3, pair moves without PUSH/POP and `INC/DEC rr`.
4. Global-array counted pointers.
5. Class 4 (regalloc), if the earlier steps leave it as the top loss.
