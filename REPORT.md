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
| `scripts/ci/pipeline_profiles.json` | the two new tests registered in the `fast` profile |
| `reports/zvdb-demo-cpm/` | captured screens, dump, codegen repros (+ `repro/README.md`) |

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
 zvdb - vector similarity on Z80 (CP/M)  records:    35  quantised: 256 bits
Rec  0: The forest is calm and serene during the early morning.
#   record                          | #  scr  nearest records
 0 The forest is calm and serene du|   1 214 Birds chirp harmoniously as the sun   <== cursor
 1 Birds chirp harmoniously as the |   3 214 Walking in the woods is a great way
 2 Nature always finds a way to rej|  18 209 Smartphones have become an integral
 3 Walking in the woods is a great |  21 208 I love the taste of freshly baked b
 4 The river flows gracefully throu|   4 206 The river flows gracefully through
 5 I love exploring new destination|  14 203 The library is a treasure trove of
 6 Traveling broadens the mind and |  15 203 Advancements in technology have res
 7 My next vacation will be to a tr|   2 202 Nature always finds a way to rejuve
 8 Backpacking across Europe was an|   9 202 The journey is often more important
 9 The journey is often more import|  13 202 Classic novels always have timeless
10 A good book can transport you to|  30 202 ABAP is programming language for SA
11 Literature provides a window int|  11 200 Literature provides a window into t
12 Reading is a journey of the mind|  26 200 ABAP is for SAP ERP.
13 Classic novels always have timel|  17 199 The digital revolution has transfor
14 The library is a treasure trove |   8 198 Backpacking across Europe was an un
15 Advancements in technology have |  10 198 A good book can transport you to an
16 Artificial intelligence is the f|  12 198 Reading is a journey of the mind an
17 The digital revolution has trans|  31 198 ABAP is ABAP ERP.
18 Smartphones have become an integ|   7 197 My next vacation will be to a tropi
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs
```

**3× down (VT100 arrows)**

```
 zvdb - vector similarity on Z80 (CP/M)  records:    35  quantised: 256 bits
Rec  3: Walking in the woods is a great way to find inner peace.
#   record                          | #  scr  nearest records
 0 The forest is calm and serene du|   0 214 The forest is calm and serene durin
 1 Birds chirp harmoniously as the |   2 210 Nature always finds a way to rejuve
 2 Nature always finds a way to rej|   1 208 Birds chirp harmoniously as the sun
 3 Walking in the woods is a great |   9 206 The journey is often more important   <== cursor
 4 The river flows gracefully throu|  18 205 Smartphones have become an integral
 5 I love exploring new destination|   4 202 The river flows gracefully through
 6 Traveling broadens the mind and |  14 201 The library is a treasure trove of
 7 My next vacation will be to a tr|  33 201 S/4 HANA is ancestor or SAP R/3.
 8 Backpacking across Europe was an|  21 200 I love the taste of freshly baked b
 9 The journey is often more import|  29 200 CDS is the best HANA DB option.
10 A good book can transport you to|  30 200 ABAP is programming language for SA
11 Literature provides a window int|   5 199 I love exploring new destinations a
12 Reading is a journey of the mind|   8 198 Backpacking across Europe was an un
13 Classic novels always have timel|  10 198 A good book can transport you to an
14 The library is a treasure trove |  11 198 Literature provides a window into t
15 Advancements in technology have |   7 197 My next vacation will be to a tropi
16 Artificial intelligence is the f|  32 197 S/4 HANA is SAP ERP.
17 The digital revolution has trans|  13 196 Classic novels always have timeless
18 Smartphones have become an integ|  28 196 S/4 HANA is the best ERP system.
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs
```

**1× up (`k`)**

```
 zvdb - vector similarity on Z80 (CP/M)  records:    35  quantised: 256 bits
