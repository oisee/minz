# Task Z3 — ZX Spectrum narrow-font text engine + windows

Branch `feat/zx-narrow-text`, no pushes, everything committed locally. Compiler
(`minzc/pkg/`) untouched; every compiler finding has a repro under
`reports/zx-narrow-text/repro/` (see §7).

## 1. What was built

| file | what |
|---|---|
| `scripts/zx_narrow_font.py` | deletion-table generator: reads the ROM font at `$3D00`, derives per-glyph top/bottom column-deletion masks for 6 px and 5 px, writes the tables module and the ASCII previews, reports ink loss. `--check` proves byte-identical regeneration |
| `stdlib/zx/narrow_font_tables.nanz` | generated: `NF{5,6}_{TOP,BOT}`, 4 × 96 mask bytes (384 bytes) |
| `stdlib/zx/narrow_font.nanz` | run-time builder: `nf_squeeze` (asm), `nf_build_slots` (asm), `nf_build_packed`, `narrow_font_build(width)`, `narrow_font_base`, `narrow_font_shift_table` |
| `stdlib/zx/text.nanz` | glyph blitter (two cores) + the window API: `tx_blit`, `tx_blit_pair`, `tx_blit_rot`, `win_open/clear/scroll/goto/putch/print/print_padded/attr/invert_row/select/apply_attr`, `win_width/height/col/row`, `split_vertical`, `text_set_width`, `text_build_font`, `text_cols`, `tx_scr_addr`, `tx_attr_addr`, `tx_zero`, `tx_copy`, `tx_clear_screen`, `tx_vline` |
| `examples/nanz/zx_narrow_text_demo.nanz` | the 48K demo: two panes, numbered sentence list with an inverse-video cursor row, wrapping paragraph, keys `1`/`2`/`3`/`j`/`k`/`q` |
| `examples/nanz/zx_narrow_text_bench.nanz` | the T-state bench driven by the Go test (phases for build / aligned blit / straddling blit / goto / putch / full row / width switch) |
| `scripts/zx_narrow_text_ocr.py` | screen → text decoder: picks W-pixel cells out of the interleaved bitmap and matches them against the packed fonts derived from the shipped tables |
| `scripts/zx_narrow_text_verify.py` | CLI end-to-end proof: builds the demo, runs mzx at 51/42/32 columns with native key-matrix scripts, dumps `.scr`, OCRs both panes, checks text + attributes + divider, writes screenshots and a `.sna` |
| `minzc/pkg/pipeline/zx_narrow_text_machine_test.go` | `TestZXNarrowTextDemoAllWidths` (compile, run on an in-process 48K machine, decode, assert) and `TestZXNarrowTextTiming` (exact T-states, both cores, three widths) |
| `scripts/ci/pipeline_profiles.json` | both tests registered in the `fast` profile |
| `reports/zx-narrow-text/` | `preview-w5.txt`, `preview-w6.txt`, `screenshot-w5/6/8.png`, `repro/` (+ a gitignored `.bin`/`.a80`/`.sna`/key scripts) |

## 2. Memory map (48K Spectrum, demo image loaded at `$8000`)

```
$0000-$3FFF  ROM            only ever read (the font at $3D00)
$4000-$57FF  pixel bitmap   6144 bytes
$5800-$5AFF  attributes     768 bytes
$5B00-$5B03  bench config   the T-state harness pokes (n, phase, width, core) here
$8000-$8F40  code           engine + demo, 4.6 K
$8F41-$9210  tables module  NF5/NF6 top+bottom masks, 384 bytes
$9211-$9510  nf_packed6     768
$9511-$980F  nf_packed5     768
$9811-$C810  nf_shift       pre-shifted glyph table, 12288 (8 shifts x 96 glyphs x 16)
$C81A-$C8DE  engine state   masks, colpx, window arrays, rotbuf, ...
$C8E4-$C905  KEYTAB         40-byte key matrix -> ASCII table
     ...$CE15 demo strings  (image is 19989 bytes, ends at $CE15)
$FF58        initial SP     (< 64 bytes of stack used); mzx --run defaults to SP=FFFF
```

