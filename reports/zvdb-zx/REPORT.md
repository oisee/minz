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
| `minzc/pkg/pipeline/zvdb_demo_zx_test.go` | `TestZVDBZXKernelMatchesReference`, `TestZVDBZXScriptedSessionScreens` (14 whole-screen states + the timings), `TestZVDBZXGlobalNamespace` |
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
| `CS+6` / `CS+7` | the same two moves — the scan steps over the shift positions, so the emulator's and the 128K machine's cursor keys work |
| `1` / `2` / `3` | 51 / 42 / 32 columns, re-laid out (the whole screen is rebuilt: `text_set_width` clears it and closes every window) |
| `Q` | quit: blanks the screen, returns from `main`, and the compiler's ZX epilogue runs `DI / HALT` — the program halts, so re-run it rather than expecting a return to BASIC |

The letter keys are `J`, `K`, `Q`, not `j`, `k`, `q`: the unshifted keyboard
matrix reports uppercase ASCII, so a lowercase code never matches (§6.3). On a
real Spectrum these are the same physical keys.

`main` executes `DI` before anything else. The compiled bodies — this module's
own included — use IYH/IYL as a spill tier, and both `mzx --run` and a BASIC
`LOAD` + `RANDOMIZE USR 32768` start with interrupts enabled, so without it a
ROM IM1 service routine would run with a garbage IY. Nothing here needs an
interrupt and the program halts rather than returns, so it is never re-enabled.

### Redraw discipline

A cursor move that does not scroll repaints **no master text at all**: the old
and new highlighted rows are two `win_invert_row` calls (an attribute flip, the
only thing that changes), then the label line, the right pane and the status
line are rewritten. A move that scrolls down is one `win_scroll(0)` plus the one
new bottom row — the highlight is dropped first, because `win_scroll` moves the
attribute cells up with the pixels and would otherwise drag the inverted row up
the pane, one stale highlight per scroll. A move that scrolls up (which
`win_scroll` cannot express, it only moves text up) repaints the pane — the
slowest single move at ~45 frames. A move past the last record is clamped and
redraws nothing at all.

## 2. Memory map (48K Spectrum, image loaded at `$8000`)

```
$0000-$3FFF  ROM            only ever read (the font at $3D00)
$4000-$57FF  pixel bitmap   6144 bytes
$5800-$5AFF  attributes     768 bytes
$8000-$A692  code           demo + engine + kernel, 9.9 K
$A693-$A812  deletion masks NF5/NF6 top+bottom, 384 bytes
$A816-$AB15  nf_packed6     768
$AB16-$AE15  nf_packed5     768
$AE16-$DE15  nf_shift       pre-shifted glyph table, 12288
$DE16-$DF08  engine state   keep masks, tx_colpx, window arrays, mirror, rotbuf
$DF09-$DF50  zv_label_off   72
$DF51-$E5A1  zv_labels      1617
$E5A2-$EA01  zv_vecs        1120  (35 x 32)
$EA02-$EB01  zv_pop         256   (popcount LUT)
$EB03-$ED00  zv_top_val     510
$ED01-$EDFF  zv_top_idx     255
$EE09-$EE30  KEYTAB         40
$EE32-$EE5F  demo consts and zvui_* globals
$EE65-$EEE1  string literals + __mul8
$EEF4        image end (28405 bytes)
```

**Free RAM above the image: `$EEF5..$FFFF` = 4363 bytes.** Initial SP is `$FFFF`
(`mzx --run`) or `$FF58` (the Go test); the demo uses fewer than 64 bytes of
stack, and writes nothing between `$5B00` and `$7FFF`, so the whole 4.4 K is
genuinely free. It fits in 48 K with room to spare. A 255-record data module
would not: its vectors alone are 255 x 32 = 8160 bytes (against 4363 free), and
its labels would be roughly 255/35 x 1617 = 11.8 K.

Importing `zvdb_demo_data` directly as well as `zvdb_kernel` (as the CP/M demo
does) would ship the 2.8 K of labels and vectors **twice** — the compiler emits
an imported module's data once per importer. Measured on this demo: 31 155 bytes
(free RAM 1613) instead of 28 405 (free 4363). So the demo derives the record
count from the kernel's own asserted contract instead:
`zvui_nrec = zv_rank_count() + 1` after the first search (`zv_top_k` ranks every
record but the selection, which `zvdb_kernel.nanz` asserts: 34 for `zv_n` = 35).

## 3. Timings — keypress to screen settled, in frames

