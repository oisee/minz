# Task Z1 — zvdb master-detail demo for CP/M (Nanz)

Branch `feat/zvdb-demo-cpm`, no pushes. Compiler (`pkg/`) untouched.

## 1. What was built

An interactive vector-similarity browser for Z80 under CP/M in the spirit of
zvdb: 35 sentence embeddings from `../zvdb-ref/sample_embeddings.tsv`, each
quantised to 256 bits by fixed-seed random-hyperplane hashing, brute-force
searched with a 256-byte popcount LUT, shown as a VT100 master-detail screen.

Left pane: every record, one per line, label truncated to the pane, cursor
moved with VT100 arrows (`ESC [ A` / `ESC [ B`, decoded by
`stdlib/tui/render.nanz`) or `k` / `j`; the list scrolls when the cursor leaves
the window. Right pane: the top K = 19 most similar *other* records, score and
label per line, descending score, ties by ascending record index (identical
rule on both sides of the Z80/Python comparison). Header holds the title and
the selection's full label; the status line holds key help plus the cost of the
last search. Only what changed is redrawn.

### Files

| file | what |
|---|---|
| `examples/nanz/zvdb_demo_cpm.nanz` | the demo: layout constants, master/detail panes, key loop, kernel asserts |
| `examples/nanz/zvdb_kernel.nanz` | the similarity kernel — popcount LUT, `zv_score`, `zv_top_k`, ranking accessors, `zv_fmt_u16` — console-free, 48 dual-backend asserts |
| `examples/nanz/zvdb_dump_cpm.nanz` | reference dumper: runs the demo's `zv_top_k` for every record and prints the full ranking in the reference file's format |
| `examples/nanz/zvdb_demo_data.nanz` | generated data module: N, label offset table, label blob, 32-byte vectors |
| `examples/nanz/zvdb_demo_expected.txt` | generated Python reference: `sel rank idx score label` for all 35 × 34 rankings |
| `scripts/zvdb_demo_data.py` | TSV → 256-bit random-hyperplane quantisation → data module + reference (stdlib only, ~0.6 s, bit-for-bit reproducible — re-running it leaves both files unchanged) |
| `scripts/zvdb_demo_verify.py` | end-to-end check + VT100 screen capture + T-state measurement |
| `minzc/pkg/pipeline/zvdb_demo_cpm_test.go` | Go tests: top-K ≡ Python reference for every record; scripted-session redraw contract |
| `minzc/pkg/pipeline/zvdb_demo_layout_test.go` | Go tests: VT100 screen reconstruction, N=120 no-overflow, N=252 top-K vs an independent reference, 51-column variant |
| `examples/nanz/zvdb_bench.nanz` | T-state bench for `zv_top_k` and the scoring pass, with an idle baseline |
| `scripts/ci/pipeline_profiles.json` | the two new tests registered in the `fast` profile |
| `reports/zvdb-demo-cpm/` | captured screens and codegen repros (+ `repro/README.md`); build products are gitignored |

Layout is parameterised by `SCR_W`, `SCR_H`, `ROW_*`, `N_ROWS`, `LEFT_X/W`,
`RIGHT_X/W`, `IDX_W`, `SCORE_W`, `LABEL_L_W`, `LABEL_R_W`; no other 80-column
assumption exists. I/O is `stdlib/tui/render.nanz` (`tui_goto`, `tui_color`,
`tui_putch`, `tui_puts`, `puts_padded`, `tui_read_key`, `tui_clear`) — the same
layer `tetris_cpm.nanz` uses; no new I/O layer.

## 2. How to run

```sh
cd repo
nice -n 19 taskset -c 4-15 minzc/mz examples/nanz/zvdb_demo_cpm.nanz -t cpm -o zvdb.com
nice -n 19 taskset -c 4-15 minzc/mze zvdb.com -t cpm          # interactive
```

Interactive: arrow keys or `j`/`k` to move, `q` or `ESC` to quit.

Headless with scripted keys (BDOS function 1 reads stdin; `--cpm-no-echo`
keeps the capture clean, `-c` prints the T-state total). **Always end the
script with `q`** — a real CP/M console blocks, but mze's BDOS 1 returns 0 at
EOF and the demo ignores unknown keys:

```sh
printf '\033[B\033[B\033[Bkjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjq' \
  | nice -n 19 taskset -c 4-15 minzc/mze zvdb.com -t cpm --cpm-no-echo -c
```

That is: 3× down (arrows), 1× up (`k`), 30× down (`j`, scrolls), quit.

Two-step production build (assembler explicitly):

```sh
minzc/mz examples/nanz/zvdb_demo_cpm.nanz -t cpm -o reports/zvdb-demo-cpm/zvdb_demo_cpm.a80
minzc/mza reports/zvdb-demo-cpm/zvdb_demo_cpm.a80 -o zvdb.com
```

Regenerate the data module and the reference from the TSV, then run the whole
verification and screen capture:

