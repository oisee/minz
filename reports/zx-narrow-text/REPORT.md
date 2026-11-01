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

> Superseded by the Round 2 map in §9.4 — round 2 added the IY wrappers, the
> window keep-mask state and the single 48-byte mask table, and the image grew
> from 19989 to 22419 bytes.

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

> Superseded by the Round 2 measurements in §9.5: the IY-preserving entry points
> and the new guards cost about 18 % more per printed row.

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
* ~~The engine clobbers `IY`, so it must run with interrupts disabled~~ — fixed
  in Round 2: every public call now preserves IY (§9.3).

---

## 9. Round 2 — the critic's findings, fixed

Critic report: `CRITIC-1.md` (verdict GO-WITH-FIXES). Everything below was
re-measured the way the critic measured it: probes compiled and run headless,
state read back out of `.sna` snapshots, and the whole 6912-byte screen diffed
against an independent model.

### 9.1 The blockers

**B1 — `tx_clear_screen` cleared 8 of 192 pixel lines and 765 of 768 attribute
bytes.** Fixed: it now walks all 24 character rows and adds the three attribute
bytes the 255-byte runs missed (`tx_zero(0x5AFD, 3)`). The critic's `$AA` probe
(`crit/probe/p_clear.nanz`, rerun here) now reports **0 of 6144 bitmap and 0 of
768 attribute bytes uncleared** (was 5888 and 3).

**B2 — `win_scroll`/`win_clear` wrote outside a window that is not byte
aligned.** Fixed properly, not by rejecting non-aligned windows: every window
now carries the keep masks `wn_ke0`/`wn_ke1` for the bits of its first and last
screen byte that it does not own, and `win_clear_line`/`win_copy_line` use them
so only the window's own pixels are ever rewritten. Attributes are split the
same way: `wn_ab0..wn_ab1` are the attribute cells whose 8 pixels are all inside
the window, and a scroll moves only those. The critic's probe
(`crit/probe/p_scroll.nanz`, rerun) gives

```
changed pixels inside B           : 0        (B row 0 still reads "aaaa")
changed attribute cells in B      : 0        (attr col 6 rows 0..9 all 0x0f)
A row 0 after three scrolls       : "DDDD"   (the scroll itself still works)
```

(was 71 pixels and 3 attribute cells). `win_open`/`win_attr` still paint the
rounded-out attribute range — that is the documented colour rule, and a window
has to be able to colour every cell its ink can reach — but a scroll no longer
recruits a shared cell.

**B3 — the right pane was not snapped to an 8-px boundary.** What the layout now
guarantees is stated honestly in the module header and measured: at 5 px the
byte-aligned origins reachable on the cell grid are 40/80/120/160 px, so snapping
would burn up to six columns of screen. Instead the two panes' *byte ranges* are
disjoint (left ends at byte 14/11/15, right starts at 16/13/17 at 5/6/8 px) and,
since B2 made every pixel operation masked, neither pane can write the gutter at
all — the divider rule is the only thing that does. The whole-screen model test
(§9.6) asserts exactly that, including the gutter columns.

**B4 — the divider was invisible at 51 columns.** `split_vertical` now records
the divider's attribute cells and paints them with the left pane's attribute
(`tx_paint_divider`), and `win_apply_attr(0, …)` repaints them whenever the left
pane's colour changes. The model expects `0x68` there and the emulator agrees.

### 9.2 The should-fix items

| # | fix |
|---|---|
| 1 | `win_open` rejects `x >= tx_cols` or `y >= 24` (the window prints nothing) and clamps `h` as well as `w`. Probe: `win_open(0,0,0,51,30)` now reports `h = 24` and writes nothing past `$5AFF`; `win_open(1,60,0,5,4)` reports `w = 0` instead of 247 |
| 2 | `win_scroll` returns immediately for `h < 2` |
| 3 | follows: a 1×1 window now shows its last character (probe prints `QRST` into one and the cell reads `T`) |
| 4 | `win_goto`/`win_putch` are no-ops on a zero-width or zero-height window, so `tx_colpx` is never read out of range |
| 5 | `text_set_width` no longer re-splits with the previous width's `sp_left_w`: it closes every window, clears the screen and leaves the layout to the caller (documented in the header). This also removes the wasted intermediate redraw the critic measured |
| 6 | `TestZXNarrowTextTiming` now asserts every measured value against the numbers in §9.5 within ±3 %; setting one expectation to a wrong value makes it fail (`win_goto: 631 T, want 400 +/- 3 %`) |
| 7 | the whole-screen check is §9.6 |
| 8 | `NF_SLOT_BYTES` and `nf_slots` deleted; `TX_NWIN` kept with the explanation (array sizes must be literals) |
| 9 | the duplicated attribute-paint loop is `win_apply_rows(id, r0, rows)`, used by `win_apply_attr`, `win_scroll` and a new `win_apply_own` |
| 10 | `zx_narrow_text_ocr.py` now exits 1 on unreadable or ambiguous cells (and says how many): decoding a 5 px screen as 6 px prints `7 unreadable cells` and exits 1 |
| 11 | memory map corrected in §9.4 |

