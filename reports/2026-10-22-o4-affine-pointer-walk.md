# O4+: affine array pointer induction (2026-10-22)

Base: `origin/main` at `1815f5a35`. Worktree branch: `opt/o4-pointer-sr`.

## Change and legality

`StrengthReduceCountedPointers` retains the original byte-copy recognizer and
adds a sibling over the existing natural-loop analysis. It recognizes a single
body with a strict increasing unsigned/signed guard, a signed `i >= 0`
countdown, an unsigned `i > 0` countdown, or a guarded byte DJNZ self-loop.
Addresses may use a normal global `addr_of` or an invariant parameter base,
constant affine offsets, and byte/word element strides. There are no source
name or benchmark name checks.

The pass proves executed IV ranges before matching the address DAG. Increasing
strict guards exclude the type's maximum value, so the next IV cannot wrap;
signed countdowns execute only nonnegative IVs, unsigned countdowns only positive
IVs. DJNZ needs a nonzero entry guard (or a known positive initial count).
An unguarded zero DJNZ entry can wrap and is refused. Byte operations must stay
in 0..255 before widening; `ext8(i-1)` in a zero-start byte loop is deliberately
refused. Word arithmetic and pointer updates retain the exact arithmetic modulo
65536. Initial addresses clone the original pure DAG, including its casts.
Zero-trip paths do no additional loads or stores. Calls, assembly, volatile
operations and physical constraints exclude the loop. Memory operations retain
their order, so overlapping accesses retain their sequential semantics.

Conditional preheaders require a final comparison, which is kept after the
cloned address arithmetic. This preserves its definition before existing uses
and avoids extending physical flags across address arithmetic. The original IV
remains for loop control or value uses; dead address arithmetic is removed.
Descending pointers use ordinary word subtraction because `ptr_bump(-1)` is
reserved for the existing SoA encoding.

After addressing has its own pointer, a word IV gets the DE preference while
the pointer prefers HL. A small post-allocation affinity for word +/-1 and
+/-2 results uses the existing interference and class checks. This fixes the
DE phi decrement: `DEC DE`, without the HL round trip. A word-store scratch
save is omitted only for the exact straight-line four-instruction
`LD (HL),r; INC HL; LD (HL),r; DEC HL` shape, with nonaliasing byte operands.
That sequence already restores HL; removing its redundant PUSH/POP changes
neither memory access order nor lasting register values.

The implementation remains conservative: multi-body loops, unknown sentinels,
wrapping byte expressions and unsupported CFG/address shapes are left alone.

## Files

- `minzc/pkg/mir2/affine_pointer_walk.go`: new recognizer and range proof.
- `minzc/pkg/mir2/counted_pointer_walk.go`: shared entry point.
- `minzc/pkg/mir2/coalesce.go`: word small-step result affinities.
- `minzc/pkg/mir2/z80codegen_muldiv.go`: recognize restored HL word stores.
- `minzc/pkg/mir2/affine_pointer_walk_test.go`: modular-address VM oracles,
  refusal tests, DJNZ guard tests, unsigned countdowns, phi affinity controls.
- `minzc/pkg/mir2/counted_pointer_walk_test.go`: move the formerly refused
  nonzero-start/changing-bound shapes to positive semantic tests.
- `minzc/pkg/pipeline/affine_pointer_walk_native_test.go`: C/Nanz MIR shape,
  raw and optimized VM, and Z80 emulator checks across all three profiles.
- `scripts/ci/pipeline_profiles.json`: add the new pipeline test to sorted fast.
- This report.

## Verification

All heavy processes ran at nice 19, with TMPDIR/GOCACHE on the SSD, at most four
jobs per phase, and an uptime/load check before each phase. No heldout suite,
assert_matrix, larger fuzz campaign, or push was run.

The focused pipeline test is `TestAffinePointerWalkNativeCAndNanz`. Its 84
subcases cover C and Nanz, global and parameter bases, byte and word arrays,
forward `a[i-1]`, signed/unsigned countdowns, and Nanz DJNZ range loops.
Each runs counts 0/1/2/127/255/256 (256 truncates to zero for a byte range), with
canaries and an independent memory oracle, against raw MIR VM, optimized MIR
VM and native Z80. It also checks that hot-loop ptr_add/multiply instructions
are gone. MIR tests separately exercise address wrap through 0xffff and
preserve an IV that remains live after a decrement.

Full required packages, final tree:

```
go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1
ok pkg/mir2                 125.611s
ok pkg/hir                   55.691s
ok pkg/c89                   88.244s
ok pkg/nanz                 128.173s
ok pkg/z80asm                 0.144s
ok pkg/z80asm/regression      0.012s
```