```sh
python3 scripts/zvdb_demo_data.py            # rewrites the data module + reference
python3 scripts/zvdb_demo_verify.py          # build, run, compare, capture, measure
```

## 3. Captured screens (scripted session, escapes stripped)

From `reports/zvdb-demo-cpm/screen-*.txt`. The `<== cursor` tag marks the row
the demo draws in inverse video — SGR carries the highlight, plain text cannot.
Master pane is columns 0–35 (a 32-character label field), a `|` divider at
column 36, detail pane columns 38–79.

**start**

```
 zvdb - Z80 vector similarity (CP/M)  records: 35  256-bit hyperplanes
Rec   0: The forest is calm and serene during the early morning.
#   record                          | #  scr  nearest records
  0 The forest is calm and serene du|   1 214 Birds chirp harmoniously as the su   <== cursor
  1 Birds chirp harmoniously as the |   3 214 Walking in the woods is a great wa
  2 Nature always finds a way to rej|  18 209 Smartphones have become an integra
  3 Walking in the woods is a great |  21 208 I love the taste of freshly baked
  4 The river flows gracefully throu|   4 206 The river flows gracefully through
  5 I love exploring new destination|  14 203 The library is a treasure trove of
  6 Traveling broadens the mind and |  15 203 Advancements in technology have re
  7 My next vacation will be to a tr|   2 202 Nature always finds a way to rejuv
  8 Backpacking across Europe was an|   9 202 The journey is often more importan
  9 The journey is often more import|  13 202 Classic novels always have timeles
 10 A good book can transport you to|  30 202 ABAP is programming language for S
 11 Literature provides a window int|  11 200 Literature provides a window into
 12 Reading is a journey of the mind|  26 200 ABAP is for SAP ERP.
 13 Classic novels always have timel|  17 199 The digital revolution has transfo
 14 The library is a treasure trove |   8 198 Backpacking across Europe was an u
 15 Advancements in technology have |  10 198 A good book can transport you to a
 16 Artificial intelligence is the f|  12 198 Reading is a journey of the mind a
 17 The digital revolution has trans|  31 198 ABAP is ABAP ERP.
 18 Smartphones have become an integ|   7 197 My next vacation will be to a trop
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs scanned
```

**3× down (VT100 arrows)**

```
 zvdb - Z80 vector similarity (CP/M)  records: 35  256-bit hyperplanes
Rec   3: Walking in the woods is a great way to find inner peace.
#   record                          | #  scr  nearest records
  0 The forest is calm and serene du|   0 214 The forest is calm and serene duri
  1 Birds chirp harmoniously as the |   2 210 Nature always finds a way to rejuv
  2 Nature always finds a way to rej|   1 208 Birds chirp harmoniously as the su
  3 Walking in the woods is a great |   9 206 The journey is often more importan   <== cursor
  4 The river flows gracefully throu|  18 205 Smartphones have become an integra
  5 I love exploring new destination|   4 202 The river flows gracefully through
  6 Traveling broadens the mind and |  14 201 The library is a treasure trove of
  7 My next vacation will be to a tr|  33 201 S/4 HANA is ancestor or SAP R/3.
  8 Backpacking across Europe was an|  21 200 I love the taste of freshly baked
  9 The journey is often more import|  29 200 CDS is the best HANA DB option.
 10 A good book can transport you to|  30 200 ABAP is programming language for S
 11 Literature provides a window int|   5 199 I love exploring new destinations
 12 Reading is a journey of the mind|   8 198 Backpacking across Europe was an u
 13 Classic novels always have timel|  10 198 A good book can transport you to a
 14 The library is a treasure trove |  11 198 Literature provides a window into
 15 Advancements in technology have |   7 197 My next vacation will be to a trop
 16 Artificial intelligence is the f|  32 197 S/4 HANA is SAP ERP.
 17 The digital revolution has trans|  13 196 Classic novels always have timeles
 18 Smartphones have become an integ|  28 196 S/4 HANA is the best ERP system.
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs scanned
```

**1× up (`k`)**

```
 zvdb - Z80 vector similarity (CP/M)  records: 35  256-bit hyperplanes
Rec   2: Nature always finds a way to rejuvenate itself.
#   record                          | #  scr  nearest records
  0 The forest is calm and serene du|   3 210 Walking in the woods is a great wa
  1 Birds chirp harmoniously as the |   0 202 The forest is calm and serene duri
  2 Nature always finds a way to rej|  15 201 Advancements in technology have re   <== cursor
  3 Walking in the woods is a great |   1 200 Birds chirp harmoniously as the su
  4 The river flows gracefully throu|   4 200 The river flows gracefully through
  5 I love exploring new destination|   8 200 Backpacking across Europe was an u
  6 Traveling broadens the mind and |  11 200 Literature provides a window into
  7 My next vacation will be to a tr|   5 199 I love exploring new destinations
  8 Backpacking across Europe was an|  14 199 The library is a treasure trove of
  9 The journey is often more import|  17 199 The digital revolution has transfo
 10 A good book can transport you to|  24 199 A balanced diet is crucial for mai
 11 Literature provides a window int|  13 198 Classic novels always have timeles
 12 Reading is a journey of the mind|  23 197 Dining out is a great way to exper
 13 Classic novels always have timel|  33 197 S/4 HANA is ancestor or SAP R/3.
 14 The library is a treasure trove |  16 196 Artificial intelligence is the fut
 15 Advancements in technology have |  21 196 I love the taste of freshly baked
 16 Artificial intelligence is the f|   9 194 The journey is often more importan
 17 The digital revolution has trans|  12 194 Reading is a journey of the mind a
 18 Smartphones have become an integ|  18 193 Smartphones have become an integra
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs scanned
```

