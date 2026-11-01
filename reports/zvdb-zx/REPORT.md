# Task Z4 — the zvdb master-detail demo on a 48K ZX Spectrum, at 51 columns

Branch `feat/zvdb-zx`, no pushes, everything committed locally. The compiler
itself is untouched: the only files changed under `minzc/pkg/` are the new Go
test the task asks for and the showcase-catalog count it asks to bump — no
`minzc/pkg/` *compiler* package was modified. One one-line fix went into
`stdlib/zx/text.nanz` (§6.2), and every finding has a repro under
`reports/zvdb-zx/repro/`.

## 1. What was built

The CP/M zvdb demo ported to the 48K Spectrum on the Z3 narrow-font window
engine: 35 quantised sentence embeddings, brute-force searched by the **same**
`examples/nanz/zvdb_kernel.nanz` over the **same**
`examples/nanz/zvdb_demo_data.nanz` (reached through the kernel's import, not
forked and not re-imported — see §6.1), drawn through `stdlib/zx/text.nanz`
into two `split_vertical` panes at 51, 42 or 32 columns.

| file | what |
|---|---|
| `examples/nanz/zvdb_demo_zx.nanz` | the demo: layout, panes, chrome, key loop, scrolling |
| `examples/nanz/zvdb_dump_zx.nanz` | the ranking dumper the kernel-equivalence test runs on a 48K machine |
| `minzc/pkg/pipeline/zvdb_demo_zx_test.go` | `TestZVDBZXKernelMatchesReference`, `TestZVDBZXScriptedSessionScreens` (10 whole-screen states + the timings), `TestZVDBZXGlobalNamespace` |
| `scripts/zvdb_zx_model.py` | the zvdb-specific model on top of `zx_narrow_text_model.py`: geometry, cell text, attribute rectangles |
| `scripts/zvdb_zx_verify.py` | CLI end-to-end proof through `mz`/`mza`/`mzx-headless`: builds, runs all three widths with native key scripts, OCRs both panes, checks the attributes, measures the settle times, writes the screenshots and the `.sna`, prints the memory map |
| `stdlib/zx/text.nanz` | one-line fix: `win_apply_attr` now paints the window's own rows (§6.2) |
| `scripts/ci/pipeline_profiles.json` | the three new tests registered in `fast` (595 entries) |
| `minzc/pkg/nanz/showcase_oracle_catalog_test.go` | corpus count 209 → **211** and the scope string reworded |
| `reports/zvdb-zx/` | `screenshot-w5/w6/w8.png`, `screenshot-session.png`, `zvdb_demo_zx.sna`, `repro/` |

### Screen

```
row  0    title bar            " zvdb Z80 sim  recs: 35  256-bit hyperplanes"
row  1    full label of the selected record
row  2    pane headers          "#   record" | "scr nearest records"
rows 3-21 master list (left) / top-K nearest other records (right), K = 19
row 22    separator rule
row 23    status bar            " JK/67 mv 123 w Q quit scan: 35 recs scanned"
```

`split_vertical(left_w, 1)` makes the panes and the divider; `left_w` is 24 cells
at 5 px (120 px) and 16 at 6/8 px, both multiples of 8 so the panes never share
an attribute cell. Both panes are then re-opened over the item rows only (same
x, same width, so the byte ranges the split chose are unchanged) so `win_scroll`
moves the list and not the chrome. The right pane shows `score label` best
first, exactly as the task specifies. Every pane row prints one cell short of
the pane edge, because the window layer scrolls when a print fills the window's
bottom row — a full-width row 18 would scroll the list the moment it was drawn.

### Keys (documented in the demo header)

| key | action |
|---|---|
| `J` or `6` | next record (down). 6/7 is the Sinclair cursor pair; the choice made here |
| `K` or `7` | previous record (up) |
| `1` / `2` / `3` | 51 / 42 / 32 columns, re-laid out (the whole screen is rebuilt: `text_set_width` clears it and closes every window) |
| `Q` | quit: blanks the screen, returns from `main`, and the compiler's ZX epilogue runs `DI / HALT` — the program halts with interrupts off, so re-run it rather than expecting a return to BASIC |