Interrupts are off (`--run` passes `DI`): the blitters clobber `IY`, so a ROM IM1
service routine would fault. Documented in the demo header.

## 3. Glyph loss

`python3 scripts/zx_narrow_font.py --loss`

```
W=6: 3/96 glyphs lost ink, 1/96 lost a non-duplicate ink column
W=5: 40/96 glyphs lost ink, 6/96 lost a non-duplicate ink column
```

The task expected "roughly 3-4/96 at 6 px and about 40/96 at 5 px" — exact for
the literal reading ("fewer ink pixels than the 8 px glyph"). The second number
is the stricter reading (an inked column was dropped that had no surviving twin,
i.e. the shape really changed). At 6 px the ROM font is already 6 px wide with a
blank column 0 and 7, so 93 glyphs lose nothing at all. Previews:
`reports/zx-narrow-text/preview-w6.txt` (96 glyphs, before/after, with `~` = a
duplicated column dropped and `v` = real ink lost) and `preview-w5.txt`.

Hand overrides: the `OVERRIDES` table in `scripts/zx_narrow_font.py` is empty on
purpose; the automatic tables judged acceptable at both widths. Re-running the
generator leaves all three outputs byte-identical (`--check`).

## 4. T-states

Measured by `TestZXNarrowTextTiming` on the in-process 48K machine (real ROM, no
contention), differential: the bench runs each phase `N=40` times and then 0
times, and the T-state difference is divided by 40. `RunFrameFast` quantises
`AbsoluteTStates` to whole 69888 T frames, so the bench is stepped opcode by
opcode to its `HALT` instead.

| width | cols | `tx_blit` aligned (s=0) | `tx_blit` straddling | `win_goto` | `win_putch` **core 0** | `win_putch` core 1 | full row print | T/glyph in a row |
|---|---|---|---|---|---|---|---|---|
| 5 | 51 | 829 | 1093 | 397 | **2173** | 3025 | **120013** | 2353 |
| 6 | 42 | 829 | 1093 | 397 | **2173** | 3025 | **99129** | 2360 |
| 8 | 32 | 829 | 822 | 397 | **2173** | 3025 | **71438** | 2232 |

```
width=5 cols=51 core=0: tx_blit(aligned)=829T tx_blit(straddling)=1093T win_goto=397T win_putch=2173T row=120013T (2353 T/glyph)
width=5 cols=51 core=1: tx_blit(aligned)=829T tx_blit(straddling)=1093T win_goto=397T win_putch=3025T row=204745T (4015 T/glyph)
width=6 cols=42 core=0: tx_blit(aligned)=829T tx_blit(straddling)=1093T win_goto=397T win_putch=2173T row=99129T  (2360 T/glyph)
width=6 cols=42 core=1: tx_blit(aligned)=829T tx_blit(straddling)=1093T win_goto=397T win_putch=3025T row=164689T (3921 T/glyph)
width=8 cols=32 core=0: tx_blit(aligned)=829T tx_blit(straddling)=822T  win_goto=397T win_putch=2173T row=71438T  (2232 T/glyph)
width=8 cols=32 core=1: tx_blit(aligned)=829T tx_blit(straddling)=822T  win_goto=397T win_putch=3025T row=98702T  (3084 T/glyph)
```

**Pre-shifted table vs rotate-by-shift-count.** Both cores are compiled in and
switchable at run time through `tx_core`; core 0 (the shipped default) wins at
every width: 2173 vs 3025 T per `win_putch` (1.39x) and 120013 vs 204745 T per
51-column row (1.71x). `tx_blit` alone costs 829 T aligned and 1093 T
straddling — the straddle path is +264 T, exactly the eight extra
`LD/AND/OR/LD` pairs. The dominant cost is *not* the blitter, though: the window
layer around it (cursor lookup in `wn_col[id]`, `tx_colpx`, the width/core
dispatch, the wrap check) is 2173 - 829 = 1344 T per character, i.e. ~60 % of a
row print. An asm fast path that keeps the cursor state in registers across one
`win_print` is the obvious next optimisation and would roughly halve a redraw.

