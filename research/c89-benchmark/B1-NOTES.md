# B1 corpus verification

The corpus was frozen in commit `ddb93f8a3` before compiler experiments. No
optimization work or compiler changes were made. Main now has 12 cases; heldout
has 5, including its 2 existing cases. Sources were unchanged after freezing.

The bitwise port retains both upstream functions and packs their byte returns
into result (AND low, OR high). The loop-11 port retains the signed countdown
and index stores, with a variable byte count and explicit 16-bit elements so GCC
and Z80 agree on array layout. Its workloads include the original 199 elements.
Checksum16 uses big-endian pairs, odd-byte zero padding, and no complement.

Validation used `TMPDIR=/mnt/ssd-510/minz/tmp`,
`GOCACHE=/mnt/ssd-510/minz/gocache`, nice level 19 and at most four jobs:

```sh
cd minzc
nice -n 19 go build -o /mnt/ssd-510/minz/tmp/measure ../research/c89-benchmark/measure.go
cd ..
nice -n 19 python3 research/c89-benchmark/run.py --out /mnt/ssd-510/minz/bench/b1-codex --sdcc-root / --suite all --jobs 4
nice -n 19 python3 research/c89-benchmark/check_host.py --out /mnt/ssd-510/minz/bench/b1-codex
```

All 45 builds of the nine new cases PASS, including raw/final MIR replay and
native execution for all three MinZ profiles. No MinZ miscompile or failing
input was observed. All 17 GCC replays PASS (26,562 calls). Across the entire
matrix, 83 of 85 builds PASS. The two existing SDCC `struct_roundtrip` profiles
remain COMPILE_ERROR: SDCC rejects returning aggregates and passing structs as
arguments. These errors are retained in summary.json. sccz80 was skipped because
no z88dk root was supplied.

Artifacts, full observations, workload/source hashes, commands and logs are in
`/mnt/ssd-510/minz/bench/b1-codex`. T below is the total over each case's workload,
not per-call timing; workloads differ between cases. Size includes globals, per
the harness definition. No profile selection or expectation adjustment was used.

Toolchain: `SDCC : mcs51/z80/z180/r2k/r2ka/r3ka/sm83/tlcs90/ez80_z80/z80n/ds390/TININative/ds400/hc08/s08/stm8/pdk13/pdk14/pdk15/mos6502 4.2.0 #13081 (Linux)`; host: `gcc (Ubuntu 13.3.0-6ubuntu2~24.04.1) 13.3.0`.

| Case | MinZ default bytes/T | SDCC size bytes/T | SDCC speed bytes/T | Status |
|---|---:|---:|---:|---|
| abs_diff | 27/220,904 | 34/264,286 | 34/264,286 | PASS |
| copy_bytes | 182/301,600 | 214/831,090 | 215/822,770 | PASS |
| fat12_decode | 167/32,000 | 149/29,184 | 159/27,392 | PASS |
| gcd | 74/5,109,358 | 55/4,398,206 | 57/4,249,984 | PASS |
| struct_roundtrip | 85/935,936 | COMPILE_ERROR | COMPILE_ERROR | SDCC COMPILE_ERROR |
| sum_array | 313/7,581,952 | 325/9,929,216 | 326/9,798,656 | PASS |
| bitwise8reg | 199/1,808,384 | 82/727,040 | 82/727,040 | PASS |
| reverse_loop11 | 592/47,729,280 | 553/15,009,920 | 554/14,683,520 | PASS |
| crc8 | 399/367,390,393 | 348/154,289,658 | 346/152,886,724 | PASS |
| mul8x8 | 75/2,836,992 | 41/1,033,472 | 42/1,012,992 | PASS |
| strlen_bytes | 294/25,150,080 | 277/9,409,920 | 278/9,083,520 | PASS |
| memset_fill | 304/25,911,040 | 294/15,137,280 | 295/14,810,880 | PASS |
| count_less | 322/90,325,939 | 331/92,641,459 | 332/91,466,419 | PASS |
| in_interval | 76/528,554 | 41/308,156 | 41/308,156 | PASS |
| popcount8 | 410/660,152,320 | 303/94,268,800 | 304/93,942,400 | PASS |
| find_max | 303/18,354,082 | 296/16,288,767 | 297/15,962,367 | PASS |
| checksum16 | 405/61,474,616 | 370/36,613,176 | 366/36,409,656 | PASS |