The letter keys are `J`, `K`, `Q`, not `j`, `k`, `q`: the unshifted keyboard
matrix reports uppercase ASCII, so a lowercase code never matches (§6.3). On a
real Spectrum these are the same physical keys.

### Redraw discipline

A cursor move that does not scroll repaints **no master text at all**: the old
and new highlighted rows are two `win_invert_row` calls (an attribute flip, the
only thing that changes), then the label line, the right pane and the status
line are rewritten. A move that scrolls down is one `win_scroll(0)` plus the one
new bottom row — the highlight is dropped first, because `win_scroll` moves the
attribute cells up with the pixels and would otherwise drag the inverted row up
the pane, one stale highlight per scroll. A move that scrolls up (which
`win_scroll` cannot express, it only moves text up) repaints the pane.

## 2. Memory map (48K Spectrum, image loaded at `$8000`)

```
$0000-$3FFF  ROM            only ever read (the font at $3D00)
$4000-$57FF  pixel bitmap   6144 bytes
$5800-$5AFF  attributes     768 bytes
$8000-$A666  code           demo + engine + kernel, 9.9 K
$A667-$A7E9  deletion masks NF5/NF6 top+bottom, 384 bytes
$A7EA-$AAE9  nf_packed6     768
$AAEA-$ADE9  nf_packed5     768
$ADEA-$DDE9  nf_shift       pre-shifted glyph table, 12288
$DDEA-$DEDC  engine state   keep masks, tx_colpx, window arrays, mirror, rotbuf
$DEDD-$DF24  zv_label_off   72
$DF25-$E575  zv_labels      1617
$E576-$E9D5  zv_vecs        1120  (35 x 32)
$E9D6-$EAD5  zv_pop         256   (popcount LUT)
$EAD7-$EDDB  zv_top_val + zv_top_idx  510 + 255
$EDDD-$EE04  KEYTAB         40
$EE24-$EEB5  demo state + string literals + __mul8
$EEC8        image end (28361 bytes)
```

**Free RAM above the image: `$EEC9..$FFFF` = 4407 bytes**, plus 4.3 K of unused
space inside the screen file's system-variable area is *not* counted (the demo
writes nothing above `$5AFF`). Initial SP is `$FFFF` (`mzx --run`) or `$FF58`
(the Go test); the demo uses fewer than 64 bytes of stack, so the whole 4.4 K is
genuinely free. It fits in 48 K with room to spare. A 255-record data module
would not: its vectors alone are 255 x 32 = 8160 bytes (against 4407 free), and
its labels would be roughly 255/35 x 1617 = 11.8 K.

Importing `zvdb_demo_data` directly as well as `zvdb_kernel` (as the CP/M demo
does) would ship the 2.8 K of labels and vectors **twice** — the compiler emits
an imported module's data once per importer. Measured on this demo: 31 155 bytes
(free RAM 1613) instead of 28 361 (free 4407). So the demo derives the record
count from the kernel's own asserted contract instead:
`zvui_nrec = zv_rank_count() + 1` after the first search (`zv_top_k` ranks every
record but the selection, which `zvdb_kernel.nanz` asserts: 34 for `zv_n` = 35).

## 3. Timings — keypress to screen settled, in frames

Measured two ways, agreeing: `TestZVDBZXScriptedSessionScreens` hashes the screen
after every frame on the in-process machine and takes the keypress frame to the
last frame whose hash differs; `scripts/zvdb_zx_verify.py` does the same on
mzx-headless's per-frame `.scr` dumps. One frame = 69 888 T-states at 3.5 MHz
(50 Hz).

