# N1a frontend-neutral optimization verification

Baseline: `origin/main` (`33418f905`) in `/mnt/ssd-510/minz/tmp/n1a-before`.
Artifacts: `/mnt/ssd-510/minz/bench/n1a-before` and `/mnt/ssd-510/minz/bench/n1a-after`.

Commands used the main suite only, `--sdcc-root / --jobs 4 --include-refs --include-nanz`, `nice -n 19`, SSD TMPDIR/GOCACHE, GOMAXPROCS=4 and GOFLAGS=-p=4. No heldout benchmark or full judge campaign was run.

## Results

Small typed structs now share the tuple/PFCCO path. Escaping/address-taken objects retain storage; typed address parameters retain memory semantics when mutation/effects cannot be excluded. Captured modules, partial/repeated literals, non-scalar fields and explicit layouts retain the conservative memory path. Initializer evaluation order and field-width conversions are preserved before tuple extraction. C legacy pointer/out-parameter admission remains gated by C origin. Its scoped lvalue-temporary aliases stay in the C compatibility path; neutral scalar storage requires no field address-taking.

Single-bit equality canonicalization runs in `SimplifyIdentities` before mask/byte narrowing and the BIT/shift-carry passes. The existing `PrepareShiftCarry` already hoists the mandatory shift from a pure diamond/triangle join, so no matcher extension was needed.

All comparable MinZ-family cells are non-regressing in both ProgramBytes and TotalT, including nograce. Idiomatic struct_roundtrip equals C and the tuple reference (59 bytes, 567296 T). Idiomatic mul8x8 is 41 bytes / 928256 T, versus C 42 bytes / 948736 T.

The new parity gate exits 1 solely for the pre-existing sum_array gap: Nanz/C 1.0952 in default and grace, unchanged from baseline. The target cases pass (struct 1.0000; multiply 0.9784).

## Per-case C MinZ matrix

Each cell is `bytes before→after; T before→after`.

| Case | Default | Size | Grace |
|---|---:|---:|---:|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 | 95→95; 831,488→831,488 |
| copy_bytes | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 | 172→172; 109,332→109,332 |
| crc8 | 308→308; 69,665,126→69,665,126 | 308→308; 69,665,126→69,665,126 | 308→308; 69,665,126→69,665,126 |
| fat12_decode | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 | 167→167; 30,336→30,336 |
| gcd | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 | 51→51; 3,067,013→3,067,013 |
| memset_fill | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 | 297→297; 3,588,390→3,588,390 |
| mul8x8 | 42→42; 948,736→948,736 | 42→42; 948,736→948,736 | 42→42; 948,736→948,736 |
| reverse_loop11 | 559→559; 11,818,880→11,818,880 | 557→557; 21,032,960→21,032,960 | 559→559; 11,818,880→11,818,880 |
| strlen_bytes | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 | 283→283; 3,574,400→3,574,400 |
| struct_roundtrip | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 | 59→59; 567,296→567,296 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

## All MinZ-family measurements