Selected pipeline checks passed: the new test, counted-pointer overlap,
invariant-address native wrap and reassociation, sequential loops retaining
word operands, struct-method pointer scratch, and word compare branches.
The latter three completed in the related focused run (62.193s); the final
pointer-focused rerun completed in 2.017s. The whole pipeline package was not
run.

Logs and raw benchmark/judge artifacts are under
`/mnt/ssd-510/minz/tmp/o4-*.log`, `/mnt/ssd-510/minz/bench/o4-before`,
`/mnt/ssd-510/minz/bench/o4-after`, and `/mnt/ssd-510/minz/bench/o4-judges`.

## Failing-first evidence and judges

Final test source, with the MIR2 implementation patch reversed against
`origin/main`: **exit 1, all 84 subcases fail the hot-loop shape assertion**.
Restoring the exact patch: **exit 0** (`pkg/mir2` 0.023s,
`pkg/pipeline` 0.430s). The working compiler files matched the staged patch
exactly after restoration. Logs: `o4-failing-first-final.log` and
`o4-passing-final.log` in the artifact directory above.

Implementation patch SHA-256:
`006b85cdb83e529b7375a5e5a18a77086a8082fb0cdf201b913a8b600fb40305`.
The after compiler was built from this uncommitted implementation; the harness
records HEAD `1815f5a35` in both metadata files. The patch hash identifies the
after implementation independently of that HEAD field.

Bounded judges, candidate CLI binary, all with `-j 4 --no-reduce`:

| Mode | Seeds | Result |
|---|---|---|
| direct-call | 0–499 | 500/500 pass |
| folded | 0–99 | 100/100 pass |
| direct-call --masked-loops | 0–299 | 300/300 pass |

Each reports zero MIR/oracle mismatches, zero Z80/MIR mismatches, and zero
assembly/compiler/oracle errors.

## Before/after benchmark

Both matrix runs used `--suite main --jobs 4 --sdcc-root / --include-refs`.
The before run used a separate detached origin/main worktree under the SSD
temporary directory. All 36 MinZ C cells and 24 idiomatic Nanz reference cells
pass. No bytes or T cell regresses. SDCC cannot compile `struct_roundtrip` in
either profile on either tree; these same two baseline failures are unchanged.
Reference size mode for Nanz reverse was measured separately using each
matrix runner binary and the same frozen source/workload.

Each cell is **program bytes before→after / total T before→after**.
Program bytes include emitted global data. Every case and all three MinZ
profiles are shown; unchanged cells are repeated explicitly.

| Case | Default B / T | Size B / T | Grace B / T |
|---|---|---|---|
| abs_diff | 27→27 / 220904→220904 | 27→27 / 220904→220904 | 27→27 / 220904→220904 |
| bitwise8reg | 95→95 / 831488→831488 | 95→95 / 831488→831488 | 95→95 / 831488→831488 |
| copy_bytes | 172→172 / 109332→109332 | 172→172 / 109332→109332 | 172→172 / 109332→109332 |
| crc8 | 315→315 / 94778402→94778402 | 315→315 / 94778402→94778402 | 315→315 / 94778402→94778402 |
| fat12_decode | 167→167 / 30336→30336 | 167→167 / 30336→30336 | 167→167 / 30336→30336 |
| gcd | 57→57 / 4953966→4953966 | 57→57 / 4953966→4953966 | 51→51 / 3067013→3067013 |
| memset_fill | 297→297 / 3588390→3588390 | 297→297 / 3588390→3588390 | 297→297 / 3588390→3588390 |
| mul8x8 | 45→45 / 1096192→1096192 | 45→45 / 1096192→1096192 | 45→45 / 1096192→1096192 |
| reverse_loop11 | 561→559 / 23644160→11818880 | 561→559 / 23644160→11818880 | 561→559 / 23644160→11818880 |
| strlen_bytes | 283→283 / 3574400→3574400 | 283→283 / 3574400→3574400 | 283→283 / 3574400→3574400 |
| struct_roundtrip | 59→59 / 567296→567296 | 59→59 / 567296→567296 | 59→59 / 567296→567296 |
| sum_array | 313→313 / 7581952→7581952 | 313→313 / 7581952→7581952 | 313→313 / 7581952→7581952 |

Nanz `reverse_loop11`:

| Profile | Bytes before→after | T before→after |
|---|---|---|
| default | 578→567 | 37508460→11662060 |
| size | 578→567 | 37508460→11662060 |
| grace | 578→567 | 37508460→11662060 |

C reverse meets the ≤12M T target: **23,644,160→11,818,880 T**, with
**561→559 bytes** (46→44 code bytes after excluding 515 global bytes).
Nanz reverse is **37,508,460→11,662,060 T**, **578→567 bytes**.
The measured Nanz baseline on this origin/main is 37.51M T rather than the
earlier report’s 39.1M T. All other main/reference cells are unchanged.

Result: GO. No push.