| action | frames | T-states | at 3.5 MHz |
|---|---|---|---|
| cursor move, no scroll, 51 columns | 29–31 | ~2.0 M | 0.58 s |
| cursor move, no scroll, 42 columns | 31 | ~2.2 M | 0.62 s |
| cursor move, no scroll, 32 columns | 25–26 | ~1.8 M | 0.50 s |
| cursor move that scrolls, 51 columns | 34–35 | ~2.4 M | 0.69 s |
| width switch 51 → 42 | 84 | 5.9 M | 1.68 s |
| width switch 42 → 32 | 55 | 3.8 M | 1.10 s |
| width switch 32 → 51 | 90 | 6.3 M | 1.80 s |
| startup (font build + search + full draw) | 93 | 6.5 M | 1.86 s |

```
move @51:              [30 30 30 30 30 29 30 30 30 30 29 30 31 30 30 31 31 31 31 30 35 34 34 33 34 34 34 35 35 35 35 34 34 35] frames (max 35)
switch to 42 columns:  [84] frames (max 84)
move @42:              [31 31 31] frames (max 31)
switch to 32 columns:  [55] frames (max 55)
move @32:              [25 26 25] frames (max 26)
switch to 51 columns:  [90] frames (max 90)
move @51 (scrolled):   [35 34] frames (max 35)
```

For comparison, the CP/M demo needs 1.34 M T-states per cursor move (~0.34 s at
4 MHz) — the Spectrum pays about 1.5× more T-states because the right pane is
redrawn through the glyph blitter (~1925 T per character) rather than emitted as
1.4 KB of VT100 text. A width switch costs the same 1.86 M T-state font rebuild
Z3 measured, plus the full redraw.

## 4. How to run it

```sh
cd repo
# build (Linux)
minzc/mz examples/nanz/zvdb_demo_zx.nanz -o reports/zvdb-zx/zvdb_demo_zx.a80
minzc/mza -s reports/zvdb-zx/zvdb_demo_zx.sym reports/zvdb-zx/zvdb_demo_zx.a80 \
          -o reports/zvdb-zx/zvdb_demo_zx.bin          # ORG $8000
# 51 columns (the startup state), screenshot + snapshot
minzc/mzx-headless --model 48k --run reports/zvdb-zx/zvdb_demo_zx.bin@8000 \
    --frames 400 --screenshot reports/zvdb-zx/screenshot-w5.png \
    --save-snapshot reports/zvdb-zx/zvdb_demo_zx.sna
# 42 / 32 columns: press '2' / '3' (matrix row 3, bit 1 / bit 2) at frame 150
echo '[{"frame":150,"row":3,"bit":1,"down":true},{"frame":230,"row":3,"bit":1,"down":false}]' \
     > reports/zvdb-zx/keys-w6.json
minzc/mzx-headless --model 48k --run reports/zvdb-zx/zvdb_demo_zx.bin@8000 \
    --frames 900 --key-script reports/zvdb-zx/keys-w6.json \
    --screenshot reports/zvdb-zx/screenshot-w6.png     # bit 2 -> screenshot-w8.png
# interactive
minzc/mzx --model 48k --snapshot reports/zvdb-zx/zvdb_demo_zx.sna
# the whole verification (all widths, session, timings, screenshots, .sna, map)
python3 scripts/zvdb_zx_verify.py
```

On macOS, build the same tools there first — `cd minzc && make all` plus
`go build -tags mzx_headless -o mzx-headless ./cmd/mzx` — then run the commands
above unchanged (mzx builds on macOS; `minzc/cmd/mzx` splits its `select()`
code by OS, commit `eee4a89e2`).

Keys: `J`/`6` down, `K`/`7` up, `1`/`2`/`3` width, `Q` quit.

## 5. Verification

### 5.1 Kernel equivalence on the Spectrum build