Nice-to-haves taken: the three 16-byte mask tables plus their run-time copy are
now one 48-byte table selected by a pointer (`tx_masks_w`/`tx_masktab`, so the
blit loads `LD IX,(tx_masktab)` — the measured cost is the +6 T on `tx_blit`);
`tx_blit_pair` carries a comment with its per-row costs (~44 T single-byte,
~74 T straddling, which is the measured +264 T per glyph); `split_vertical`
clamps `left_w` so the gutter still fits on the line — the critic's
`split_vertical(51,1)` case now yields a 49-cell left pane and an empty right
pane instead of two identical overlapping windows.

Not taken: emitting `tx_masks` from arithmetic at run time needs variable shifts
the compiler would have to loop, and the table is 48 bytes.

### 9.3 IY — the critic's recommendation, implemented and measured

Every public entry point is now a thin wrapper that saves IY, calls a
parameterless `*_i` implementation and restores it: `win_open`, `win_select`,
`win_width/height/col/row`, `text_cols`, `win_clear`, `win_scroll`, `win_goto`,
`win_putch`, `win_print`, `win_print_padded`, `win_attr`, `win_invert_row`,
`split_vertical`, `text_set_width`.

Two things made it a wrapper rather than a `PUSH IY` in each function:

* the compiled bodies — not just the asm blitters — use IYH/IYL as a spill tier,
  so the save has to run before any generated code;
* arguments cannot be passed through an asm body, because the compiler picks a
  different calling convention per function and per call site (for a
  `(pointer, byte)` pair the declared ABI says the byte arrives in A while the
  call site puts it in C — that inconsistency is reproducible and is now
  compiler finding 6 below). The wrappers therefore park their arguments in
  `tx_arg*` slots, except `win_putch` and `win_goto`, where a single-register
  ABI is reliable and worth the ~500 T.

Measured cost (see §9.5): `win_goto` 397 → 631 T, `win_putch` (which includes a
`win_goto`) 2173 → 2881 T, a 51-column row 120013 → 141829 T (+18 %). The
critic's estimate of +29 T per call holds for the trampoline itself
(`PUSH IY` + `CALL` + `POP IY` = 46 T against 17 T); the rest of the regression
is the argument slots and the new zero-size guards, which cost about 500 T per
`win_putch` because a dynamically indexed window-array read is ~250 T in this
compiler.

Verified with the critic's method, extended to every entry point: IY is set to
`$5C3A` and checked inside the same asm block, with no generated code in
between (an ordinary Nanz caller can clobber IY between calls — that is the
caller's code, not the engine's). Result for all 17 public calls:
**IY preserved**. The demo still runs with DI because its *own* compiled code
clobbers IY; that is now stated in the demo header.

### 9.4 Memory map after round 2 (image 22419 bytes, `$8000..$D793`)

```
$0000-$3FFF  ROM            only ever read (the font at $3D00)
$4000-$57FF  pixel bitmap   6144 bytes
$5800-$5AFF  attributes     768 bytes
$8000-$99F1  code           engine + demo, 6.0 K
$99F2-$9B71  deletion masks NF5/NF6 top+bottom, 384 bytes
$9B75-$9E74  nf_packed6     768
$9E75-$A174  nf_packed5     768
$A175-$D174  nf_shift       pre-shifted glyph table, 12288
$D17D-$D261  engine state   masks, colpx, window arrays (now incl. wn_ke0/ke1,
                             wn_ab0/ab1 and the tx_arg slots), rotbuf
$D262-$D289  KEYTAB         40 bytes
     ...$D793 demo strings
$FF58        initial SP     (< 64 bytes used)
```

