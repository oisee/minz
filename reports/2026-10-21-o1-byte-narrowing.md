# O1: demanded low-byte arithmetic (2026-10-21)

`narrowTruncatedByteArithmetic` now clones byte versions of promoted add, sub,
and, or, xor and constant left-shift trees at low-byte consumers. Its leaves
are zero-extended unsigned bytes or constants reduced modulo 256. The proof
is limited to preceding, uniquely defined values in one block and depth 12.
The old word definitions remain available for all other uses; byte copies are
shared within the block. Truncation destinations used in the cache must also
be uniquely defined.

Right shifts only accept a direct zero-extended u8 input and a constant count
in [0,7]; arithmetic word shifts become logical byte shifts. A general word
comparison never demands only the low byte. Explicit byte comparisons only
strip zero extensions. Byte stores may consume cloned arithmetic. Hard
classes, raw assembly, carry-returning functions and ambiguous definitions
retain the original conservative guards. Standalone conversion folding stays
outside this arithmetic pass.

Validation artifacts are in `/mnt/ssd-510/minz/bench/O1-logs`.

- Failing first: `TestDemandedLowByteArithmetic`, all eight operation subtests
  fail with the implementation diff reversed, then pass with it restored.
  The pre-change implementation is byte-identical to origin/main (SHA256
  `302985143b06259117194e18681c957da959fc44b84d465fd61b3c37c0e8997a`).
  Evidence: `failing-first.log` and the focused/required suite runs.
- Unit coverage: exhaustive byte pairs for add/sub/and/or/xor; nested constant
  trees and preserved word users; invalid shifts; high-byte-dependent right
  shifts; shared stores/truncations; word comparisons; recursion and non-SSA
  refusal cases.
- `TestDemandedBytePromotionNative`: C expression shape checks plus MIR VM and
  Z80 emulator checks for shift/xor, add/and and unsigned-byte right shift,
  across default, size and Grace. Registered in the sorted fast CI inventory.
- Required mir2/hir/c89/nanz/z80asm suite: PASS (`unit-final.log` and `unit-final-recheck.log`).
- Direct-call differential judge, seeds 0–1999, four jobs: 2000/2000 PASS.
- Folded differential judge, seeds 0–299, four jobs: 300/300 PASS.
- Baseline/candidate assertion matrix: exit 0 on both backends; each checks
  1731 assertion units and 1739 negative controls, with zero new failures,
  zero unexpected control passes and zero new listing errors. Existing
  failures remain: 135 Z80 units and 99 MIR2 units fail on both compilers.
- Requested monolithic full pipeline command: cumulative 30-minute timeout,
  with no preceding test failures. The active test was
  `TestMemoryHostArenaSelectorOriginal30Objectives/speed`, only five seconds
  into execution when the package timer fired. That complete test passes
  on unchanged origin/main in 36.084 seconds.
- Full-mode continuation completed all 549 remaining entries. Two process
  groups also reached cumulative 30-minute limits (their active tests had run
  43s and 18s); their 64 unfinished tests were resumed in four smaller groups,
  all exit 0. No `-short` mode was used. Together with the original 276-test
  prefix, every one of the 825 inventory entries is accounted for, with no
  missing or unexpected entries. The continuation contains 479 PASS, 69 SKIP
  (normal environment-dependent skips), and the single baseline FAIL below.
  Evidence: `pipeline-coverage.json`, `pipeline-remainder/inventory.json`,
  `pipeline-remainder/statuses.json`, `pipeline-tail/statuses.json`.
- Pre-existing pipeline failure: `TestSeededPointerWalkMeasuredCaller` asserts
  that countdown must win, but both origin/main and O1 measure the same four
  candidates: original masks 0/2 = 120327 T / 57 B; countdown masks 1/3 =
  144573 T / 150 B. Evidence: `baseline-seeded.log` and
  `pipeline-remainder/shard-5.log`. The existing test is unchanged.

The first development version also folded standalone trunc(ext) conversions;
it failed `TestC89ControlMatrixVMZ80` on nested_switch(4), while origin/main
passed. Restricting roots to arithmetic preserves the existing mask lowering;
the focused control-matrix test and subsequent full package run pass.

All heavy commands use nice 19, TMPDIR `/mnt/ssd-510/minz/tmp`, GOCACHE
`/mnt/ssd-510/minz/gocache`, and Go parallelism capped at four jobs. No heldout
benchmark suite was run.

## Main-suite benchmark

Before: origin/main `c3d779b0d`, isolated checkout
`/mnt/ssd-510/minz/tmp/o1-before`. Origin/main predates the 12-case manifest
and `--suite`, so only the task branch's frozen **main** case inputs, manifest,
`run.py` and `measure.go` were copied into that checkout. Compiler sources
remain unchanged. Before and after therefore use identical sources and
workload hashes. Both runs use `--suite main --jobs 4 --sdcc-root /`.

Artifacts: `/mnt/ssd-510/minz/bench/O1-before` and
`/mnt/ssd-510/minz/bench/O1-after`. Bytes are initialized image bytes; T is the
full workload TotalT. Every one of the 36 MinZ measurements passes, with no
increase in bytes or T. Default and size are identical for all cases.

| Case | Default = size: bytes; T | Grace: bytes; T |
|---|---|---|
| abs_diff | 27→27; 220,904→220,904 | 27→27; 220,904→220,904 |
| bitwise8reg | 199→101; 1,808,384→899,072 | 183→101; 1,677,312→899,072 |
| copy_bytes | 182→182; 301,600→301,600 | 182→182; 301,600→301,600 |
| crc8 | 399→364; 367,390,393→296,012,962 | 391→362; 359,221,433→296,004,002 |
| fat12_decode | 167→167; 32,000→32,000 | 167→167; 32,000→32,000 |
| gcd | 74→74; 5,109,358→5,109,358 | 68→68; 3,222,405→3,222,405 |
| memset_fill | 304→304; 25,911,040→25,911,040 | 304→304; 25,911,040→25,911,040 |
| mul8x8 | 75→71; 2,836,992→2,683,392 | 75→71; 2,836,992→2,683,392 |
| reverse_loop11 | 592→592; 47,729,280→47,729,280 | 592→592; 47,729,280→47,729,280 |
| strlen_bytes | 294→294; 25,150,080→25,150,080 | 294→294; 25,150,080→25,150,080 |
| struct_roundtrip | 85→71; 935,936→612,352 | 85→71; 935,936→612,352 |
| sum_array | 313→313; 7,581,952→7,581,952 | 313→313; 7,581,952→7,581,952 |

The requested pipeline gate is not green because of the unchanged baseline
seeded-pointer performance assertion. Final task status: EXIT 1; no new
functional failures or main-suite benchmark regressions were found.