Measured by `TestZVDBZXScriptedSessionScreens`: it hashes the screen after every
frame on the in-process 48K machine and takes the keypress frame to the last
frame whose hash differs, stopping at the frame the next key is applied (so a
later key's redraw cannot be credited to this one). One frame = 69 888 T-states
at 3.5 MHz (50 Hz). The session uses 60-frame key spacing because a scroll-up
move takes ~45 frames and the demo has no key queue.

| action | frames | T-states | at 3.5 MHz |
|---|---|---|---|
| cursor move, no scroll, 51 columns | 28–30 | ~2.0 M | 0.58 s |
| cursor move, no scroll, 42 columns | 30 | ~2.1 M | 0.60 s |
| cursor move, no scroll, 32 columns | 24–25 | ~1.7 M | 0.49 s |
| cursor move that scrolls **down**, 51 columns | 32–34 | ~2.3 M | 0.65 s |
| cursor move that scrolls **up**, 51 columns (full pane repaint) | 45 | ~3.1 M | 0.90 s |
| cursor move that scrolls up, 42 columns | 39 | ~2.7 M | 0.78 s |
| cursor move past the last record (clamped) | 0 | — | nothing is redrawn |
| CS+6 cursor-key move, 51 columns | 30 | ~2.1 M | 0.60 s |
| width switch 51 → 42 (in the session) | 71 | 5.0 M | 1.42 s |
| width switch 51 → 42 (from a fresh run) | 66 | 4.6 M | 1.32 s |
| width switch 42 → 32 | 42 | 2.9 M | 0.84 s |
| width switch 32 → 51 | 77 | 5.4 M | 1.54 s |
| startup (font build + search + full draw) | 82 | 5.7 M | 1.64 s |

```
move @51:                [29 29 29 30 28 29 29 29 29 28 29 30 29 29 30 30 30 30 29] frames (max 30)
CS+6 move @51:           [30] frames (max 30)
move @51 (scrolled):     [34 33 33 32 33 33 33 34 34 34 33 34 34 34 34 33] frames (max 34)
clamp @51:               [0 0] frames (max 0)
move @51 (up):           [31 31 30 30 30 31 31 30 30 30 30 30 29 30 31 29 30 30] frames (max 31)
move @51 (scrolled up):  [45 45] frames (max 45)
switch to 42 columns:    [71 71] frames (max 71)
move @42:                [30 30 30] frames (max 30)
move @42 (scrolled up):  [39] frames (max 39)
switch to 32 columns:    [42] frames (max 42)
move @32:                [24 25 24] frames (max 25)
switch to 51 columns:    [77] frames (max 77)
```

`scripts/zvdb_zx_verify.py` measures the same quantities on independent mzx
runs and lands within a frame or two on the moves (29 for a plain move, 33 for
a scroll down) and 5–6 frames (~7 %) lower on the width switches (66/36/82).
The two press the key in different contexts — the CLI from the freshly drawn
startup screen, the test after the 34-move session — and the redraw's final
write therefore lands on a different frame boundary; both agree on the ordering
of every case.

For comparison, the CP/M demo needs 1.34 M T-states per cursor move (~0.34 s at
4 MHz) — the Spectrum pays about 1.5× more T-states because the right pane is
redrawn through the glyph blitter (~1925 T per character) rather than emitted as
1.4 KB of VT100 text. A width switch costs the 1.86 M T-state font rebuild Z3
measured plus the full redraw; the ranking is *not* recomputed on a switch (the
selection has not changed), which is worth ~12 frames a switch.

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

`TestZVDBZXScriptedSessionScreens` (in the `fast` profile, 4.7 s) drives the
demo through **start, 3 down, 1 up, 30 down (scroll), `2`, `3`, `1`** with
native key-matrix presses, then through everything the first draft left
untested: two moves past the last record (the bottom clamp), twenty moves back
up the list until it scrolls the *other* way (the branch `win_scroll` cannot
serve, since it only moves text up), a width switch while the list is scrolled
and one more scroll-up at the new width, and one `CAPS SHIFT + 6` cursor-key
move. After every step it diffs **all 6912 screen bytes** against an independent
model (`zvdbZXExpectedScreen`, a Go twin of `scripts/zvdb_zx_model.py`: the
geometry, the blit with keep masks, the divider rule, the wrap/scroll rule and
the attribute rectangles re-derived from the engine's documented contract, driven
by the reference ranking), and separately decodes both panes back to characters
and compares them cell for cell with what the reference predicts for the selected
record.

```sh
$ go -C minzc test ./pkg/pipeline/ -run TestZVDBZXScriptedSessionScreens -count=1 -v
=== RUN   TestZVDBZXScriptedSessionScreens/start_(51_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/3_down_(one_of_them_CS+6)
=== RUN   TestZVDBZXScriptedSessionScreens/1_up
=== RUN   TestZVDBZXScriptedSessionScreens/30_down_(scrolled)
=== RUN   TestZVDBZXScriptedSessionScreens/key_2_(42_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/3_up_at_42_columns
=== RUN   TestZVDBZXScriptedSessionScreens/key_3_(32_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/3_down_at_32_columns
=== RUN   TestZVDBZXScriptedSessionScreens/key_1_(51_columns)
=== RUN   TestZVDBZXScriptedSessionScreens/2_down_at_51_columns_(scrolled)
=== RUN   TestZVDBZXScriptedSessionScreens/2_down_past_the_end_(clamped)
=== RUN   TestZVDBZXScriptedSessionScreens/20_up_at_51_columns_(scrolled_up)
=== RUN   TestZVDBZXScriptedSessionScreens/key_2_while_scrolled
=== RUN   TestZVDBZXScriptedSessionScreens/1_up_at_42_columns_(scrolled_up)
--- PASS: TestZVDBZXScriptedSessionScreens (4.68s)
    ... all 14 subtests PASS, 0 bytes differ in any of them ...
```

Each subtest reports `0 bitmap bytes inside the panes, 0 outside, 0 attribute
cells` on success. The model also encodes two things a text-only check would
miss: the divider rule survives only in the rows no chrome cell covers (rows
2–21), and the gutter columns between the panes are never printed in.

Mutation checks — each defect reintroduced in turn is caught by exactly the
subtest that was added for it, and the tree is restored afterwards:

```
mutation                                       caught by
scroll-up branch does not reposition the pane  20 up (scrolled up), key 2 while scrolled, 1 up at 42 (scrolled up)
bottom clamp removed (moves wrap past the end) 2 down past the end (clamped), plus the three scroll-up states
global renamed back to `top`                   TestZVDBZXGlobalNamespace
```

The same run also asserts that the screen has stopped changing for the three
frames before every keypress: the demo has no key queue, so a press that lands
while a redraw is still writing would be lost, and a settle measurement alone
cannot show that (its search has to stop at the next key).

### 5.3 The independent CLI path

```sh
$ python3 scripts/zvdb_zx_verify.py
[w5] 5 px (51 columns): PASS
[w6] 6 px (42 columns): PASS
[w8] 8 px (32 columns): PASS
[session] 5 px after start, 3 down, 1 up, 30 down: PASS
[settle] move that does not scroll: 29 frames
[settle] move that scrolls the pane:  33 frames
[settle] startup (font build + search + full draw): 82 frames
[settle] switch to w6: 66 frames
[settle] switch to w8: 36 frames
[sna] .../reports/zvdb-zx/zvdb_demo_zx.sna
image 28405 bytes, $8000..$EEF4, free RAM to $FFFF: 4363 bytes
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
--- PASS: TestZVDBZXGlobalNamespace (1.29s)
--- PASS: TestZVDBZXKernelMatchesReference (0.95s)
--- PASS: TestZVDBZXScriptedSessionScreens (4.68s)
--- PASS: TestZXNarrowTextDemoScreens (6.65s)
--- PASS: TestZXNarrowTextTiming (2.67s)
ok      github.com/minz/minzc/pkg/pipeline	22.936s

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

### 6.1 An identifier inside an imported function resolves to the importer's same-named global (silent wrong code)

`nf_build_packed` in `stdlib/zx/narrow_font.nanz` declares its own `var top` and
assigns it `top = &NF5_TOP`. Normally that compiles to `LD IYH, B / LD IYL, C`
(the local lives in IY). But when the *importing* module declares a `global
top`, the same source line compiles to `LD A, C / LD (top), A` — the identifier
has resolved to the main module's global instead of the function's own local,
and has taken the global's type, which is why exactly one byte of the pointer
survives. The mask is read back through that byte, so every glyph on the screen
is squeezed wrong. No diagnostic at any stage: the program compiles, runs and
draws.

`left_w` and `right_x` in `zx.text.split_vertical_i` alias the same way, so the
demo's right pane would have opened at a garbage column too. `minzc/pkg/` is off
limits for this task, so the fix is in the demo: every lowercase global lives in
the `zvui_` namespace, and `TestZVDBZXGlobalNamespace` enforces it (plus a check
that no bare label of the main module is referenced from imported code again).
Renaming one global back to `top` makes that test fail with
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
  — the difference between 4363 and 1613 bytes of free RAM. Hence `zvui_nrec`
  is derived from the kernel instead of read from the data module.
* **`win_invert_row` does not clamp its row.** `tx_attr_addr(wn_b0, wn_y+row)`
  with a row past the window writes above `$5AFF` into the system-variable area.
  My own `move_up` triggered exactly that before §6.5's fix (it passed a record
  index where a window row was expected); the engine rejected nothing. A clamp
  would match `win_goto`'s.
* **The aliased identifier takes the global's type** (§6.1), so the store is one
  byte of a pointer: the corruption is a *plausible-looking* font rather than an
  obvious one, which is why it survived a visual smoke test for as long as it
  did.

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
  author to avoid touching a file under review. The same file's `zx_scan`
  returns 0 as soon as it sees a shift, so CS+digit does not work there either
  (this demo steps over the shift positions instead, §1).
* **The name-resolution defect** (§6.1) is a compiler bug; the `zvui_` namespace
  and the test are workarounds. A real fix would resolve an identifier inside an
  imported function against that function's own locals before the importing
  module's globals — and diagnose a shadowed name rather than silently changing
  the identifier's type.
* **`win_invert_row` should clamp** its row argument (§6.4).
* A width switch still costs the 1.86 M T-state font rebuild Z3 measured; only
  one width's tables exist at a time, so pre-warming both would need another
  ~13 K — not available in 48 K alongside the data.
* The window layer costs ~1950 T per character on top of the 835 T blit (Z3
  §9.8); an asm print loop with cached cursor state would roughly halve a
  redraw, and with it every number in §3.
* The 5 px glyphs still have no inter-character gap, inherent to 51 columns.

---

## Round 2 — the critic's findings, fixed

Critic round 1 on commit `858f4c6f2`. Everything below was re-measured after the
change, on the same in-process machine and the same mzx runs.

**1. The scroll-up branch and the bottom clamp were never exercised.** The
session now runs two moves past record 34 (clamped — the screen does not change
at all), twenty moves back up until `top` drops from 16 to 14, a width switch
while the list is scrolled, and one more scroll-up at 42 columns: fourteen
whole-screen states where there were ten. The key spacing went from 45 to 60
frames because a scroll-up move measures 45 frames and the demo has no key
queue. Both new paths are mutation-checked (§5.2): removing the clamp fails the
clamped state, and not repositioning the pane on a scroll-up fails all three
scroll-up states.

**2. The label-collision diagnosis was wrong.** It is not an unmangled variable
home: diffing the two builds shows `top = &NF5_TOP` in `nf_build_packed`
compiling to `LD IYH, B / LD IYL, C` normally and to `LD A, C / LD (top), A`
when the importer declares `global top`. The identifier resolves to the
importer's same-named global and takes its type, which is why one byte of the
pointer survives. §6.1, §6.4 and `repro/README.md` §1 now say so, the test
comment names the directory, and `control.nanz`'s header no longer calls itself
`collide.nanz`.

**3. `main` never disabled interrupts.** It now executes `DI` before anything
else (the header and §1 say why): the compiled bodies use IYH/IYL as a spill
tier, and both `mzx --run` and a BASIC `RANDOMIZE USR 32768` start with
interrupts enabled, so a ROM IM1 handler would have run with a garbage IY.

**4. `CAPS SHIFT + 6/7` did nothing**, because `zx_scan` returned the shift's
KEYTAB entry (0) as soon as it saw one. It now steps over the two shift
positions, so the emulator's and the 128K machine's cursor keys move the list;
one CS+6 press is in the session (the "3 down" step) and measures 30 frames,
the same as a plain move.

**5. Two redundant searches removed.** Startup ran `zv_top_k(0)` for the record
count and then `do_search()` for the same selection; a width switch re-ran
`do_search()` although the selection — and therefore the ranking — had not
changed. Both are gone: the width switches dropped from 84/55/90 to 71/42/77
frames and the startup from 93 to 87.

**6. The settle assertion could not fire**, because the search stopped at the
next keypress. It is replaced by a check that the screen has been stable for the
three frames before every keypress — the thing that actually matters, since a
press landing during a redraw is lost.

**7. Report accuracy.** The Go and CLI timings are now measured with the same
frame convention (§3 explains the residual ~5-frame difference on the width
switches instead of claiming the two agree); the memory map's last lines are
corrected (`$EE32-$EE5F` demo consts and globals, image end `$EEF4`, free RAM
4363); the muddled sentence about "4.3 K in the system-variable area" is gone.

**8. Small source fixes.** The KEYTAB comment now lists every half-row bit 0
first, as its rule claims (three of the eight rows were listed the other way
round); `zvdb_dump_zx.nanz` no longer assigns to a `let`.

STATUS: DONE