`examples/nanz/zvdb_dump_zx.nanz` runs the kernel's `zv_top_k` for every record
on a 48K machine with the real ROM and copies each ranking into flat arrays;
the test reads them out of the in-process machine and compares with
`examples/nanz/zvdb_demo_expected.txt`. A Spectrum build has no stdout, so the
ranking leaves through memory instead of BDOS — the CP/M branch's dump approach,
minus the console.

```sh
$ go -C minzc test ./pkg/pipeline/ -run TestZVDBZXKernelMatchesReference -count=1 -v
    zvdb_demo_zx_test.go:536: 1190 rows identical across 35 selections, 30437228 T-states
--- PASS: TestZVDBZXKernelMatchesReference (0.80s)
```

1190 = 35 selections × 34 ranks, every `(idx, score)` pair equal to the Python
reference — the same 1190 rows the CP/M test proves, on the Spectrum target.

### 5.2 Whole-screen proof for the scripted session

`TestZVDBZXScriptedSessionScreens` (in the `fast` profile, 3.8 s) drives the
demo through **start, 3 down, 1 up, 30 down (scroll), `2`, `3`, `1`** with
native key-matrix presses, plus three moves after every width switch so the
scrolled screen is proved and the settle time measured at each width too. After
every step it diffs **all 6912 screen bytes** against an independent model
(`zvdbZXExpectedScreen`, a Go twin of `scripts/zvdb_zx_model.py`: the geometry,
the blit with keep masks, the divider rule, the wrap/scroll rule and the
attribute rectangles re-derived from the engine's documented contract, driven by
the reference ranking), and separately decodes both panes back to characters and
compares them cell for cell with what the reference predicts for the selected
record.

```sh
$ go -C minzc test ./pkg/pipeline/ -run TestZVDBZXScriptedSessionScreens -count=1 -v
=== RUN   TestZVDBZXScriptedSessionScreens
=== RUN   TestZVDBZXScriptedSessionScreens/start_(51_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/3_down
=== RUN   TestZVDBZXScriptedSessionScreens/1_up
=== RUN   TestZVDBZXScriptedSessionScreens/30_down_(scrolled)
=== RUN   TestZVDBZXScriptedSessionScreens/key_2_(42_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/3_moves_at_42_columns
=== RUN   TestZVDBZXScriptedSessionScreens/key_3_(32_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/3_moves_at_32_columns
=== RUN   TestZVDBZXScriptedSessionScreens/key_1_(51_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/2_moves_at_51_columns
--- PASS: TestZVDBZXScriptedSessionScreens (3.76s)
    --- PASS: TestZVDBZXScriptedSessionScreens/start_(51_columns) (0.00s)
    ... all 10 subtests PASS, 0 bytes differ in any of them ...
```

Each subtest reports `0 bitmap bytes inside the panes, 0 outside, 0 attribute
cells` on success. The model also encodes two things a text-only check would
miss: the divider rule survives only in the rows no chrome cell covers (rows
2–21), and the gutter columns between the panes are never printed in.

### 5.3 The independent CLI path

```sh
$ python3 scripts/zvdb_zx_verify.py
[w5] 5 px (51 columns): PASS
[w6] 6 px (42 columns): PASS
[w8] 8 px (32 columns): PASS
[session] 5 px after start, 3 down, 1 up, 30 down: PASS
[settle] move that does not scroll: 29 frames
[settle] move that scrolls the pane:  33 frames
[settle] startup (font build + search + full draw): 93 frames
[settle] switch to w6: 77 frames
[settle] switch to w8: 47 frames
[sna] .../reports/zvdb-zx/zvdb_demo_zx.sna
image 28361 bytes, $8000..$EEC8, free RAM to $FFFF: 4407 bytes
ALL PASS
```

This goes through the shipped binaries (`mz`, `mza`, `mzx-headless`) rather than
the in-process pipeline, OCRs both panes with `zx_narrow_text_ocr.py`, and checks
the attribute rectangles and the highlighted row against
`scripts/zvdb_zx_model.py`.