**30× down (`j`, scrolled — window now 14…32)**

```
 zvdb - Z80 vector similarity (CP/M)  records: 35  256-bit hyperplanes
Rec  32: S/4 HANA is SAP ERP.
#   record                          | #  scr  nearest records
 14 The library is a treasure trove |  33 236 S/4 HANA is ancestor or SAP R/3.
 15 Advancements in technology have |  28 233 S/4 HANA is the best ERP system.
 16 Artificial intelligence is the f|  27 226 S/4 HANA uses ABAP and JS.
 17 The digital revolution has trans|  31 225 ABAP is ABAP ERP.
 18 Smartphones have become an integ|  26 223 ABAP is for SAP ERP.
 19 Innovative tech startups are cha|  30 223 ABAP is programming language for S
 20 Italian cuisine is known for its|  25 220 ABAP is the best programming langu
 21 I love the taste of freshly bake|  29 217 CDS is the best HANA DB option.
 22 Sushi is a delicate art form as |  34 215 CDS is the best for ABAP.
 23 Dining out is a great way to exp|  17 204 The digital revolution has transfo
 24 A balanced diet is crucial for m|  19 203 Innovative tech startups are chang
 25 ABAP is the best programming lan|  14 198 The library is a treasure trove of
 26 ABAP is for SAP ERP.            |  15 198 Advancements in technology have re
 27 S/4 HANA uses ABAP and JS.      |   1 197 Birds chirp harmoniously as the su
 28 S/4 HANA is the best ERP system.|   3 197 Walking in the woods is a great wa
 29 CDS is the best HANA DB option. |  18 196 Smartphones have become an integra
 30 ABAP is programming language for|   0 195 The forest is calm and serene duri
 31 ABAP is ABAP ERP.               |   9 195 The journey is often more importan
 32 S/4 HANA is SAP ERP.            |  10 195 A good book can transport you to a   <== cursor
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs scanned
```

(The ABAP cluster ranks itself first — record 32's nearest neighbours are
records 33, 28, 27, 31, 26, 30, 25, 29, 34; the quantisation is doing real
semantic work.)

`q` clears the screen and exits; the whole scripted session is 63,122 bytes of
terminal output and 52,956,952 T-states.

## 4. Per-keypress search cost

Measured with `mze -c` (whole-process T-states), taking the difference between
runs with N and 0 cursor moves:

| measurement | T-states | at 4 MHz |
|---|---|---|
| startup: first search + full draw | 1,657,124 | 414 ms |
| `zv_top_k` alone (one full ranking of 34 others, `zvdb_bench.nanz`, idle-subtracted) | 756,156 | 189 ms |
| scoring pass alone (34 × `zv_score`, `zvdb_bench.nanz`, idle-subtracted) | 382,123 | 96 ms |
| **marginal cost per cursor move** (no scroll) | **1,344,844** | **336 ms** |
| marginal cost per cursor move (10 moves) | 1,351,671 | 338 ms |
| marginal cost per cursor move (34 moves, incl. scrolling redraws) | 1,531,981 | 383 ms |
| records scanned per search | 35 (34 comparisons, self skipped) | — |

So one cursor move ≈ 1.34M T-states ≈ 756k search + ~590k redraw. Under `mze`
there is no real-time throttling, so the demo is instant in the emulator; on a
4 MHz machine a move takes ~0.34 s, of which about half is VT100 output
(1,407 bytes per move) — at 9,600 baud the terminal, not the CPU, is the
bottleneck.

**The status line prints only the record count.** `mze` reports whole-process
T-states after the program exits; the running Z80 has no readable cycle
counter, so the program cannot obtain its own per-search cost. Task Z1 allows
exactly this fallback.

Incremental redraw, verified on the byte stream: one cursor move emits 1,407
bytes against 2,618 for the initial full draw; the screen is cleared exactly
twice (startup, shutdown); the title, both pane headers and the separator are
written exactly once each while the label, detail and status rows are rewritten
once per move.

## 5. Verification results

### 5.1 Compiles, assembles, runs under `mze -t cpm`