Free RAM above the image: `$D793..$FFFF` = 10318 bytes, so the zvdb port has
roughly 10 K for its data and kernel. One caveat the round-1 report glossed
over, now stated plainly: **core 1 (rotate) does not save the 12 KB table.**
`nf_shift` is emitted unconditionally and `narrow_font_build` always fills all
eight slots for widths 5 and 6, so `tx_core = 1` costs the same memory and 1.6×
the time. If 48 K ever gets tight the lever is making `nf_shift` conditional,
not switching core.

### 9.5 T-states after round 2 (same bench, same differential method)

| width | cols | `tx_blit` aligned | `tx_blit` straddling | `win_goto` | `win_putch` core 0 | core 1 | full row | T/glyph |
|---|---|---|---|---|---|---|---|---|
| 5 | 51 | 835 | 1099 | 631 | **2881** | 3733 | **141829** | 2781 |
| 6 | 42 | 835 | 1099 | 631 | **2881** | 3733 | **117183** | 2790 |
| 8 | 32 | 835 | 828 | 631 | **2881** | 3733 | **85312** | 2666 |

```
width=5 cols=51 core=0: tx_blit(aligned)=835T tx_blit(straddling)=1099T win_goto=631T win_putch=2881T row=141829T (2781 T/glyph)
width=5 cols=51 core=1: tx_blit(aligned)=835T tx_blit(straddling)=1099T win_goto=631T win_putch=3733T row=226561T (4442 T/glyph)
width=6 cols=42 core=0: tx_blit(aligned)=835T tx_blit(straddling)=1099T win_goto=631T win_putch=2881T row=117183T (2790 T/glyph)
width=6 cols=42 core=1: tx_blit(aligned)=835T tx_blit(straddling)=1099T win_goto=631T win_putch=3733T row=182743T (4351 T/glyph)
width=8 cols=32 core=0: tx_blit(aligned)=835T tx_blit(straddling)=828T  win_goto=631T win_putch=2881T row=85312T  (2666 T/glyph)
width=8 cols=32 core=1: tx_blit(aligned)=835T tx_blit(straddling)=828T  win_goto=631T win_putch=3733T row=112576T (3518 T/glyph)
width=5: narrow_font_build=1595698T text_set_width=1857070T
width=6: narrow_font_build=1584165T text_set_width=1845490T
width=8: narrow_font_build=43621T   text_set_width=304948T
```

The pre-shifted core still wins at every width (2881 vs 3733 T per character,
141829 vs 226561 T per row — now 1.6×, was 1.4×). A full demo redraw is
19 × 141829 = **2 694 751 T** (0.77 s at 3.5 MHz, 39 frames); round 1 was
2 280 247 T. `text_set_width` grew from 1.60 M to 1.86 M T because it now
clears the whole screen file (192 `tx_zero` calls, ~250 k T) — that is the B1
fix, not a regression.

### 9.6 The whole-screen check

New: `scripts/zx_narrow_text_model.py` renders the expected 6912 bytes from
first principles (geometry, blit with keep masks, wrap/scroll, attribute rule,
divider), and both verifiers diff the emulator's screen against it — all 6144
bitmap bytes and all 768 attribute bytes, with anything *outside* the windows
reported separately.

* Go: `TestZXNarrowTextDemoScreens` (in the `fast` profile) runs the demo on an
  in-process 48K machine for the startup screen at 51/42/32 columns and for all
  six transitions 5→6, 5→8, 6→8, 8→6, 6→5, 8→5, pressing keys through the
  native matrix. **All nine cases: 0 bytes differ.**
* CLI: `python3 scripts/zx_narrow_text_verify.py` does the same through
  `mzx-headless` and regenerates the screenshots:

```
[w5-startup] 5 px: PASS      [6to8] 8 px: PASS
[w6-startup] 6 px: PASS      [8to6] 6 px: PASS
[w8-startup] 8 px: PASS      [6to5] 5 px: PASS
[5to6] 6 px: PASS            [8to5] 5 px: PASS
[5to8] 8 px: PASS            ALL PASS
```

**It fails on the round-1 code.** With `stdlib/zx/text.nanz` and
`stdlib/zx/narrow_font.nanz` reverted to commit `6048acbc9` and nothing else
changed, the same test reports:

