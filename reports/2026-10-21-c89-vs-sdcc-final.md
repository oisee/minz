# MinZ C vs SDCC 4.2: where we win and where we lose (2026-10-21, final)

**MinZ commit:** `e41e0ca20` (main after the three optimisations #226, #228, #227).
**Toolchains:** SDCC 4.2.0 #13081 (size and speed profiles), z88dk `1ce28d0e77` sccz80 `-O3`, host GCC 13.3.0.
**Corpus:** 12 main cases + 5 heldout cases, frozen in #224 before any optimisation.
**Artifacts:** [summary.json](c89-benchmark-2026-10-21-final/summary.json),
[toolchains.json](c89-benchmark-2026-10-21-final/toolchains.json),
[host-replay.json](c89-benchmark-2026-10-21-final/host-replay.json).

```sh
python3 research/c89-benchmark/run.py --out <dir> --sdcc-root / --z88dk-root ~/dev/z88dk \
  --suite all --include-nanz --jobs 6
python3 research/c89-benchmark/check_host.py --out <dir>
```

123 builds: 120 PASS, 3 COMPILE_ERROR (struct_roundtrip on SDCC size/speed and sccz80, as before).
Host GCC replay: 17/17 cases, 26 562 calls PASS. Every MinZ build is also replayed on the raw and
final MIR VM.

Correctness of the combined compiler on `e41e0ca20`: direct-call fuzz seeds 0–19999 20000/20000,
`--signed` 0–3999 4000/4000, folded 0–999 1000/1000.

## Headline

| Suite | MinZ T before | MinZ T now | Change | Best SDCC T | T lost to SDCC, before → now |
|---|---:|---:|---:|---:|---:|
| Main (11 cases SDCC compiles) | 484.1M | 269.0M | **−44.4 %** | 208.4M | 278.5M → **63.4M** |
| Heldout (5 cases, never used for tuning) | 830.8M | 520.6M | **−37.3 %** | 238.1M | 593.9M → 291.3M |

Bytes on the main suite: 2626 → 2419 (SDCC best per case: 2370). Heldout: 1516 → 1464 (SDCC 1337).

MinZ now beats the better SDCC profile on speed in 3 of 11 main cases and on size in 4; on
heldout in 1 of 5 for each. MinZ beats sccz80 `-O3` on speed in 15 of 16 comparable cases;
the exception is reverse_loop11 (39.7M vs 32.0M T).

## Per case (MinZ default; bytes / T-states over the fixed workload)

Ratios are MinZ ÷ best SDCC profile; below 1 means MinZ is better.

| Case | Before | Now | SDCC size | SDCC speed | sccz80 | T ratio | B ratio |
|---|---:|---:|---:|---:|---:|---:|---:|
| abs_diff | 27 / 220 904 | 27 / 220 904 | 34 / 264 286 | 34 / 264 286 | 87 / 696 320 | **0.84** | **0.79** |
| copy_bytes | 182 / 301 600 | 182 / 301 600 | 214 / 831 090 | 215 / 822 770 | 230 / 1 474 590 | **0.37** | **0.85** |
| sum_array | 313 / 7 581 952 | 313 / 7 581 952 | 325 / 9 929 216 | 326 / 9 798 656 | 375 / 27 157 248 | **0.77** | **0.96** |
| crc8 | 399 / 367 390 393 | 342 / 176 060 962 | 348 / 154 289 658 | 346 / 152 886 724 | 410 / 343 028 137 | 1.15 | **0.99** |
| fat12_decode | 167 / 32 000 | 167 / 30 336 | 149 / 29 184 | 159 / 27 392 | 207 / 57 952 | 1.11 | 1.12 |
| gcd | 74 / 5 109 358 | 57 / 4 953 966 | 55 / 4 398 206 | 57 / 4 249 984 | 168 / 28 596 874 | 1.17 | 1.04 |
| bitwise8reg | 199 / 1 808 384 | 101 / 872 448 | 82 / 727 040 | 82 / 727 040 | 266 / 3 317 760 | 1.20 | 1.23 |
| memset_fill | 304 / 25 911 040 | 299 / 18 403 840 | 294 / 15 137 280 | 295 / 14 810 880 | 318 / 33 372 800 | 1.24 | 1.02 |
| mul8x8 | 75 / 2 836 992 | 51 / 1 331 712 | 41 / 1 033 472 | 42 / 1 012 992 | 130 / 5 817 600 | 1.31 | 1.24 |
| strlen_bytes | 294 / 25 150 080 | 293 / 19 468 800 | 277 / 9 409 920 | 278 / 9 083 520 | 288 / 19 954 560 | 2.14 | 1.06 |
| reverse_loop11 | 592 / 47 729 280 | 587 / 39 732 480 | 553 / 15 009 920 | 554 / 14 683 520 | 565 / 32 045 440 | 2.71 | 1.06 |
| struct_roundtrip | 85 / 935 936 | 71 / 612 352 | compile error | compile error | compile error | — | — |
| *Heldout* | | | | | | | |
| count_less | 322 / 90 325 939 | 322 / 82 688 179 | 331 / 92 641 459 | 332 / 91 466 419 | 381 / 191 678 425 | **0.90** | **0.97** |
| find_max | 303 / 18 354 082 | 303 / 18 354 082 | 296 / 16 288 767 | 297 / 15 962 367 | 348 / 40 078 586 | 1.15 | 1.02 |
| checksum16 | 405 / 61 474 616 | 401 / 58 206 776 | 370 / 36 613 176 | 366 / 36 409 656 | 441 / 94 991 592 | 1.60 | 1.10 |
| in_interval | 76 / 528 554 | 76 / 516 266 | 41 / 308 156 | 41 / 308 156 | 109 / 999 804 | 1.68 | 1.85 |
| popcount8 | 410 / 660 152 320 | 362 / 360 843 520 | 303 / 94 268 800 | 304 / 93 942 400 | 395 / 514 383 360 | 3.84 | 1.19 |

With Grace (`--grace`), gcd drops to 51 B / 3 067 013 T, which beats both SDCC profiles on both
axes; crc8 to 328 B / 163.3M T, smaller than SDCC.

## What changed

| PR | Loss class | Main suite effect | Heldout effect |
|---|---|---|---|
| #226 O1 | Byte arithmetic left in 16 bits after C promotion | 12 cells better: bitwise8reg 199→101 B and −52 % T, crc8 −19 % T, struct_roundtrip −35 % T | popcount8 −14 % T |
| #228 O2 | Byte compares against constants done in 16 bits | 12 cells better: crc8 −41 % T (on top of O1), mul8x8 −50 % T, gcd 74→57 B | popcount8 −37 % T |
| #227 O3 | 16-bit moves via PUSH/POP, ±1 via HL copies | 15 cells better: memset_fill and strlen_bytes −29 % T, reverse_loop11 −17 % T | count_less −8 %, checksum16 −5 % T |

Each PR passed: a test that fails without the change; direct-call fuzz 0–19999 20000/20000;
folded fuzz 1000/1000; assert_matrix with 0 newly failing and 0 unexpected control passes; the
main-suite matrix with 0 regressed cells in any MinZ profile; heldout checked once, after the
change was final, with 0 regressed cells; and an independent critic GO.

## Where SDCC still wins, and why

From the loss classification ([report](2026-10-21-c89-loss-classes.md)), what remains:

1. **Loop variable kept in A** (crc8, memset_fill, popcount8, mul8x8). Every ALU op, compare and
   byte load around it saves and restores A. Fix lives in PBQP costs; the hardest remaining item.
2. **Signed and zero compares in codegen** (reverse_loop11, strlen_bytes). `i >= 0` still goes
   through `SBC HL` and a sign trampoline instead of `BIT 7,H`; `x != 0` on a word through `SBC`
   instead of `LD A,H; OR L`.
3. **No pointer induction for global arrays** (memset_fill, strlen_bytes, crc8, reverse_loop11,
   popcount8). `StrengthReduceCountedPointers` only accepts parameter bases; this is exactly what
   makes copy_bytes and sum_array beat SDCC.
4. **Small-function overhead** (in_interval, fat12_decode, bitwise8reg): calling-convention glue
   and an IY-assigned u16 parameter cost more than SDCC's straight-line code at this size.

## Limits

- All 17 cases are small kernels; they do not represent whole programs.
- T-states are emulator counts over a fixed workload: no ULA contention, interrupts or stack peak.
- bitwise8reg comes from SDCC's own regression suite, so SDCC may be tuned for it.
- SDCC 4.5 was not rerun (no local build on this host); on 2026-10-16 it was a few percent better
  than 4.2 on the original six cases.