```sh
$ minzc/mz examples/nanz/zvdb_demo_cpm.nanz -t cpm -o zvdb.com
$ ls -l zvdb.com
-rw-r--r-- 1 alice alice 10560 zvdb.com
$ minzc/mza reports/zvdb-demo-cpm/zvdb_demo_cpm.a80 -o zvdb.com   # two-step path
$ printf '\033[B\033[B\033[Bkjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjq' \
    | minzc/mze zvdb.com -t cpm --cpm-no-echo -c | tail -c 40
Total execution: 52956952 T-states
```

### 5.2 Z80 top-K equals the Python reference for every record

Go test (runs the production pipeline, assembles, emulates with a BDOS hook):

```sh
$ cd minzc && go test ./pkg/pipeline/ -run TestZVDBDemo -v -count=1
=== RUN   TestZVDBDemoTopKMatchesPythonReference
    zvdb_demo_cpm_test.go:122: 1190 rows identical
--- PASS: TestZVDBDemoTopKMatchesPythonReference (0.50s)
=== RUN   TestZVDBDemoCPMScriptedSession
--- PASS: TestZVDBDemoCPMScriptedSession (0.88s)
PASS
ok      github.com/minz/minzc/pkg/pipeline	1.392s
```

Script (builds the dumper, runs it under `mze`, diffs against the reference):

```sh
$ python3 scripts/zvdb_demo_verify.py
[1] Z80 top-K vs Python reference: 1190/1190 rows, 0 mismatches -> PASS (91316556 T-states for all 35 searches)
[2] start   keys=  0B out= 2628B clears=1 T=1657124
[2] down3   keys=  9B out= 6849B clears=1 T=5705324
[2] up1     keys= 10B out= 8256B clears=1 T=7048664
[2] down30  keys= 40B out=63080B clears=1 T=52956952
[2] incremental redraw: one cursor move emits 1407B (full repaint 2618B), chrome untouched -> PASS
[3] startup + first search + full draw: 1657124 T-states
[3] marginal cost per cursor move over  1 moves: 1,344,844 T-states
[3] marginal cost per cursor move over 10 moves: 1,351,671 T-states
[3] marginal cost per cursor move over 34 moves: 1,531,981 T-states
artifacts in /home/alice/minz-workers/zvdb/repo/reports/zvdb-demo-cpm
```

1190 = 35 records × 34 ranks, every `sel rank idx score label` row identical.

### 5.3 Asserts pass via mir2 and via z80

```sh
$ minzc/mz examples/nanz/zvdb_kernel.nanz --assert-receipt --asserts-force mir2 -o /dev/null
ASSERTS: executed=48 passed=48 failed=0
$ minzc/mz examples/nanz/zvdb_kernel.nanz --assert-receipt --asserts-force z80 -o /dev/null
ASSERTS: executed=48 passed=48 failed=0
$ minzc/mz examples/nanz/zvdb_demo_cpm.nanz --assert-receipt --asserts-force mir2 -o /dev/null
ASSERTS: executed=14 passed=14 failed=0
$ minzc/mz examples/nanz/zvdb_demo_cpm.nanz --assert-receipt --asserts-force z80 -o /dev/null
ASSERTS: executed=14 passed=14 failed=0
```

62 asserts in total, both backends, covering `zv_popcount`, `zv_score`,
`zv_top_count`, `zv_top_index`, `zv_top_score` (including rank 0, rank 1, the
last rank, and a tie — records 1 and 3 both score 214 against record 0 and are
ordered by index) and `zv_fmt_u16`.

### 5.4 Repo assert matrix

The two new files sit under `examples/nanz/*.nanz`, so `scripts/assert_matrix.py`
picks them up in its default sweep:

```
$ python3 scripts/assert_matrix.py --mz minzc/mz --glob 'examples/nanz/zvdb*.nanz' --no-controls -j 4
... 124 classified rows (62 asserts x mir2 + z80), every classification "pass"
```

### 5.5 No regressions in the pipeline suite

```sh
$ cd minzc && python3 ../scripts/ci/run_stable_pipeline.py \
      --profile fast --output /tmp/fast-out --shards 4 --workers 4
Pipeline inventory: 956 total, 585 selected (fast, job 1/1), 4 exhaustive process shards, 4 workers
$ cat /tmp/fast-out/statuses.json | python3 -c "import json,sys; print([s['exit_code'] for s in json.load(sys.stdin)])"
[0, 0, 0, 0]
$ grep -c '^--- FAIL' /tmp/fast-out/shard-*.log
shard-1.log:0  shard-2.log:0  shard-3.log:0  shard-4.log:0
$ grep -h ZVDB /tmp/fast-out/shard-*.log
--- PASS: TestZVDBDemoTopKMatchesPythonReference (0.44s)
--- PASS: TestZVDBDemoCPMScriptedSession (0.79s)
```

(A single unsharded `go test ./pkg/pipeline/ -count=1` hits Go's default 10-minute
panic while still in `TestDivModModuleVMZ80` — that suite is why the repo ships
the sharded runner. Not a regression: it is unrelated to these files.)