Rec  2: Nature always finds a way to rejuvenate itself.
#   record                          | #  scr  nearest records
 0 The forest is calm and serene du|   3 210 Walking in the woods is a great way
 1 Birds chirp harmoniously as the |   0 202 The forest is calm and serene durin
 2 Nature always finds a way to rej|  15 201 Advancements in technology have res   <== cursor
 3 Walking in the woods is a great |   1 200 Birds chirp harmoniously as the sun
 4 The river flows gracefully throu|   4 200 The river flows gracefully through
 5 I love exploring new destination|   8 200 Backpacking across Europe was an un
 6 Traveling broadens the mind and |  11 200 Literature provides a window into t
 7 My next vacation will be to a tr|   5 199 I love exploring new destinations a
 8 Backpacking across Europe was an|  14 199 The library is a treasure trove of
 9 The journey is often more import|  17 199 The digital revolution has transfor
10 A good book can transport you to|  24 199 A balanced diet is crucial for main
11 Literature provides a window int|  13 198 Classic novels always have timeless
12 Reading is a journey of the mind|  23 197 Dining out is a great way to experi
13 Classic novels always have timel|  33 197 S/4 HANA is ancestor or SAP R/3.
14 The library is a treasure trove |  16 196 Artificial intelligence is the futu
15 Advancements in technology have |  21 196 I love the taste of freshly baked b
16 Artificial intelligence is the f|   9 194 The journey is often more important
17 The digital revolution has trans|  12 194 Reading is a journey of the mind an
18 Smartphones have become an integ|  18 193 Smartphones have become an integral
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs
```

**30× down (`j`, scrolled — window now 14…32)**

```
 zvdb - vector similarity on Z80 (CP/M)  records:    35  quantised: 256 bits
Rec 32: S/4 HANA is SAP ERP.
#   record                          | #  scr  nearest records
14 The library is a treasure trove |  33 236 S/4 HANA is ancestor or SAP R/3.
15 Advancements in technology have |  28 233 S/4 HANA is the best ERP system.
16 Artificial intelligence is the f|  27 226 S/4 HANA uses ABAP and JS.
17 The digital revolution has trans|  31 225 ABAP is ABAP ERP.
18 Smartphones have become an integ|  26 223 ABAP is for SAP ERP.
19 Innovative tech startups are cha|  30 223 ABAP is programming language for SA
20 Italian cuisine is known for its|  25 220 ABAP is the best programming langua
21 I love the taste of freshly bake|  29 217 CDS is the best HANA DB option.
22 Sushi is a delicate art form as |  34 215 CDS is the best for ABAP.
23 Dining out is a great way to exp|  17 204 The digital revolution has transfor
24 A balanced diet is crucial for m|  19 203 Innovative tech startups are changi
25 ABAP is the best programming lan|  14 198 The library is a treasure trove of
26 ABAP is for SAP ERP.            |  15 198 Advancements in technology have res
27 S/4 HANA uses ABAP and JS.      |   1 197 Birds chirp harmoniously as the sun
28 S/4 HANA is the best ERP system.|   3 197 Walking in the woods is a great way
29 CDS is the best HANA DB option. |  18 196 Smartphones have become an integral
30 ABAP is programming language for|   0 195 The forest is calm and serene durin
31 ABAP is ABAP ERP.               |   9 195 The journey is often more important
32 S/4 HANA is SAP ERP.            |  10 195 A good book can transport you to an   <== cursor
--------------------------------------------------------------------------------
 j/k or UP/DN move   q/ESC quit   last search: 35 recs