### 5.4 Existing tests

```sh
$ go -C minzc test ./pkg/pipeline/ -run 'TestZVDBDemo|TestZXNarrowText|TestZVDBZX' -count=1
--- PASS: TestZVDBDemoTopKMatchesPythonReference (0.51s)
--- PASS: TestZVDBDemoCPMScriptedSession (0.71s)
--- PASS: TestZVDBDemoCPMScreenContent (0.91s)
--- PASS: TestZVDBDemoWideDataSetDoesNotOverflow (2.76s)
--- PASS: TestZVDBDemoTopKAtCapacityMatchesReference (0.53s)
--- PASS: TestZVDBDemoNarrowScreenLayout (0.82s)
--- PASS: TestZVDBDemoBelowMinimumScreenLayout (0.81s)
--- PASS: TestZVDBZXGlobalNamespace (1.08s)
--- PASS: TestZVDBZXKernelMatchesReference (0.82s)
--- PASS: TestZVDBZXScriptedSessionScreens (3.73s)
--- PASS: TestZXNarrowTextDemoScreens (6.68s)
--- PASS: TestZXNarrowTextTiming (2.47s)
ok      github.com/minz/minzc/pkg/pipeline	21.845s

$ go -C minzc test ./pkg/nanz/ -run TestShowcaseExecutableOracleCatalog -count=1
--- PASS: TestShowcaseExecutableOracleCatalog (5.50s)      # 211 paths

$ python3 scripts/assert_matrix.py --mz minzc/mz --glob 'examples/nanz/zvdb*.nanz' --no-controls -j 4
      "pass": 62, "fail": 0          # 62 units; the two new sources carry no asserts
```

The sharded `fast` profile (the three new tests registered; 595 entries,
inventory check clean):

```sh
$ (cd minzc && nice -n 19 taskset -c 4-15 python3 ../scripts/ci/run_stable_pipeline.py \
      --profile fast --output /tmp/zx4-fast2 --shards 4 --workers 4)
Pipeline inventory: 966 total, 595 selected (fast, job 1/1), 4 exhaustive process shards, 4 workers
$ grep -c '^--- FAIL' /tmp/zx4-fast2/shard-*.log
shard-1.log:0  shard-2.log:0  shard-3.log:0  shard-4.log:0
$ grep -hc -- '^--- PASS' /tmp/zx4-fast2/shard-*.log | paste -sd+ | bc     # 563 (+32 skipped = 595)
```

### 5.5 Screenshots and snapshot

`reports/zvdb-zx/screenshot-w5.png` (8642 B), `screenshot-w6.png` (7864 B),
`screenshot-w8.png` (6737 B), `screenshot-session.png` (8080 B, the scrolled
51-column screen after the scripted session) and `zvdb_demo_zx.sna` (49 179 B)
— the snapshot is committed, unlike the Z3 one, because the task names it as an
artifact for Alice; the `.a80`/`.bin`/`.sym` and the `.scr` dumps are gitignored
and regenerated by the commands in §4. Loading the `.sna` reproduces
`screenshot-w5.png`.

## 6. Findings

Everything below has a repro and its observed output in
`reports/zvdb-zx/repro/README.md`.

### 6.1 An imported module's variable home aliases a main-module global (silent wrong code)

`zx.narrow_font.nf_build_packed` has a local `top` (the mask-table pointer).
When the compiler gives it a memory home it emits the **bare** label `top`, so a
main-module global of the same name silently becomes that spill slot: the font
was squeezed with a mask byte read back from the wrong address and every glyph
on the screen was wrong. Same for `left_w` and `right_x`, locals of
`zx.text.split_vertical_i`, which would have opened the right pane at a garbage
column. No diagnostic at any stage — the program compiles, runs and draws.