| Compiler | Profile | Case | Bytes before→after | T before→after |
|---|---|---|---:|---:|
| minz | default | abs_diff | 27→27 | 220,904→220,904 |
| minz | default | bitwise8reg | 95→95 | 831,488→831,488 |
| minz | default | copy_bytes | 172→172 | 109,332→109,332 |
| minz | default | crc8 | 308→308 | 69,665,126→69,665,126 |
| minz | default | fat12_decode | 167→167 | 30,336→30,336 |
| minz | default | gcd | 51→51 | 3,067,013→3,067,013 |
| minz | default | memset_fill | 297→297 | 3,588,390→3,588,390 |
| minz | default | mul8x8 | 42→42 | 948,736→948,736 |
| minz | default | reverse_loop11 | 559→559 | 11,818,880→11,818,880 |
| minz | default | strlen_bytes | 283→283 | 3,574,400→3,574,400 |
| minz | default | struct_roundtrip | 59→59 | 567,296→567,296 |
| minz | default | sum_array | 313→313 | 7,581,952→7,581,952 |
| minz | grace | abs_diff | 27→27 | 220,904→220,904 |
| minz | grace | bitwise8reg | 95→95 | 831,488→831,488 |
| minz | grace | copy_bytes | 172→172 | 109,332→109,332 |
| minz | grace | crc8 | 308→308 | 69,665,126→69,665,126 |
| minz | grace | fat12_decode | 167→167 | 30,336→30,336 |
| minz | grace | gcd | 51→51 | 3,067,013→3,067,013 |
| minz | grace | memset_fill | 297→297 | 3,588,390→3,588,390 |
| minz | grace | mul8x8 | 42→42 | 948,736→948,736 |
| minz | grace | reverse_loop11 | 559→559 | 11,818,880→11,818,880 |
| minz | grace | strlen_bytes | 283→283 | 3,574,400→3,574,400 |
| minz | grace | struct_roundtrip | 59→59 | 567,296→567,296 |
| minz | grace | sum_array | 313→313 | 7,581,952→7,581,952 |
| minz | nograce | abs_diff | 27→27 | 220,904→220,904 |
| minz | nograce | bitwise8reg | 95→95 | 831,488→831,488 |
| minz | nograce | copy_bytes | 172→172 | 109,332→109,332 |
| minz | nograce | crc8 | 308→308 | 69,665,126→69,665,126 |
| minz | nograce | fat12_decode | 167→167 | 30,336→30,336 |
| minz | nograce | gcd | 51→51 | 3,067,013→3,067,013 |
| minz | nograce | memset_fill | 297→297 | 3,588,390→3,588,390 |
| minz | nograce | mul8x8 | 42→42 | 948,736→948,736 |
| minz | nograce | reverse_loop11 | 559→559 | 11,818,880→11,818,880 |
| minz | nograce | strlen_bytes | 283→283 | 3,574,400→3,574,400 |
| minz | nograce | struct_roundtrip | 59→59 | 567,296→567,296 |
| minz | nograce | sum_array | 313→313 | 7,581,952→7,581,952 |
| minz | size | abs_diff | 27→27 | 220,904→220,904 |
| minz | size | bitwise8reg | 95→95 | 831,488→831,488 |
| minz | size | copy_bytes | 172→172 | 109,332→109,332 |
| minz | size | crc8 | 308→308 | 69,665,126→69,665,126 |
| minz | size | fat12_decode | 167→167 | 30,336→30,336 |
| minz | size | gcd | 51→51 | 3,067,013→3,067,013 |
| minz | size | memset_fill | 297→297 | 3,588,390→3,588,390 |
| minz | size | mul8x8 | 42→42 | 948,736→948,736 |
| minz | size | reverse_loop11 | 557→557 | 21,032,960→21,032,960 |
| minz | size | strlen_bytes | 283→283 | 3,574,400→3,574,400 |
| minz | size | struct_roundtrip | 59→59 | 567,296→567,296 |
| minz | size | sum_array | 313→313 | 7,581,952→7,581,952 |
| nanz | default | abs_diff | 27→27 | 220,904→220,904 |
| nanz | default | copy_bytes | 172→172 | 109,332→109,332 |
| nanz | default | fat12_decode | 167→167 | 30,336→30,336 |
| nanz | default | gcd | 51→51 | 3,067,013→3,067,013 |
| nanz | default | struct_roundtrip | 154→59 | 1,845,248→567,296 |
| nanz | default | sum_array | 314→314 | 8,303,584→8,303,584 |
| nanz | grace | abs_diff | 27→27 | 220,904→220,904 |
| nanz | grace | copy_bytes | 172→172 | 109,332→109,332 |
| nanz | grace | fat12_decode | 167→167 | 30,336→30,336 |
| nanz | grace | gcd | 51→51 | 3,067,013→3,067,013 |
| nanz | grace | struct_roundtrip | 154→59 | 1,845,248→567,296 |
| nanz | grace | sum_array | 314→314 | 8,303,584→8,303,584 |
| nanz | nograce | abs_diff | 27→27 | 220,904→220,904 |
| nanz | nograce | copy_bytes | 172→172 | 109,332→109,332 |
| nanz | nograce | fat12_decode | 167→167 | 30,336→30,336 |
| nanz | nograce | gcd | 51→51 | 3,067,013→3,067,013 |
| nanz | nograce | struct_roundtrip | 154→59 | 1,845,248→567,296 |
| nanz | nograce | sum_array | 314→314 | 8,303,584→8,303,584 |
| nanz | size | abs_diff | 27→27 | 220,904→220,904 |
| nanz | size | copy_bytes | 172→172 | 109,332→109,332 |
| nanz | size | fat12_decode | 167→167 | 30,336→30,336 |
| nanz | size | gcd | 51→51 | 3,067,013→3,067,013 |
| nanz | size | struct_roundtrip | 154→59 | 1,845,248→567,296 |
| nanz | size | sum_array | 314→314 | 8,303,584→8,303,584 |
| nanz-idiom | default | abs_diff | 27→27 | 220,904→220,904 |
| nanz-idiom | default | bitwise8reg | 97→97 | 847,872→847,872 |
| nanz-idiom | default | copy_bytes | 171→171 | 108,630→108,630 |
| nanz-idiom | default | crc8 | 313→313 | 72,387,814→72,387,814 |
| nanz-idiom | default | fat12_decode | 167→167 | 30,336→30,336 |
| nanz-idiom | default | gcd | 51→51 | 3,067,013→3,067,013 |
| nanz-idiom | default | memset_fill | 303→303 | 3,638,295→3,638,295 |
| nanz-idiom | default | mul8x8 | 53→41 | 1,358,336→928,256 |
| nanz-idiom | default | reverse_loop11 | 567→567 | 11,662,060→11,662,060 |
| nanz-idiom | default | strlen_bytes | 287→287 | 3,611,520→3,611,520 |
| nanz-idiom | default | struct_roundtrip | 154→59 | 1,845,248→567,296 |
| nanz-idiom | default | sum_array | 314→314 | 8,303,584→8,303,584 |
| nanz-idiom | grace | abs_diff | 27→27 | 220,904→220,904 |
| nanz-idiom | grace | bitwise8reg | 97→97 | 847,872→847,872 |
| nanz-idiom | grace | copy_bytes | 171→171 | 108,630→108,630 |
| nanz-idiom | grace | crc8 | 313→313 | 72,387,814→72,387,814 |
| nanz-idiom | grace | fat12_decode | 167→167 | 30,336→30,336 |
| nanz-idiom | grace | gcd | 51→51 | 3,067,013→3,067,013 |
| nanz-idiom | grace | memset_fill | 303→303 | 3,638,295→3,638,295 |
| nanz-idiom | grace | mul8x8 | 53→41 | 1,358,336→928,256 |
| nanz-idiom | grace | reverse_loop11 | 567→567 | 11,662,060→11,662,060 |
| nanz-idiom | grace | strlen_bytes | 287→287 | 3,611,520→3,611,520 |
| nanz-idiom | grace | struct_roundtrip | 154→59 | 1,845,248→567,296 |
| nanz-idiom | grace | sum_array | 314→314 | 8,303,584→8,303,584 |
| nanz-tuple | default | struct_roundtrip | 59→59 | 567,296→567,296 |
| nanz-tuple | grace | struct_roundtrip | 59→59 | 567,296→567,296 |
| nanz-tuple | nograce | struct_roundtrip | 59→59 | 567,296→567,296 |
| nanz-tuple | size | struct_roundtrip | 59→59 | 567,296→567,296 |

