# C89/Nanz benchmark against SDCC 4.6, 2026-11-25

Raw rows behind [report 139](../2026-11-25-139-SDCC-46-Benchmark.md).

| | |
|---|---|
| Compiler tree | `9daf7a8ba` (`origin/main` after #330) |
| SDCC | 4.6.0 (#16555), code-size and code-speed profiles |
| Host | Linux x86-64 (T-states are emulated Z80 counts and do not depend on the host) |
| Rows | 220, all `Status: PASS` (130 in `all-summary.json`, 90 in `fusion-summary.json`) |
| sccz80, SDCC 4.5 | not run (no `--z88dk-root`, no `--sdcc45-root`) |

Every row's `SourceSHA256` matches a source file under `research/c89-benchmark/` at `9daf7a8ba`
(checked by hashing every `.c`, `.nanz`, `.a80`, `.asm` and `.s` file there; 0 of 220 rows unmatched).
The SDCC version and the commit are recorded here by the person who ran the benchmark; the JSON rows
themselves carry no toolchain field.

## Files

| File | Content |
|---|---|
| `all-summary.json` | `run.py --suite all --include-nanz`: 12 main + 5 heldout C cases (`minz` default/size/grace/nograce, `sdcc` size/speed), 6 Nanz ports of main cases (`nanz`), 1 tuple variant (`nanz-tuple`) |
| `fusion-summary.json` | `run.py --suite fusion`: 9 iterator-fusion C cases, plus idiomatic Nanz (`nanz-idiom` default/size/grace) and hand-written Z80 (`hand` asm) for the same 9 cases |
| `geomean.py`, `geomean.txt` | geometric means of MinZ/SDCC ratios per profile pair |
| `tables.py`, `tables.md` | headline, per-case, Nanz and "where SDCC wins" tables used in report 139 |

Row fields: `Compiler` (`minz` = MinZ compiling the C source; `nanz`, `nanz-idiom`, `nanz-tuple` =
Nanz sources; `hand` = hand-written assembly; `sdcc`), `Profile`, `Case`, `TotalT` (Z80 T-states of
the case's whole frozen workload), `ProgramBytes` (occupied image bytes plus declared data storage, as
defined in `research/c89-benchmark/README.md`).

## Commands

The two runs, from the repository root at `9daf7a8ba` (host paths replaced by placeholders; the
driver writes `summary.json` into `--out`, renamed here to `all-summary.json` and
`fusion-summary.json`):

```sh
python3 research/c89-benchmark/run.py --suite all --include-nanz \
  --sdcc-root <SDCC_4_6_ROOT> --out <OUT_ALL>
python3 research/c89-benchmark/run.py --suite fusion \
  --sdcc-root <SDCC_4_6_ROOT> --out <OUT_FUSION>
```

`<SDCC_4_6_ROOT>` contains `usr/bin/sdcc` and `usr/share/sdcc/lib/z80`. `--jobs` changes only wall
time, not T-states. `--include-refs` was not passed to the `all` run, so idiomatic Nanz and
hand-written rows exist only for the 9 fusion cases.

Regenerate the derived files from this directory:

```sh
python3 geomean.py . > geomean.txt
python3 tables.py .  > tables.md
```

## What the ratio means

`geomean.txt` keys are `(compiler, profile, sdcc profile)`, values `(cases, T ratio, bytes ratio)`.
Each ratio is the geometric mean, over the cases both sides have, of MinZ value / SDCC value.
Below 1 means MinZ needs fewer T-states or bytes than that SDCC profile; 0.629 means 37 % fewer
T-states on the geometric mean, not on every case. The 26 cases are a small frozen set chosen by the
MinZ project; per-case results vary in both directions (see `tables.md`). These files do not show
that MinZ is better than SDCC on other programs.