A full demo screen redraw is 19 rows x 51 chars = 19 x 120013 = **2,280,247 T**
(0.65 s at 3.5 MHz, 33 frames); a single row is 120013 T = 1.7 frames.

**Font build time** (`narrow_font_build`, i.e. the packed font plus all eight
shift slots):

| width | build | `text_set_width` (build + geometry) |
|---|---|---|
| 5 | 1 595 718 T (0.46 s, 23 frames) | 1 600 719 T |
| 6 | 1 584 185 T | 1 589 174 T |
| 8 | 43 641 T (ROM font, slot 0 only) | 48 564 T |

The packed-font pass (plain Nanz, 768 `nf_squeeze` calls through a doubly nested
loop) costs ~1.2 M of those T-states; the two asm streaming passes that build
the 12288-byte shift table cost ~0.4 M. A width switch therefore costs about
0.46 s — acceptable for a mode key, and the obvious optimisation (asm the
packed-font loop like `nf_build_slots`) is recorded rather than done.

## 5. Verification

Headless key input **is** available in `mzx` (`--key-script`, native
keyboard-matrix transitions), so no compiled-in script mode was needed.

### 5.1 Generator determinism

```sh
$ python3 scripts/zx_narrow_font.py            # writes the 3 files
$ python3 scripts/zx_narrow_font.py --check
OK: 3 generated files up to date
$ python3 scripts/zx_narrow_font.py --loss
W=6: 3/96 glyphs lost ink, 1/96 lost a non-duplicate ink column
W=5: 40/96 glyphs lost ink, 6/96 lost a non-duplicate ink column
```

### 5.2 Run-time font builder equals the generator's model

A throwaway program built the font on the emulator and copied `nf_packed5` and
the first two shift slots into screen RAM; the Python model matched byte for
byte (768 + 3072 bytes) — this is how the generator and the Z80 builder were
reconciled during development.

### 5.3 CLI end-to-end proof (mzx + Python OCR)

```sh
$ python3 scripts/zx_narrow_text_verify.py
[w5] 51 columns, frame_0500.scr: PASS
[w6] 42 columns, frame_1000.scr: PASS
[w8] 32 columns, frame_1000.scr: PASS
[sna] /home/alice/minz-workers/zx3/repo/reports/zx-narrow-text/zx_narrow_text_demo.sna
ALL PASS
```

For each width this builds the demo, runs it under mzx (pressing `2` or `3` at
frame 400 through `--key-script`), dumps the 6912-byte screen, decodes both
panes cell for cell and asserts them against the expected text — including the
right pane's wrap and the window scroll it causes — plus the attribute
rectangles and every pixel of the split divider. Decoding one pane:

```sh
$ python3 scripts/zx_narrow_text_ocr.py --scr <(last dump) --width 5 --x-px 0 --cols 24
00 The forest is calm at
01 Birds chirp in the co
...
23 Rain on the roof is c
```

### 5.4 Go machine test (CI, `fast` profile)

```sh
$ cd minzc && go test ./pkg/pipeline/ -run TestZXNarrowText -v -count=1
=== RUN   TestZXNarrowTextDemoAllWidths
=== RUN   TestZXNarrowTextDemoAllWidths/5px
=== RUN   TestZXNarrowTextDemoAllWidths/6px
=== RUN   TestZXNarrowTextDemoAllWidths/8px
--- PASS: TestZXNarrowTextDemoAllWidths (2.90s)
    --- PASS: TestZXNarrowTextDemoAllWidths/5px (0.56s)
    --- PASS: TestZXNarrowTextDemoAllWidths/6px (0.93s)
    --- PASS: TestZXNarrowTextDemoAllWidths/8px (0.94s)
=== RUN   TestZXNarrowTextTiming
--- PASS: TestZXNarrowTextTiming (1.82s)      # logs reproduced in §4
PASS
ok      github.com/minz/minzc/pkg/pipeline	4.734s
```

