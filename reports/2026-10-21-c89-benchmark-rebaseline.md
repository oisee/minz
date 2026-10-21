# C89 benchmark re-baseline on current main (2026-10-21)

**MinZ commit:** `fa25ad366` (after P12–P22, #206–#220, and G1 #216).
**Toolchains:** SDCC 4.2.0 #13081 (system package, `--sdcc-root /`), z88dk `1ce28d0e77` (sccz80 `-O3`, helper archive rebuilt per README), host GCC 13.3.0.
**Artifacts:** [summary.json](c89-benchmark-2026-10-21/summary.json), [toolchains.json](c89-benchmark-2026-10-21/toolchains.json), [host-replay.json](c89-benchmark-2026-10-21/host-replay.json).

## Command

```sh
python3 research/c89-benchmark/run.py --out <dir> --sdcc-root / \
  --z88dk-root ~/dev/z88dk --include-nanz --jobs 6
python3 research/c89-benchmark/check_host.py --out <dir>
```

`--z88dk-root` is now optional: without it the sccz80 profile is skipped and the
MinZ × SDCC matrix still runs. SDCC 4.5 was not rerun (no local 4.5 build on this host).

## Result

57 builds: 54 PASS, 3 COMPILE_ERROR (struct_roundtrip on SDCC size/speed and sccz80 — same as
2026-10-16). Host GCC replay: 6/6 cases, 6 850 calls PASS.

Cells are bytes / T-states (bytes = occupied image plus declared globals, overlaps once;
T-states = whole frozen workload).

| Case | minz default | minz size | minz grace | sdcc size | sdcc speed | sccz80 o3 | nanz default | nanz grace |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| abs_diff | 27 / 220 904 | 27 / 220 904 | 27 / 220 904 | 34 / 264 286 | 34 / 264 286 | 87 / 696 320 | 27 / 220 904 | 27 / 220 904 |
| gcd | 74 / 5 109 358 | 74 / 5 109 358 | 68 / 3 222 405 | 55 / 4 398 206 | 57 / 4 249 984 | 168 / 28 596 874 | 57 / 4 953 966 | 51 / 3 067 013 |
| sum_array | 313 / 7 581 952 | 313 / 7 581 952 | 313 / 7 581 952 | 325 / 9 929 216 | 326 / 9 798 656 | 375 / 27 157 248 | 317 / 8 308 684 | 317 / 8 308 684 |
| copy_bytes | 182 / 301 600 | 182 / 301 600 | 182 / 301 600 | 214 / 831 090 | 215 / 822 770 | 230 / 1 474 590 | 182 / 301 600 | 182 / 301 600 |
| fat12_decode | 167 / 32 000 | 167 / 32 000 | 167 / 32 000 | 149 / 29 184 | 159 / 27 392 | 207 / 57 952 | 167 / 32 000 | 167 / 32 000 |
| struct_roundtrip | 85 / 935 936 | 85 / 935 936 | 85 / 935 936 | compile error | compile error | compile error | 154 / 1 925 120 | 154 / 1 925 120 |

## Delta to 2026-10-16

Compared cell by cell with [reports/c89-full-benchmark/summary.json](c89-full-benchmark/summary.json)
(main `4c79b1f8`): **0 of 57 cells changed** (status, bytes, globals and T-states all identical).
The correctness fixes P12–P22 did not alter code for this corpus. The only rows missing are the
SDCC 4.5 ones, which were not rerun.

## Where SDCC 4.2 still wins

| Case | Best MinZ C | Best SDCC 4.2 | Gap |
|---|---|---|---|
| gcd | 68 B (grace) / 3 222 405 T (grace) | 55 B (size) / 4 249 984 T (speed) | +13 B; MinZ grace is faster, default is 20 % slower than SDCC speed |
| fat12_decode | 167 B / 32 000 T | 149 B (size) / 27 392 T (speed) | +18 B, +17 % T |

MinZ C default wins abs_diff, sum_array and copy_bytes on both size and speed, and is the only
C compiler here that builds struct_roundtrip. The six-case corpus is too small to generalise;
the next step widens it and freezes a heldout set before any optimisation.