```
--- FAIL: TestZXNarrowTextDemoScreens/w5-startup  0 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/w6-startup  118 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/w8-startup   90 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/5to6        118 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/5to8         90 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/6to8        126 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/8to6        103 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/6to5        110 outside, 24 attribute cells
--- FAIL: TestZXNarrowTextDemoScreens/8to5          0 outside, 24 attribute cells
```

(the counts are differing *bytes*, so they are smaller than the critic's ink
*pixel* counts of 312/137/256/199/227; the 24 attribute cells are the gutter
column the round-1 code never painted). Restoring the round-2 stdlib makes all
nine pass again.

Screenshots regenerated: `screenshot-w5.png` (8958 B), `screenshot-w6.png`
(8127 B), `screenshot-w8.png` (7095 B) — the 42-column one no longer carries the
column of stale 51-column glyphs at screen x=96..103.

### 9.7 "Compiler finding 6" — RETRACTED in round 3

Round 2 claimed that for `fun f(s: ^u8, w: u8)` with an asm body "the emitted ABI
comment says `w: u8 = A` and a probe reads `w` in **C**". That was a measurement
artefact, not a compiler bug, and the claim is withdrawn.

What my probe actually did wrong: it stored B, C, D, E and HL at the asm entry
but never A, because storing A would have needed a `LD A,r` first. The emitted
caller is

```
LD HL, _mir2_str_0
LD C, 17          ; the caller's own internal convention
LD A, C           ; shuffle into the register the callee declares
CALL f
```

so at the entry *both* A and C hold 17; sampling A (round 3, same probe) reads
`A = 0x11` exactly as the comment says. The critic's independent matrix — six
shapes, `mzd --regs --verify-abi` reporting 68/68 functions matched, and nine
differently-shaped Nanz→Nanz call sites all correct — agrees.

What *is* true, and is the real reason the zx.text wrappers park their arguments
in `tx_arg*` slots instead of passing registers through: **the calling
convention is chosen per function, from the callee's own body**, so a
hand-written `CALL` into compiled Nanz code cannot know a register convention in
advance. In this tree `win_print_padded_i` takes its string *in memory*
(`str: ptr = mem, width: u8 = C`) because the pointer is spilled in its loop,
while a sibling function with the same signature takes it in HL. That is a
documentation caution, not a defect: check the emitted `; fun … (p: t = reg)`
comment for the exact build before writing such a call. (Round-1 finding 5 — an
imported module's code is emitted once per importer — did bite again here: the
bench importing `zx.narrow_font` directly grew from 18.3 K to 33.1 K, which is
why `text_build_font` exists.)

### 9.8 What is still open

* The window layer costs ~1950 T per character on top of the 835 T blit (round 1:
  ~1344). An asm fast path that keeps the cursor state in registers across one
  `win_print` is still the single biggest win, and would more than pay back the
  round-2 regression.
* A width switch costs 1.86 M T (0.53 s), of which ~250 k T is the screen clear.
* 5 px glyphs still have no inter-character gap — inherent to 51 columns.
* ~~`win_print_padded`'s string pointer still goes through a slot rather than
  HL~~ — moot: "finding 6" is retracted in §9.7, and the slots stay for the
  documented reason.

---

## 10. Round 3 — the critic's second report, fixed

Critic report: `CRITIC-2.md` (verdict GO-WITH-FIXES). Everything below was
re-measured the same way the critic measured it.

### 10.1 The CI manifest (the blocker)