The test compiles the demo through the production pipeline, runs it on a 48K
machine with the real ROM, switches widths with `machine.KeyPress`, and decodes
with a Go twin of the OCR that rebuilds the packed fonts from
`stdlib/zx/narrow_font_tables.nanz` — so both decoders are driven by the bytes
that actually ship. Cost: ~4.7 s for both tests, inside the `fast` budget.

### 5.5 Screenshots and snapshot

`reports/zx-narrow-text/screenshot-w5.png` (8976 B), `screenshot-w6.png`
(8636 B), `screenshot-w8.png` (7432 B) and `zx_narrow_text_demo.sna` (49179 B,
gitignored). Exact commands:

```sh
# build
minzc/mz examples/nanz/zx_narrow_text_demo.nanz -o reports/zx-narrow-text/zx_narrow_text_demo.a80
minzc/mza reports/zx-narrow-text/zx_narrow_text_demo.a80 -o reports/zx-narrow-text/zx_narrow_text_demo.bin
# 51 columns (startup state) + snapshot + screenshot
minzc/mzx-headless --model 48k \
  --run reports/zx-narrow-text/zx_narrow_text_demo.bin@8000 \
  --frames 500 --screenshot reports/zx-narrow-text/screenshot-w5.png \
  --save-snapshot reports/zx-narrow-text/zx_narrow_text_demo.sna
# 42 / 32 columns: press '2' / '3' (matrix row 3, bit 1 / bit 2) at frame 400
echo '[{"frame":400,"row":3,"bit":1,"down":true},{"frame":420,"row":3,"bit":1,"down":false}]' > k.json
minzc/mzx-headless --model 48k --run ...demo.bin@8000 --frames 1000 \
  --key-script k.json --screenshot reports/zx-narrow-text/screenshot-w6.png
# open the snapshot
minzc/mzx-headless --model 48k --snapshot reports/zx-narrow-text/zx_narrow_text_demo.sna --frames 5 --screenshot /tmp/check.png
# (the graphical build: minzc/mzx --model 48k --snapshot .../zx_narrow_text_demo.sna)
```

Loading the `.sna` and taking a screenshot reproduces `screenshot-w5.png`
byte-identically.

## 6. API summary (for the zvdb port, task Z4)

Same signatures at every width; positions are character cells of the *current*
font, so a port re-lays out rather than re-deriving addresses.

```text
text_set_width(w: u8)            // 5 | 6 | 8. Builds the font, sets tx_cols
                                 //   (51/42/32), refills tx_colpx/tx_masks and,
                                 //   if split_vertical was used, re-lays out the
                                 //   split. Call before any window work.
text_build_font(w: u8)           // rebuild only, no layout change
text_cols() -> u8                // 51 | 42 | 32
win_open(id, x, y, w, h)         // id 0..3; also paints the window's attribute
                                 //   over the attribute columns it touches and
                                 //   makes it current
win_select(id)                   // which window the calls below write to
win_clear(id)                    // blank pixels, repaint attributes, home cursor
win_goto(col, row)               // clamped into the window
win_putch(ch)                    // 13/10 = newline; wraps at the window edge
win_print(str)                   // NUL-terminated
win_print_padded(str, width)     // pads or truncates to exactly `width` cells
win_attr(ink, paper, bright)     // ink/paper 0-7, bright 0/1
win_invert_row(row)              // swaps ink/paper bits = inverse video row
win_width/height/col/row() -> u8
split_vertical(left_w, gap)      // gap 0 = 1 px rule, 1 = one attribute column;
                                 //   right pane snapped to an 8 px boundary
tx_core = 0 | 1                  // 0 pre-shifted table (default), 1 rotate
```