### 5.6 Boundary behaviour (scripted)


Cost of a key sequence relative to an idle run, from the same `mze -c` harness:

```
k at top (should be a no-op)           delta vs idle =          820 T
ESC [ A at top (no-op)                 delta vs idle =          927 T
40 j (clamps at the last record)       delta vs idle =  50,183,512 T
34 j then 34 k (down and back)         delta vs idle = 100,329,464 T
```

`k` / `ESC [ A` at record 0 and `j` past record 34 redraw nothing (the two
no-op rows cost only the BDOS reads); 40 `j` costs the same 34 moves as walking
to the end, and the round trip costs twice that.

## 6. Compiler findings

No correctness bug was hit: the Z80 and MIR2 backends agreed with the Python
reference on every one of the 1190 ranking rows. Two **codegen quality**
findings, both reproduced and measured in `reports/zvdb-demo-cpm/repro/`
(numbers and analysis in `repro/README.md`):

1. **32-byte XOR + popcount-LUT scan, 2.42× more T-states than hand Z80 (measured; see Round 2 §2.7).**
   `repro-scan-nanz.nanz` (plain Nanz, i.e. `zvdb_kernel.zv_score_pv`) costs
   84,734,801 T for 250 passes of 34 scans = 9,969 T per scan (~311 T per
   byte); `repro-scan-asm.nanz` (same scan in inline Z80) costs 35,017,301 T
   = 4,120 T per scan. The generated loop keeps three pointers plus a 16-bit
   accumulator live, so the allocator spills the `b` pointer and the
   accumulator to memory and rematerialises the LUT base each iteration via
   `PUSH IX / POP HL / ADD HL,DE`; hand Z80 holds `HL`=a, `DE`=b, `BC`=LUT,
   `IX`=sum, `IYL`=counter with no spills.
   **Deliberately not worked around.** An `asm z80` body cannot be evaluated
   by the MIR2 VM (`assert f(..) == v via mir2` fails with "function returned
   no value"), so replacing the scan would break task Z1's requirement that
   `score`/`top_k` asserts pass on both backends. The kernel stays plain Nanz
   and costs 756k instead of an estimated ~290-480k T-states per search.
2. **Dynamically indexed `u16` global array element move costs 309 T-states.**
   `repro-array-move.nanz` measures `zv_val[k] = zv_val[k-1]` +
   `zv_idx[k] = zv_idx[k-1]` at 309 T per element: the emitted code
   re-materialises both array bases, doubles the index with `SLA C / RL B`,
   stages the loaded low byte through `PUSH AF` and routes the destination
   address through `PUSH IX / POP IX`. A pointer walk would be ~6 T per step.
   Three ranking layouts were measured before choosing one: parallel byte
   arrays by index 1.00M T per `zv_top_k`, parallel byte arrays by pointer walk
   1.54M, and the shipped `u16` score + `u8` index arrays by index 721k — the
   shipped form is the fastest of the three, so the finding is recorded rather
   than worked around.

## 7. What a ZX Spectrum 51-column port needs

The port is a constants change plus an I/O swap; nothing in the kernel or the
layout logic moves.

1. **Layout constants.** In `zvdb_demo_cpm.nanz` edit three literals:
   `SCR_W 80 → 51` and `LEFT_W 36 → 24` (`SCR_H` stays 24); `layout_init()`
   derives every other width, so `SEP_X` becomes 24, `RIGHT_X` 26, `RIGHT_W` 25
   and the label fields 20 and 17 characters. `N_ROWS 19` stays, so K is
   unchanged and the reference file needs no regeneration. This is verified, not
   asserted: `TestZVDBDemoNarrowScreenLayout` performs exactly this rewrite,
   rebuilds and checks that nothing is written past column 51. `SCR_W` must be
   at least 49 (the status literal plus a three-digit record count), which 51
   clears.
2. **I/O layer.** `import tui.render` is the only console dependency; the
   Spectrum needs a `tui.render` equivalent writing to the ZX attribute map
   instead of VT100 — `tui_goto` becomes a screen-address computation
   (`0x4000 + y*32 + x` with the third-of-screen fold), `tui_color` becomes an
   attribute-byte write at `0x5800 + y*32 + x`, `tui_clear` clears both maps,
   and `tui_read_key` swaps BDOS 1 for the keyboard matrix (`stdlib/input/
   keyboard.nanz`). `puts_padded` needs a Spectrum twin; it is the single
   hottest output primitive (1,405 bytes per cursor move).
3. **Memory.** The demo image is 10,560 bytes with all data and code; the
   ranking buffers (250 + 500 + 250 bytes) can be cut to `zv_n - 1` entries
   (34/68/34) for the Spectrum, and `zv_rank_cap` lowered to match.
4. **Input.** `j`/`k` work as-is; `q`/ESC should become a Spectrum key
   (`q` is fine, ESC is not on the matrix) and `KEY_UP`/`KEY_DOWN` (128/129,
   produced by `tui_read_key`) need Spectrum values from the new reader.
5. **Colour.** Only 8-colour SGR codes are used; the Spectrum has 8 INK + 8
   PAPER with two brightness levels, so the existing `tui_color(fg, bg, bright)`
   signature maps 1:1 onto an attribute byte.
6. **Unchanged.** `zvdb_kernel.nanz`, `zvdb_demo_data.nanz`, the Python
   generator, the reference file and the Go/Python equivalence tests all carry
   over verbatim — the dump test would only need its assembler target changed
   from `cpm` to the Spectrum target.

Agon MOS needs the same two steps with its own dimensions and `stdlib/agon/*`.

## Round 2 — critic fixes (CRITIC-1.md, GO-WITH-FIXES)

All three blockers and should-fix 1, 2, 3, 4 and 6 are applied. Should-fix 5 is
**not** applied and is blocked by the task's own constraint; see 2.5. Two new
compiler findings came out of the round, one of them silent wrong code.

### 2.1 The three blockers

**Divider column (blocker 1).** `SEP_X` was dead and the master rows were one
column narrower than the header, so the `|` sat at column 35 in the 19 item
rows and 36 in the header. The label field is now derived as
`label_l_w = LEFT_W - IDX_W - 1`, so a row is exactly `LEFT_W` columns and the
divider always lands on `LEFT_X + LEFT_W`. Measured on the captured stream: the
divider is at column 36 on the header row and on all 19 item rows
(`reports/zvdb-demo-cpm/screen-start.txt`).

**u8 underflow for 101..250 records (blocker 2).** `ui_put_num` computed
`width - n` on u8; a three-digit index in a two-digit field wrapped to 255
spaces and destroyed the screen. Fixed three ways: the pad is guarded
(`if n < width`), `IDX_W` is 3 (0..254 needs three digits), and the label row
uses `IDX_W` and `SCR_W - 6 - IDX_W` instead of literals. Verified at N=120 by
`TestZVDBDemoWideDataSetDoesNotOverflow`, which walks the cursor to record 119
and asserts zero characters past column 80; with the pre-fix demo the same run
reports 140,005 characters past column 80 and a label line truncated to "Rec".

**Out-of-bounds shift in `zv_top_k` (blocker 3).** The guard was on `p`, not on
`cnt`, so once the ranking filled up the first store wrote `zv_top_val[cap]`
and clobbered the next array. Fixed by raising `zv_rank_cap` from 250 to 255 —
one slot per candidate, which covers the whole u8 record range and is the actual
repair — plus clamping both the shift start and the count as defence in depth.
`scripts/zvdb_demo_data.py` now allows any N up to 255 to match. Verified at
N=252 by `TestZVDBDemoTopKAtCapacityMatchesReference`; against the pre-fix
kernel that test fails 8 ways, including record 0 appearing in its own top-K at
rank 1:

```
sel 0 rank 0: z80 ['0','0','109','151'] want [sel rank 161 151]
sel 0 rank 1: z80 ['0','1','0','150']   want [sel rank 60 150]     <- record 0
```

### 2.2 Layout widths are derived, and a 51-column build proves it (should-fix 1, 2)

Every width is now an expression over `SCR_W`, `SCR_H`, `LEFT_W`, `GAP_W`,
`N_ROWS` and the field widths; `draw_title`, `draw_sep`, `draw_status` and
`draw_label_line` contain no 80-column literals. The title and status rows are
`[head literal][number][tail]` with the head field exactly as wide as its own
literal (so the text stays contiguous) and the tail absorbing the rest of the
row. `LABEL_R_W`'s stale comment is gone — the derivation is
`right_w - IDX_W - SCORE_W - 2`, computed, not described.

The derived values cannot be `const` expressions: see finding 2.4. They are
computed once in `layout_init()`.

`TestZVDBDemoNarrowScreenLayout` rewrites `SCR_W 80 -> 51` and `LEFT_W 36 -> 24`
in a temp copy, rebuilds and asserts nothing is written past column 51 and the
divider still lines up. Round 1's §7 claim ("the port is a constants change plus
an I/O swap") is now true for the layout; §7's constant list has been corrected
to name only `SCR_W`, `SCR_H` and `LEFT_W`.

### 2.3 Screen content is now asserted (should-fix 3)

`minzc/pkg/pipeline/zvdb_demo_layout_test.go` adds a ~90-line VT100
reconstructor (CUP, ED, EL, CUU/CUD/CUF/CUB, SGR, printable text) that records
the text grid, the rows painted in the selection colours, the column of every
divider and a count of characters written past the right edge.
`TestZVDBDemoCPMScreenContent` uses it on the scripted session and checks the
label line, the status line and the highlighted master row, plus the two
invariants (one divider column across header and item rows; no overflow).
Blocker 1 and blocker 2 are both visible to this test and to
`TestZVDBDemoWideDataSetDoesNotOverflow`.

Mutation check — each defect reintroduced in turn is caught, and the tree is
restored afterwards:

```
mutation                                                       caught by
label field one column narrow (blocker 1)                      ScreenContent, NarrowScreenLayout
IDX_W back to 2 + pad guard removed (blocker 2)                ScreenContent, WideDataSet
ranking cap 250 + guards removed (blocker 3)                   TopKAtCapacity
fixed 25-column title tail (should-fix 1)                      NarrowScreenLayout
separator rule back to 80 dashes (should-fix 1)                NarrowScreenLayout
restored: tree matches the fixed sources, all 6 tests green
```

### 2.4 New compiler finding: a `const` initialiser referencing another `const` is 0

Writing the derived widths as const expressions compiled cleanly, passed every
assert, and produced a screen with the divider at column 0 and no label text at
all. `reports/zvdb-demo-cpm/repro/repro-const-expr.nanz` is the minimal repro:

```nanz
const A: u8 = 10
const B: u8 = A + 5      // compiles; B reads back as 0
```

Both backends agree, in either declaration order, while the same arithmetic at
run time is correct — silent wrong code, not a diagnostic. Workaround (applied):
only literal consts, derived widths computed at run time in `layout_init()`.

### 2.5 Should-fix 5 is blocked: `tui_read_key` cannot change without touching `pkg/`

The fix itself is written and verified —
`reports/zvdb-demo-cpm/repro/tui_read_key-sequence-decode.patch` consumes whole
CSI/SS3 sequences and returns 255 for an undecoded one, so only a lone ESC
quits. Measured on a patched build, `<sequence> j q` for PageUp, Home, Insert,
Delete, Shift-Tab, SS3-up, DECID and Ctrl-Down all move the cursor (sequence
ignored) while bare ESC still quits.

It is **not** applied. The guarded CP/M runtime admits `tui_read_key` only
through a byte-exact canonical-body table in
`minzc/pkg/mir2/cpm_runtime_tui_bodies.go`; with the patch applied
`TestOriginalTetrisCPMMainNative` and `TestPublicCPMTetrisNative` fail with
`CP/M runtime rejects ASM without canonical body: tui__render__tui_read_key`,
and `stdlib/tui/render.nanz` is additionally pinned by SHA-256 in
`reports/tetris-main-native/freeze-corrected.json` and
`reports/tetris-family-native/freeze.json`. Fixing it is a compiler change, and
task Z1 forbids changing `pkg/`. With the patch reverted all Tetris tests pass
again. Consequence for the demo as shipped: on a real VT100 keyboard Home,
PageUp, Delete, an F-key or Shift-Tab quits the demo, because `tui_read_key`
reports every undecoded CSI as 27.

Two by-products recorded in `repro/README.md`: a `/` inside an `asm z80` block
is a statement separator even inside a `;` comment (it turned a comment into
three bogus instructions and made the assembler blame an unrelated `LD A, B`),
and the freeze files that would have to be re-frozen alongside any future
change to that asm.

### 2.6 Artifact and bench hygiene (should-fix 4, 6)

`reports/zvdb-demo-cpm/zvdb_dump.txt` (69 KB, byte-identical to
`examples/nanz/zvdb_demo_expected.txt` modulo the header lines) is removed from
the commit and ignored; `scripts/zvdb_demo_verify.py` still writes it under its
`--artifacts` dir for local inspection. The two T-state benches that round 1
quoted without committing are now `examples/nanz/zvdb_bench.nanz` — one module,
three bodies sharing a loop nest, selected by the first key read, so the harness
cost can be subtracted. Both repro scan headers no longer describe the wrong
variant or the wrong output name.

### 2.6b Nice-to-haves applied

2 (`ui_put_blank` inlined away), 3 (`old_row` is a `let` local again, not a
global), 6 (a key of 0 — end of input on the host side, unreachable on real
CP/M — now quits, so a key script with no trailing `q` terminates instead of
spinning to the cycle limit) and 9 (both repro scan headers corrected). 7 was
also picked up incidentally: `zv_popcount` now carries an "assert surface only"
note so nobody wires the hot loop through it.

Verified for 6, on the shipped build:

```
$ printf 'jj' | mze zvdb.com -t cpm --cpm-no-echo -c | tail -c 40
Total execution: 4345407 T-states      # exits rc=0 after the two moves
```

### 2.7 Corrected numbers

| measurement | round 1 | round 2 | note |
|---|---|---|---|
| scan ratio, plain Nanz vs hand Z80 | "about 3x" | **2.42x** | 9,969 / 4,120 T per scan, both measured |
| `zv_top_k` alone | 720,814 T | **756,156 T** | now from `zvdb_bench.nanz`, idle baseline subtracted |
| scoring pass alone | 401,593 T | **382,123 T** | same bench, same subtraction |
| ranking half (difference) | — | **≈374k T** | |
| startup + first draw | 1,596,950 T | 1,657,124 T | |
| marginal cost per cursor move | 1,313,039 T | **1,344,844 T** | |
| …over 10 moves | 1,318,295 T | 1,351,671 T | |
| …over 34 moves (with scrolling) | 1,475,831 T | 1,531,981 T | |
| one cursor move emits | 1,405 B | 1,407 B | full repaint 2,618 B |
| whole scripted session | 62,797 B / 51,090,494 T | 63,122 B / 52,956,952 T | |
| `.com` image | 10,336 B | 10,560 B | |
| all-35 dump | 89,397,136 T | 91,316,556 T | |

The per-keypress cost rose ~2.4% because `IDX_W` went from 2 to 3 and the
status/title rows now pad to the full screen width.

On the ratio: **2.42x is the measured figure** for the whole scan, harness
included, and is what should be quoted. Round 1's "~3x" and its "~250k instead
of ~721k per pass" extrapolation subtracted an *assumed* common harness cost of
roughly 1,170 T per scan (4,120 T minus 32 x ~92 T of hand-Z80 loop) to isolate
the loop body; that subtraction is an estimate, not a measurement. Scaling the
whole measured pass by 2.42x, or replacing just the scoring half with
34 x 4,120 T, puts an all-asm kernel at roughly 290-480k T per pass rather than
~250k — direction and order of magnitude unchanged, headline rounded up.

### 2.8 Verification after the fixes

```
$ for f in zvdb_kernel zvdb_demo_cpm zvdb_dump_cpm zvdb_bench; do for be in mir2 z80; do
    minzc/mz examples/nanz/$f.nanz --assert-receipt --asserts-force $be -o /dev/null | tail -1; done; done
ASSERTS: executed=48 passed=48 failed=0      # zvdb_kernel / mir2
ASSERTS: executed=48 passed=48 failed=0      # zvdb_kernel / z80
ASSERTS: executed=14 passed=14 failed=0      # zvdb_demo_cpm / mir2
ASSERTS: executed=14 passed=14 failed=0      # zvdb_demo_cpm / z80
ASSERTS: executed=0 passed=0 failed=0        # zvdb_dump_cpm and zvdb_bench, both backends

$ go -C minzc test ./pkg/pipeline/ -run TestZVDBDemo -v -count=1
--- PASS: TestZVDBDemoTopKMatchesPythonReference (0.50s)     1190 rows identical
--- PASS: TestZVDBDemoCPMScriptedSession (0.73s)
--- PASS: TestZVDBDemoCPMScreenContent (0.77s)
--- PASS: TestZVDBDemoWideDataSetDoesNotOverflow (2.62s)
--- PASS: TestZVDBDemoTopKAtCapacityMatchesReference (0.48s)
--- PASS: TestZVDBDemoNarrowScreenLayout (0.61s)
ok      github.com/minz/minzc/pkg/pipeline	5.734s

$ python3 scripts/assert_matrix.py --mz minzc/mz --glob 'examples/nanz/zvdb*.nanz' --no-controls -j 4
124 classified rows, every one "pass"

$ python3 scripts/zvdb_demo_verify.py
[1] Z80 top-K vs Python reference: 1190/1190 rows, 0 mismatches -> PASS (91316556 T-states ...)
[2] incremental redraw: one cursor move emits 1407B (full repaint 2618B), chrome untouched -> PASS
[3] marginal cost per cursor move over  1 moves: 1,344,844 T-states
```

The two new Go tests cost 2.6 s and 0.5 s and are registered in the `fast`
profile (now 589 entries, inventory check clean). The sharded fast profile
re-run after the fixes:

```
$ (cd minzc && python3 ../scripts/ci/run_stable_pipeline.py --profile fast \
      --output /tmp/fast2 --shards 4 --workers 4)
Pipeline inventory: 960 total, 589 selected (fast, job 1/1), 4 exhaustive process shards, 4 workers
$ python3 -c "import json;print([s['exit_code'] for s in json.load(open('/tmp/fast2/statuses.json'))])"
[0, 0, 0, 0]
$ grep -hc '^--- FAIL' /tmp/fast2/shard-*.log        # 0 0 0 0
$ grep -hc '^--- PASS' /tmp/fast2/shard-*.log | paste -sd+ | bc   # 557  (+32 skipped = 589)
$ grep -h ZVDB /tmp/fast2/shard-*.log
--- PASS: TestZVDBDemoTopKMatchesPythonReference (0.49s)
--- PASS: TestZVDBDemoTopKAtCapacityMatchesReference (0.44s)
--- PASS: TestZVDBDemoCPMScriptedSession (1.01s)
--- PASS: TestZVDBDemoNarrowScreenLayout (0.72s)
--- PASS: TestZVDBDemoCPMScreenContent (0.75s)
--- PASS: TestZVDBDemoWideDataSetDoesNotOverflow (2.54s)
```

STATUS: DONE