`TestZXNarrowTextDemoAllWidths` was replaced in round 2 but
`scripts/ci/pipeline_profiles.json` still listed the old name, so
`run_stable_pipeline.py` aborted on every profile ("unclassified=…Screens,
stale=…AllWidths"). Fixed — the `fast` list now carries `TestZXNarrowTextDemoScreens`
(585 entries, unchanged count) — and the whole fast profile was run with shards:

```
$ cd minzc && nice -n 19 taskset -c 4-15 python3 ../scripts/ci/run_stable_pipeline.py       --profile fast --output /tmp/fast-out --shards 4 --workers 4
Pipeline inventory: 956 total, 585 selected (fast, job 1/1), 4 exhaustive process shards, 4 workers
$ cat /tmp/fast-out/statuses.json
[{"shard": 1, "exit_code": 0, ...}, {"shard": 2, "exit_code": 0, ...},
 {"shard": 3, "exit_code": 0, ...}, {"shard": 4, "exit_code": 0, ...}]
$ grep -c '^--- FAIL' /tmp/fast-out/shard-*.log
shard-1.log:0  shard-2.log:0  shard-3.log:0  shard-4.log:0
$ grep -h -- '--- PASS: TestZXNarrowText' /tmp/fast-out/shard-*.log
--- PASS: TestZXNarrowTextDemoScreens (6.46s)     [all nine subtests PASS]
--- PASS: TestZXNarrowTextTiming (2.08s)
```

### 10.2 win_open's clamps overflowed

`if x + w > tx_cols` wraps in u8: `win_open(0, 0, 20, 10, 250)` gave `y + h = 14`,
kept `h = 250` and painted attribute rows 20..269, i.e. straight past `$5AFF`
into the system-variable area. Both tests now use the overflow-safe form
`if ww > tx_cols - x` / `if hh > TX_ROWS - y` (the reject above already
guarantees `x < tx_cols` and `y < TX_ROWS`, so the right-hand side cannot wrap).
The critic's probe, rerun:

```
win_open(1, 0, 20, 10, 250)  ->  w = 10, h = 4    (was h = 250)
win_open(2, 40, 0, 250, 4)   ->  w = 11, h = 4    (was w = 250, and win_clear on
                                                    it corrupted the neighbour)
bytes at $5B00..$5B07 after both:  all 0x00        (round 2 wrote 0x07 there)
```

### 10.3 Performance — round 1 beaten, not just matched

Two changes, both from the critic's analysis:

* **A scalar mirror of the current window** (`tx_c_xpx/y/w/h/col/row`), refreshed
  by `win_open`, `win_select` and `win_goto`. A dynamically indexed `wn_x[id]`
  read costs ~250 T in this compiler and `win_putch` did six of them per
  character. The zero-size guard is now one scalar compare (an empty window is
  collapsed to `w = h = 0` when the mirror is refreshed, so a zero-*height*
  window with a non-zero width cannot slip through and underflow the row — the
  edge-case probe caught exactly that during the port).
* **The scroll is one asm block** (`tx_scroll_row`): one address per pixel line
  (byte columns of a char row at a given line are contiguous, and the row below
  is +32), a plain `LDIR` for the span when the window owns both edge bytes, and
  two merged bytes otherwise. Round 2's scroll cost six `tx_scr_addr` calls per
  line through a per-line helper.

Measured with the same bench and the same differential method, and with the
round-1 and round-2 trees checked out and re-measured against it:

| | round 1 | round 2 | **round 3** |
|---|---|---|---|
| `tx_blit` aligned / straddling | 829 / 1093 | 835 / 1099 | 835 / 1099 |
| `win_goto` | 397 | 631 | **562** |
| `win_putch` (goto + putch, core 0) | 2173 | 2881 | **1960** |
| `win_putch` (core 1) | 3025 | 3733 | **2812** |
| 51-column row (core 0) | 120013 | 141829 | **98167** |
| 42-column row | 99129 | 117183 | **81189** |
| 32-column row | 71438 | 85312 | **57838** |
| `win_scroll` (24-row pane, 5/6 px) | 484924 | 1117331 | **311292** |
| `win_scroll` (8 px) | 484924 | 1118086 | **267330** |
| T per glyph in a row | 2353 | 2781 | **1925** |

**Full-redraw settle time** — frames from the keypress to the last screen
change, the critic's `settle.py` method, which is also the window in which the
demo cannot see the keyboard:

| target width | round 1 | round 2 | **round 3** |
|---|---|---|---|
| 51 col | 90 frames / 6.29 M T / 1.80 s | 122 / 8.53 M / 2.44 s | **83 / 5.80 M / 1.66 s** |
| 42 col | 90 / 6.29 M / 1.80 s | 130 / 9.09 M / 2.60 s | **82 / 5.73 M / 1.64 s** |
| 32 col | 167 / 11.67 M / 3.33 s | 360 / 25.16 M / 7.19 s | **103 / 7.20 M / 2.06 s** |

So the keyboard-deaf window after a width switch is down from 2.44 s to 1.66 s,
and the 32-column case — 96 % of which was scrolling — from 7.19 s to 2.06 s.
`narrow_font_build` and `text_set_width` are unchanged (1 595 698 / 1 857 070 T
at 5 px). The timing test's assertion table carries all of these.

### 10.4 "Finding 6" retracted

See §9.7: the claim was a measurement artefact (the probe sampled C but never A,
and the caller's `LD A, C` shuffle leaves the value in both). Sampling A reads
exactly what the emitted ABI comment says, and `mzd --regs --verify-abi` reports
68/68 functions matched. The true, documented statement — the convention is
*per function*, chosen from the callee's body, so check the emitted
`; fun … (p: t = reg)` comment before a hand-written `CALL` — replaces it in
§9.7 and in `reports/zx-narrow-text/repro/README.md`. The argument-slot design
stays, for that reason.

### 10.5 The remaining should-fix items

* **Memory map** (§9.4 numbers): code is 6.0 K not 8.1 K, free RAM 10318 not
  10317, and there are 17 wrappers not 15. Fixed in place. Round 3 moves it
  again — see §10.7.
* **The inside/outside diagnostic** in `TestZXNarrowTextDemoScreens` multiplied
  the right pane's pixel origin by the width a second time, so no byte was ever
  classified "inside the right pane" and every right-pane difference was
  reported as outside. Fixed: `rightPx` is used as the pixel origin directly and
  the cell count comes from the geometry's fourth result. (The pass/fail verdict
  was never affected — it fails on the total.)
* **`sp_left_w` / `sp_gap` were write-only** after round 2's `text_set_width`
  change. Dropped, along with the header sentence that claimed they were kept
  for the caller; `split_vertical` no longer pretends to remember the layout.

### 10.6 The showcase oracle catalog

`minzc/pkg/nanz/showcase_oracle_catalog_test.go` hard-codes the number of corpus
paths. The two new examples (`zx_narrow_text_demo.nanz`,
`zx_narrow_text_bench.nanz`) take it from 202 to **204**, and the test failed
until the count and the scope string were bumped:

```
$ go test ./pkg/nanz/ -run TestShowcaseExecutableOracleCatalog -count=1
    showcase_oracle_catalog_test.go:247: corpus changed: 204 paths; refresh catalog intentionally
--- FAIL   ->   (after the bump)   --- PASS (4.52s)
```

The scope string now reads "All 204 paths retained (the previous 202 plus the
task-Z3 ZX narrow-text demo and its bench)". **Rebase note:** PR #296 adds five
more examples, so if it lands before this branch the count must be bumped again
to 209 (and the scope string reworded) — the test fails loudly, so it cannot be
missed.