## Test evidence

- `TestSmallStructNeutralVMZ80`: fails on origin/main for Nanz (`struct return not scalarized`); C passes. Both frontends pass with scalar tuples/storage elimination, address escape retention, wraparound, and native/MIR execution.
- `TestSingleBitSpellingsNeutralVMZ80`: fails on origin/main for `(x & 1) == 1` in C and Nanz, with Grace on/off (`bit predicate did not fuse with join shift`). With the change all equivalent nonzero spellings emit identical executable assembly, and inverted zero tests preserve polarity.
- Origin evidence: `/mnt/ssd-510/minz/tmp/n1a-origin-red.log`. Initial changed runs: `n1a-struct-final.log`, `n1a-targeted.log`.
- Focused MIR tests cover every bit in u8/i8/u16/i16, signed top-bit constants, shared AND consumers, idempotence, and structural refusal guards.
- First full unit run exposed a fixture that used C origin to force memory ABI; the fixture now uses explicit natural layout.

Final compiler checks:

- Required full unit command: `go test ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... ./pkg/z80asm/... -count=1`, all six package results PASS (`/mnt/ssd-510/minz/tmp/n1a-unit-complete.log`). Final HIR-only admission tightening is followed by another affected-package run.
- Related pipeline run: 16 tests covering aggregate address classes/lifetimes, bool storage, pointer methods, global aggregate literals, diamond shifts and O8 safety/profitability kernels, PASS in 182.296 s (`n1a-pipeline-final.log`). Only targeted pipeline tests were run.
- Final C corpus fingerprint: 66 files; 61 byte-identical assemblies and five identical frontend errors (c23_bitint, c23_constexpr_enum, c23_misc, fatfs/diskio and import_test). No successful example was lost (`n1a-c-corpus-before.json`, `n1a-c-corpus-final.json`).
- Production reversal proof: `git diff --binary --no-renames origin/main` saved to `n1a-production.patch`, applied with `git apply -R`, tests returned exit 1, then restored with `git apply`. The bit test failed for C and Nanz in both Grace modes; the struct test failed for Nanz's unscalarized return (`n1a-reversed-red.log`).
- Struct tests also cover source-ordered effectful initializers, returns and calls inside switch cases, whole-object and field-address escapes, and repeated pointer names in separate lexical blocks. The alias safety subtest passes with storage semantics preserved; it is complementary negative coverage, not an origin/main optimization failure.
- Parity Python tests: 3/3 PASS. Both new top-level Go tests are in the sorted fast inventory.