Mapping from the CP/M demo's `tui.render` layer: `tui_goto`→`win_goto`,
`tui_color`→`win_attr`, `tui_putch`→`win_putch`, `tui_puts`→`win_print`,
`puts_padded`→`win_print_padded`, `tui_clear`→`win_clear` + `tx_clear_screen`,
`tui_read_key`→`zx_getkey` in the demo (port `$FE` matrix, one event per
physical press). Layout: at 51 columns use `split_vertical(24, 1)` — the left
pane is 24 cells/120 px (8-px aligned), the divider owns attribute column 15 and
the right pane starts at character 26 (130 px) with 25 columns; at 42 columns
`split_vertical(16, 1)` gives 16 + 24 columns; at 32 columns `split_vertical(16,
1)` gives 16 + 15.

Two rules to know before porting:
* **Colour granularity.** One attribute cell is 8x8 px, i.e. 1.6 characters at
  5 px and 1.33 at 6 px. `win_open`/`win_attr` paint the *whole* attribute
  columns the window's ink can touch, rounded outward, last writer wins.
  `split_vertical` absorbs the rounding in the divider gutter so the two panes
  never share an attribute cell. Within a pane, adjacent characters share an
  attribute cell, so a per-character colour change is impossible — the zvdb
  highlight must stay per row (`win_invert_row`), which it already is.
* **Byte granularity.** `win_clear`/`win_scroll` operate on the rounded-out
  byte range `[x_px>>3, (x_px+w_px-1)>>3]`. With `split_vertical` both panes are
  byte aligned, so nothing leaks; a hand-placed window at a non-aligned x would
  clear/scroll whole bytes it shares with its neighbour.

## 7. Compiler findings

Five, all with repros and outputs in `reports/zx-narrow-text/repro/README.md`:

1. **A hand-written `asm z80` operand cannot name a module global portably**
   (blocker, worked around). The compiler prefixes every symbol of an imported
   module with the mangled name of the *importing* module, and leaves asm
   operands untouched, so `LD HL, leaf__table` inside `leaf.nanz` dangles the
   moment `mid.nanz` imports `leaf`. Workaround: pass the address as a parameter
   (`nf_build_slots` takes the table base in `DE`) or cache it in the importing
   module's own global (`tx_font`/`tx_shift`).
2. **Imported code is duplicated per importer** (blocker for the 48K budget,
   worked around). The bench importing both `zx.text` and `zx.narrow_font`
   emitted the font builder twice and grew from 18261 to 33013 bytes, which no
   longer fits below `$FFFF`. Fixed by importing only `zx.text` and adding
   `text_build_font()`.
3. **`SRL IYL` / `RR IYH` fail Z80-VALIDATE** although the emulator executes
   them. Worked around by rotating in `BC`.
4. **Array sizes must be literals** (`[u8; NWIN]` is a parse error) — same
   family as the known const-expression bug; every window array spells `[u8; 4]`.
5. **`//` comments inside `asm z80` blocks** are re-emitted as instruction lines
   (and a standalone `;` comment line loses its marker). All asm comments in
   `stdlib/zx` are trailing `;` comments.

No miscompilation was found: the Z80 output matched the Python font model byte
for byte, and all three widths decode to exactly the expected text.

## 8. Honest limitations

* The window layer costs 1344 T per character on top of the 829 T blit — the
  single biggest remaining win (an asm print loop with cached cursor state).
* A width switch rebuilds the whole font (0.46 s). Only one width's tables exist
  at a time; pre-warming both would need another 13 K.
* 5 px glyphs have no inter-character gap (51 columns at 5 px is 255 px), so
  letters abut. That is inherent to the width; `OVERRIDES` in the generator is
  the escape hatch, and the previews exist to judge it.
* The engine clobbers `IY`, so it must run with interrupts disabled (or with
  `IY` restored around `EI`).

STATUS: DONE