### 10.7 Memory map after round 3 (image 23327 bytes, `$8000..$DB1E`)

```
$0000-$3FFF  ROM            only ever read (the font at $3D00)
$4000-$57FF  pixel bitmap   6144 bytes
$5800-$5AFF  attributes     768 bytes
$8000-$9EF6  code           engine + demo + generated tables, 7.9 K
$9EF7-$A1F6  nf_packed6     768
$A1F7-$A4F6  nf_packed5     768
$A4F7-$D4F6  nf_shift       12288
$D53F-$D5D2  engine state   masks, colpx, window arrays, mirror, scroll params
$D5D3-$D5ED  tx_rotbuf      16
$D5EE-$D615  KEYTAB         40
     ...$DB1E demo strings
$FF58        initial SP     (< 64 bytes used)
```

Free RAM above the image: `$DB1F..$FFFF` = **9441 bytes** — still comfortably
more than the zvdb port's ~4.2 K of data, but 877 bytes less than round 2
(the mirror, the scroll parameters and the asm scroll helper).

### 10.8 Verification after round 3

* Whole screen, all nine states, both verifiers: `TestZXNarrowTextDemoScreens`
  PASS (9 subtests) and `python3 scripts/zx_narrow_text_verify.py` → `ALL PASS`.
* `$AA` clear-coverage probe: 0 of 6144 bitmap and 0 of 768 attribute bytes
  uncleared.
* Non-aligned scroll isolation probe: the neighbour's pixels and its attribute
  column unchanged, the scrolling window's content correct.
* Edge probe (rejected origin, overflow clamp, zero-height, zero-size, 1×1
  window, `win_goto` clamp): all as specified, nothing written past `$5AFF`.
* IY: preserved across all public calls (re-run after the mirror change).
* Screenshots regenerated from the round-3 build.

STATUS: DONE
