# P24: masked byte comparison loses its test under register pressure

The production HIR → MIR2 → PBQP → Z80 path compiled the shift/add
`mul8x8(1, 2)` loop to 3 instead of 2. Both raw and optimized MIR2 execute
correctly. The relevant expression survives MIR optimization unchanged:

```text
%r11 = const 1 : u8 [general]
%r12 = and %r7, %r11 : u8 [general]
%r13 = const 0 : u8 [general]
%r14 = cmp.ne %r12, %r13 : bool [flag]
```

The failure is in Z80 emission, not MIR rewriting, constant propagation or
narrowing. `scanBitCmpPatterns` accepts this expression for `BIT 0,A` and
suppresses the AND and zero definitions. At `genCmp`, ordinary CP setup
preserves the loop's live accumulator before attempting the BIT. With A,
DE and HL live and B/C occupied by the original comparison operands, the
preservation scratch is IYH. `physOverride` therefore relocates the BIT
source to IYH. BIT cannot address an index half, so emission falls through
to ordinary CP even though its producers have already been suppressed:

```asm
LD IYH, A       ; preserve live accumulator before CMP
LD A, C
CP B            ; neither C's masked value nor B's zero was emitted
```

Class: **post-allocation fusion invalidated by operand relocation, followed
by an unsound fallback using elided producers**. It affects single-bit
masked equality/inequality against zero and explicit MIR2 BitGet tests
when the source is live in A under register pressure. Multi-bit masks and
`== mask` provide controls for the unfused path.

The change emits the accepted BIT immediately after pending-flag preservation,
before CP's accumulator preservation. BIT preserves A, so that relocation
is unnecessary. The rest of comparison emission and fusion matching stays
unchanged.

## Reduction and failing-first evidence

A byte-only count loop passes: retaining the word loop state is necessary
to reproduce the allocation pressure. The source regression reduces the
original loop to two byte parameters, two word accumulators and one masked
conditional add, with no helpers or imports. `TestMaskedBytePredicatesVMZ80`
parameterizes this shape. A smaller hand-built MIR fixture removes the loop
entirely, pins the three live parameters to A/DE/HL, and branches on either
AND+compare or BitGet+compare. Thus the loop triggers the allocation; the
underlying bug also applies to straight-line MIR.

Before the change:

- Original Z80 assertion: `mul8x8(1, 2)` got 3, wanted 2.
- `TestBitCmpLiveAccumulatorFusion` fails: expected BIT absent; emitted
  `LD IYH,A; LD A,C; CP B`.
- Exhaustive source matrix fails six loop subcases: masks 1, 2 and 128,
  each with `!= 0` and `== 0`. Mask 15 and `== mask` are passing controls.
- Raw/final MIR2 retain AND+compare, ruling out the MIR passes.

The source matrix checks all 256 bytes × four masks (1, 2, 128, 15) × three
predicates (`!= 0`, `== 0`, `== mask`) × two control-flow forms, or **6,144
cases per engine**. Each case runs raw MIR2, final MIR2, and assembled Z80
against a host-computed result. The original four multiply assertions are
also retained. Both new pipeline tests are in the sorted `fast` manifest.

## Generator coverage

The original fuzzer's `Gen.cmp` only selects bare u16 names from `vs`;
it cannot select byte variables from `v8`, a mask expression or a zero
literal. Its while counters are also u16. Increasing the seed range does
not repair this structural blind spot.

`--masked-loops` adds a separate opt-in corpus with a byte-controlled while,
all four masks and all three comparisons, and live word state. Division by
2 expresses the shift within the existing Python oracle's supported subset.
Default/signed seed mappings and all argument generation remain unchanged.
Judge self-tests check the independent result, coverage of all 12 predicate
combinations, determinism, and existing default corpus fingerprints. On the
unmodified compiler, opt-in seeds 0–99 pass 46/100: the other 54 are Z80/MIR2
mismatches, with zero MIR2/oracle, assembly, compiler or oracle errors.

## Validation

Fresh validation logs and before/after dumps are in
`/mnt/ssd-510/minz/tmp/p24-evidence/`. All builds and execution use
`nice -n 19`, SSD TMPDIR/GOCACHE, GOMAXPROCS=4 and at most four jobs.

Completed checks:

- `go test -p 4 ./pkg/mir2/... ./pkg/hir/... ./pkg/c89/... ./pkg/nanz/... -count=1`:
  all pass (229.689s / 97.425s / 192.221s / 227.588s).
- `go test -p 4 ./pkg/nanz -run TestShowcaseExecutableOracleCatalog`: pass.
- `go test -p 4 ./cmd/minzc -run TestMetaSpecialize`: pass.
- Focused pipeline tests, including the pre-existing low-bit-before-shift
  regression: pass (17.482s).
- `python3 scripts/test_judges.py`: 42 tests, OK, five existing skips.