`minzc/pkg/` is off limits for this task, so the demo namespaces every lowercase
global as `zvui_*`, and `TestZVDBZXGlobalNamespace` enforces it (plus a check
that no bare label of the main module is referenced from imported code). Renaming
one global back to `top` makes that test fail with
`global "top" is not in the zvui_ namespace`.

### 6.2 `win_open`/`win_attr` painted attribute rows `0..h-1`, ignoring `y` (engine bug, fixed)

`win_apply_rows` takes absolute character rows but `win_apply_attr` passed it
`0`, so opening a window at y = 3 with h = 4 coloured screen rows 0–3 instead of
3–6. Invisible to every Z3 caller (all its windows start at row 0) and to both
Z3 whole-screen tests, for the same reason. Fixed with one line in
`stdlib/zx/text.nanz`; both `TestZXNarrowText*` tests still pass, and the
 zvdb whole-screen proof is what caught it (173 attribute cells differed).

### 6.3 The Z3 demo's `j`/`k`/`q` keys never fire (demo bug, not fixed here)

`examples/nanz/zx_narrow_text_demo.nanz` compares `zx_getkey()` against the
lowercase codes 106/107/113, but the unshifted matrix reports 74/75/81
(`J`/`K`/`Q`) — measured in `repro/keytab-case/`. Only its `1`/`2`/`3` keys
work, and those are the only ones its tests press, so the Z3 report's claim that
`j`/`k` move the list is not true of the shipped build. This port uses the
uppercase codes and documents them; the Z3 file belongs to PR #297, so the
three-constant fix is left to its author (see §8).

### 6.4 Observations, recorded rather than acted on

* **Imported data is emitted once per importer** (known from Z3, measured here):
  importing `zvdb_demo_data` alongside `zvdb_kernel` costs this demo 2804 bytes
  — the difference between 4407 and 1613 bytes of free RAM. Hence `zvui_nrec`
  is derived from the kernel instead of read from the data module.
* **`win_invert_row` does not clamp its row.** `tx_attr_addr(wn_b0, wn_y+row)`
  with a row past the window writes above `$5AFF` into the system-variable area.
  My own `move_up` triggered exactly that before §6.5's fix (it passed a record
  index where a window row was expected); the engine rejected nothing. A clamp
  would match `win_goto`'s.
* **The compiler's `LD (top), A` store is only the low byte** of the pointer it
  is spilling, which is why the corruption was a *plausible-looking* font rather
  than an obvious one — the high byte happened to survive elsewhere.

### 6.5 Two bugs in this port's own first draft, found by the whole-screen test

Both are worth recording because a text-only check missed them:

* every pane row printed to the full pane width, so filling the bottom row
  scrolled the list up by one the moment it was drawn (fixed by printing one
  cell short on every row, which also keeps the labels off the divider gutter);
* `move_up` passed `zvui_cursor + 1` (a record index) to `win_invert_row` (which
  takes a window row) — invisible while `top == 0`, and once the list had
  scrolled it both left stale highlights and wrote past the attribute file.

## 7. What is still open

* **PR #297's `j`/`k`/`q` keys** (§6.3): three constants in
  `examples/nanz/zx_narrow_text_demo.nanz` plus its report. Left to that PR's
  author to avoid touching a file under review.
* **The unmangled variable homes** (§6.1) are a compiler defect; the namespace
  rule and the test are workarounds. A real fix would mangle a memory-homed
  local with its module prefix, like `_spill_<module>__<fn>_rN>` already is.
* **`win_invert_row` should clamp** its row argument (§6.4).
* A width switch still costs the 1.86 M T-state font rebuild Z3 measured; only
  one width's tables exist at a time, so pre-warming both would need another
  ~13 K — not available in 48 K alongside the data.
* The window layer costs ~1950 T per character on top of the 835 T blit (Z3
  §9.8); an asm print loop with cached cursor state would roughly halve a
  redraw, and with it every number in §3.
* The 5 px glyphs still have no inter-character gap, inherent to 51 columns.

STATUS: DONE
