# Main-suite reference implementations

The twelve main cases have separate frozen Nanz and assembly manifests. The C
sources, workload generation, expected results and memory regions are unchanged.
No compiler code was modified. No heldout results were read or generated.

Nanz retains the algorithms and byte wrapping of the C cases: subtractive GCD,
forward copy, CRC-8 polynomial 7, shift/add multiplication, struct construction
and consumption, and reverse word-array fill. Sum and copy use pointer-slice
foreach; CRC uses a pointer foreach with a range for its eight bit steps; reverse
fill uses the language's countdown range and subtracts one from the counter.
The other cases use byte types, typed pointers and ordinary functions as natural.
There is no inline assembly, SMC, table or workload-specific specialization.
The small abs/GCD/FAT/struct/sum sources retain the already idiomatic forms from
`nanz-cases`; they are separately pinned here as part of the twelve-case set.

Hand modules include all globals and an `execute` entry at origin 0x8000, without
startup code. Scratch state lives in registers. Copy uses a zero-guarded LDIR;
strlen uses CPIR over the 256-byte terminating buffer; other loops use DJNZ or
register flags. CRC uses shift carry to choose polynomial XOR, sum increments its
high byte on carry, and multiplication shifts its multiplier and multiplicand.
The assembler reserves global storage with DS/DW, included once in ProgramBytes.
Only `result` and the contract's destination buffers are written.

## Findings

1. **Nanz low-bit condition miscompilation (both default and grace).** In
   `mul8x8.nanz`, the initial natural spelling was:

   ```nanz
   if (multiplier & 1) != 0 { product = product + multiplicand }
   ```

   The raw and final MIR VMs pass all 2,048 inputs, but native Z80 fails 765.
   The first failure is `x=1,y=2`: `native result3 want2` (315 T-states), with
   correct return PC/SP. Generated code contains `LD A,C; CP B` at the test,
   without the required bit mask or initialization of B. Reproduce by replacing
   `== 1` with `!= 0` in the committed source. The equivalent byte condition
   `(multiplier & 1) == 1` passes every workload input on both profiles and is
   the retained implementation. Original failing source SHA256:
   `07ee60406b7ac86a20bead9a3972d4b6ee6eb6da5ebaa22203c031022fc224e0`.
   Full source, assembly, MIR and observations for the rejected form are preserved
   outside the repository under `/mnt/ssd-510/minz/bench/r1-probe/mul8x8-ne-default`
   and `mul8x8-ne-grace`.

2. **mza register-name label ambiguity.** `LD A,[a]` and `LD A,[b]`, with
   globals named `a:` and `b:`, produce `unknown instruction or invalid operands:
   LD`. The modules use `v_a` and `v_b`, as generated MinZ code does. The existing
   harness symbol resolver already accepts this prefix, so inputs and outputs
   keep their original contract names. No assembler change was made.

3. **Range elements count down.** As documented in the Nanz language book and
   `lowerRangeLoop`, `range(0..n)` yields `n..1`, not `0..n-1`. Reverse fill uses
   `counter - 1`; the CRC inner loop ignores the element and performs eight
   steps. All zero-count paths pass. This is a language semantic, not a newly
   discovered compiler failure.

4. **Idiomatic spelling does not guarantee faster output.** Pointer foreach
   sum costs 8,308,684 T versus C's 7,581,952 T, and foreach copy costs 616,990 T
   versus C's 301,600 T. The Nanz struct-return implementation costs 154 bytes /
   1,845,248 T versus C's 71 / 612,352. Conversely, the Nanz CRC costs 99,145,840 T
   versus C's 176,060,962 T, and Nanz fill costs 8,855,680 T versus C's 18,403,840 T.
   These are whole-workload observations for these frozen sources, not tuned
   profile choices. Grace changes the Nanz GCD result; the other eleven Nanz
   default/grace B/T pairs are identical.

5. **Hand Z80 provides smaller, faster references for all twelve cases.** LDIR,
   CPIR, register loop state and carry-based arithmetic leave substantial gaps
   in the generated code for the array loops. These complete modules include
   global storage in their byte counts and use the unchanged measurement path.

6. **Installed SDCC rejects struct return/pass by value.** Both SDCC profiles
   report error 54 (`Function cannot return aggregate`) and error 97 (`cannot
   pass structure ... as function argument`) for the unchanged C case. Its rows
   remain COMPILE_ERROR and its comparison cell is unsupported; there is no
   substitute C source or adjusted expectation.

## Verification and comparison

Run on the shared host with no more than four jobs:

```sh
TMPDIR=/mnt/ssd-510/minz/tmp GOCACHE=/mnt/ssd-510/minz/gocache GOMAXPROCS=4 \
  nice -n 19 python3 research/c89-benchmark/run.py \
  --out /mnt/ssd-510/minz/bench/r1 --sdcc-root / --suite main \
  --include-nanz --include-refs --jobs 4
```

All 24 Nanz idiom and 12 hand assembly rows PASS (48,198 reference calls). All original MinZ/Nanz rows
PASS; only the two existing SDCC struct rows are unsupported. Nanz checks raw
and final MIR VM results/memory as well as native Z80; hand uses the identical
native result, complete declared after-memory, return PC and SP checks used for
external C images. Every passing row for a case has the same workload SHA256.
The driver records reference source hashes and the built mza hash; source, image
and symbol hashes are checked again before external execution. Negative replays
confirmed that a tampered hand image is rejected by the hash check and an
attested image returning an incorrect result is rejected by native verification.
A correct-result image that corrupts an input global is also rejected by the
native memory check.

B/T means occupied program/global bytes / total T-states over all calls. Best
SDCC is the passing profile with the lowest total T (ties choose fewer bytes).
No heldout measurements inform this table.

| case | MinZ C default B/T | nanz-idiom default B/T | nanz-idiom grace B/T | hand B/T | best SDCC B/T |
|---|---:|---:|---:|---:|---:|
| abs_diff | 27/220904 | 27/220904 | 27/220904 | 23/172474 | 34/264286 |
| copy_bytes | 182/301600 | 194/616990 | 194/616990 | 158/100340 | 215/822770 |
| fat12_decode | 167/30336 | 167/30336 | 167/30336 | 107/12864 | 159/27392 |
| gcd | 57/4953966 | 57/4953966 | 51/3067013 | 43/3031996 | 57/4249984 |
| struct_roundtrip | 71/612352 | 154/1845248 | 154/1845248 | 23/192512 | unsupported |
| sum_array | 313/7581952 | 317/8308684 | 317/8308684 | 286/3008370 | 326/9798656 |
| bitwise8reg | 101/872448 | 115/978944 | 115/978944 | 42/323584 | 82/727040 |
| reverse_loop11 | 587/39732480 | 581/39140460 | 581/39140460 | 546/7643955 | 554/14683520 |
| crc8 | 342/176060962 | 327/99145840 | 327/99145840 | 292/44057064 | 346/152886724 |
| mul8x8 | 51/1331712 | 55/1440256 | 55/1440256 | 35/744448 | 42/1012992 |
| strlen_bytes | 293/19468800 | 297/17536000 | 297/17536000 | 277/3548800 | 278/9083520 |
| memset_fill | 299/18403840 | 303/8855680 | 303/8855680 | 287/4366015 | 295/14810880 |