Final affected-package, focused-test, judge and refreshed benchmark outcomes follow below.

## Final outcomes

- Final HIR/C89/Nanz affected-package rerun: PASS (46.634 s / 93.110 s / 126.113 s), `n1a-hir-final.log`. MIR2 and Z80 assembler code did not change after their full required-package PASS.
- Final focused pipeline run: PASS, 140.909 s, `n1a-new-tests-complete.log`; both C and Nanz versions plus the alias safety submatrix.
- Final rebuilt compiler: direct-call seeds 0–499, 500/500 PASS; folded seeds 0–99, 100/100 PASS; `-j 4 --no-reduce`, zero oracle/MIR/native disagreements or compile/assembly failures. No larger seeds, signed campaigns, assert_matrix or heldout benchmarks were run.
- Final refreshed benchmark: 136 total rows, 100 MinZ-family rows; zero regressions in bytes or T, including nograce. The recorded tables exactly match the refreshed measurements.
- Parity gate: exit 1, only unchanged sum_array (default/grace 1.0952×). struct_roundtrip 1.0000× C and tuple; mul8x8 0.9784× C.
- Exact CI inventory: 882 entries, no missing/stale entries, fast list sorted. Parity unit tests: 3/3 PASS.
- Benchmark compiler source commit: `e86e4bd8c6a0b974bf11a37a3877353ee4383981`; baseline `33418f905`.

## Files touched

The C-prefixed aggregate implementation files were renamed to the neutral
`aggregate_promote.go` and `aggregate_scalar.go`; compatibility adapters remain
explicitly gated. The complete resulting file inventory is:

- `docs/adr/0025-struct-return-promotion.md`
- `minzc/pkg/c89/aggregate_transfer_cost_test.go`
- `minzc/pkg/hir/aggregate_promote.go`
- `minzc/pkg/hir/aggregate_scalar.go`
- `minzc/pkg/mir2/constprop.go`
- `minzc/pkg/mir2/single_bit.go`
- `minzc/pkg/mir2/single_bit_test.go`
- `minzc/pkg/pipeline/aggregate_address_class_test.go`
- `minzc/pkg/pipeline/pipeline.go`
- `minzc/pkg/pipeline/single_bit_neutral_test.go`
- `minzc/pkg/pipeline/small_struct_alias_test.go`
- `minzc/pkg/pipeline/small_struct_neutral_test.go`
- `minzc/pkg/pipeline/variant_search.go`
- `reports/2026-10-22-n1a-frontend-neutral.md`
- `research/c89-benchmark/README.md`
- `research/c89-benchmark/nanz-idiomatic/manifest.json`
- `research/c89-benchmark/nanz-idiomatic/struct_roundtrip_tuple.nanz`
- `research/c89-benchmark/parity.py`
- `research/c89-benchmark/run.py`
- `research/c89-benchmark/test_parity.py`
- `scripts/ci/pipeline_profiles.json`

## Local commits

- `d94e154db`: neutral HIR small-struct scalarization and C/Nanz regressions.
- `f719cc0b1`: MIR2 single-bit canonicalization and C/Nanz regressions.
- `e86e4bd8c`: pinned explicit tuple reference and runner integration.
- The final commit adds the parity gate/tests, README documentation and this verification record.

No commits were pushed.