```

(The ABAP cluster ranks itself first — record 32's nearest neighbours are
records 33, 28, 27, 31, 26, 30, 25, 29, 34; the quantisation is doing real
semantic work.)

`q` clears the screen and exits; the whole scripted session is 62,797 bytes of
terminal output and 51,090,494 T-states.

## 4. Per-keypress search cost

Measured with `mze -c` (whole-process T-states), taking the difference between
runs with N and 0 cursor moves:

| measurement | T-states | at 4 MHz |
|---|---|---|
| startup: first search + full draw | 1,596,950 | 399 ms |
| `zv_top_k` alone (one full ranking of 34 others, isolated 250-rep bench) | 720,814 | 180 ms |
| scoring pass alone (34 × `zv_score`, isolated) | 401,593 | 100 ms |
| **marginal cost per cursor move** (no scroll) | **1,313,039** | **328 ms** |
| marginal cost per cursor move (10 moves) | 1,318,295 | 330 ms |
| marginal cost per cursor move (34 moves, incl. scrolling redraws) | 1,475,831 | 369 ms |
| records scanned per search | 35 (34 comparisons, self skipped) | — |

So one cursor move ≈ 1.31M T-states ≈ 721k search + ~590k redraw. Under `mze`
there is no real-time throttling, so the demo is instant in the emulator; on a
4 MHz machine a move takes ~0.33 s, of which about half is VT100 output
(1,405 bytes per move) — at 9,600 baud the terminal, not the CPU, is the
bottleneck.

**The status line prints only the record count.** `mze` reports whole-process
T-states after the program exits; the running Z80 has no readable cycle
counter, so the program cannot obtain its own per-search cost. Task Z1 allows
exactly this fallback.

Incremental redraw, verified on the byte stream: one cursor move emits 1,405
bytes against 2,599 for the initial full draw; the screen is cleared exactly
twice (startup, shutdown); the title, both pane headers and the separator are
written exactly once each while the label, detail and status rows are rewritten
once per move.

## 5. Verification results

### 5.1 Compiles, assembles, runs under `mze -t cpm`

```sh
$ minzc/mz examples/nanz/zvdb_demo_cpm.nanz -t cpm -o zvdb.com
$ ls -l zvdb.com
-rw-r--r-- 1 alice alice 10336 zvdb.com
$ minzc/mza reports/zvdb-demo-cpm/zvdb_demo_cpm.a80 -o zvdb.com   # two-step path
$ printf '\033[B\033[B\033[Bkjjjjjjjjjjjjjjjjjjjjjjjjjjjjjjq' \
    | minzc/mze zvdb.com -t cpm --cpm-no-echo -c | tail -c 40
Total execution: 51090494 T-states
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
[1] Z80 top-K vs Python reference: 1190/1190 rows, 0 mismatches -> PASS (89397136 T-states for all 35 searches)
[2] start   keys=  0B out= 2609B clears=1 T=1596950
[2] down3   keys=  9B out= 6824B clears=1 T=5545115
[2] up1     keys= 10B out= 8229B clears=1 T=6859647
[2] down30  keys= 40B out=62755B clears=1 T=51090494
[2] incremental redraw: one cursor move emits 1405B (full repaint 2599B), chrome untouched -> PASS
[3] startup + first search + full draw: 1596950 T-states
[3] marginal cost per cursor move over  1 moves: 1,313,039 T-states
[3] marginal cost per cursor move over 10 moves: 1,318,295 T-states
[3] marginal cost per cursor move over 34 moves: 1,475,831 T-states
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

1. **32-byte XOR + popcount-LUT scan, ~3× more T-states than hand Z80.**
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
   and costs 721k instead of ~250k T-states per search.
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

1. **Layout constants.** In `zvdb_demo_cpm.nanz`: `SCR_W 80 → 51`, `SCR_H` stays
   24, and the derived widths must be re-derived by hand (they are declared as
   separate constants, not expressions): `LEFT_W`, `SEP_X`, `RIGHT_X`,
   `RIGHT_W`, `LABEL_L_W`, `LABEL_R_W`. A sane 51-column split is
   `LEFT_W 24`, `SEP_X 24`, `RIGHT_X 25`, `RIGHT_W 26`, giving label fields of
   21 and 18 characters. `N_ROWS 19` can stay, so K is unchanged and the
   reference file needs no regeneration.
2. **I/O layer.** `import tui.render` is the only console dependency; the
   Spectrum needs a `tui.render` equivalent writing to the ZX attribute map
   instead of VT100 — `tui_goto` becomes a screen-address computation
   (`0x4000 + y*32 + x` with the third-of-screen fold), `tui_color` becomes an
   attribute-byte write at `0x5800 + y*32 + x`, `tui_clear` clears both maps,
   and `tui_read_key` swaps BDOS 1 for the keyboard matrix (`stdlib/input/
   keyboard.nanz`). `puts_padded` needs a Spectrum twin; it is the single
   hottest output primitive (1,405 bytes per cursor move).
3. **Memory.** The demo image is 10,336 bytes with all data and code; the
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

STATUS: DONE
