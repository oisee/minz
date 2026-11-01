#!/usr/bin/env python3
"""ZX Spectrum narrow-font deletion-table generator (task Z3).

The 48K ROM font at $3D00 holds 96 glyphs (ASCII 32..127), 8 bytes each, one
byte per pixel row with bit 7 = leftmost column. Every glyph therefore occupies
an 8x8 cell even though most glyphs are narrower.

To print at 51 columns (5 px per glyph) or 42 columns (6 px) we drop 8-W pixel
columns out of every glyph row and left-align what is left in the byte. Which
columns to drop is decided once, here, and shipped as two 8-bit masks per glyph
(one for the top half rows 0-3, one for the bottom half rows 4-7) so the run
time builder in stdlib/zx/narrow_font.nanz only has to squeeze.

Mask convention: bit (7-c) set  =>  delete pixel column c. Exactly 8-W bits are
set, so the packed glyph is exactly W pixels wide, left-aligned.

Heuristic, in order (task Z3 spec):
  1. delete empty columns, preferring the outer ones;
  2. then delete a column identical to its (still living) neighbour - this
     keeps the shape, only the doubled stroke loses one pixel of width;
  3. then delete the column with the least ink.
Ties are broken outermost first, then rightmost.

Outputs (all deterministic - re-running leaves every file byte-identical):
  stdlib/zx/narrow_font_tables.nanz    the 4 x 96 mask bytes
  reports/zx-narrow-text/preview-w6.txt  ASCII preview, before/after per glyph
  reports/zx-narrow-text/preview-w5.txt

Usage:
  python3 scripts/zx_narrow_font.py            # write the files
  python3 scripts/zx_narrow_font.py --check    # exit 1 if they would change
  python3 scripts/zx_narrow_font.py --loss     # print ink-loss counts only
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
ROM_PATH = REPO / "minzc" / "cmd" / "mzx" / "roms" / "48.rom"
FONT_ADDR = 0x3D00
NGLYPH = 96
FIRST = 32
WIDTHS = (6, 5)
OUT_MODULE = REPO / "stdlib" / "zx" / "narrow_font_tables.nanz"
OUT_PREVIEW = {w: REPO / "reports" / "zx-narrow-text" / f"preview-w{w}.txt" for w in WIDTHS}

# Hand overrides for glyphs the heuristic renders badly. Empty on purpose: the
# automatic tables judged acceptable at both widths (see reports/zx-narrow-text/
# preview-w*.txt). Key = ASCII code, value = ("top", "bot") hex masks, e.g.
#   63: (0x81, 0x81),   # '?'
# The masks still have to carry exactly 8-W set bits; that is asserted below.
OVERRIDES: dict[int, dict[int, tuple[int, int]]] = {}


# ── font access ──────────────────────────────────────────────────────────────

def load_font() -> list[list[int]]:
    rom = ROM_PATH.read_bytes()
    if len(rom) < FONT_ADDR + NGLYPH * 8:
        sys.exit(f"{ROM_PATH}: too short for a 48K ROM ({len(rom)} bytes)")
    return [list(rom[FONT_ADDR + i * 8: FONT_ADDR + i * 8 + 8]) for i in range(NGLYPH)]


def column_patterns(rows: list[int]) -> list[int]:
    """One integer per pixel column; bit j set = row j has ink in that column."""
    out = []
    for c in range(8):
        v = 0
        for j, r in enumerate(rows):
            if r & (0x80 >> c):
                v |= 1 << j
        out.append(v)
    return out


# ── the heuristic ────────────────────────────────────────────────────────────

def choose_deletions(rows: list[int], need: int, code: int, half: str) -> list[int]:
    """Pixel columns to delete from one 4-row half, in deletion order."""
    pat = column_patterns(rows)
    ink = [bin(p).count("1") for p in pat]
    alive = [True] * 8
    deleted: list[int] = []

    def take(c: int) -> None:
        alive[c] = False
        deleted.append(c)

    def rank_outer(c: int) -> tuple[float, int]:
        # outermost first, then rightmost
        return (abs(c - 3.5), c)

    # 1. empty columns, preferring the outer ones
    for c in sorted((c for c in range(8) if ink[c] == 0), key=rank_outer, reverse=True):
        if len(deleted) < need:
            take(c)

    # 2. a column identical to a still-living neighbour within this half
    while len(deleted) < need:
        live = [c for c in range(8) if alive[c]]
        cands = [
            c
            for pos, c in enumerate(live)
            if (pos > 0 and pat[live[pos - 1]] == pat[c])
            or (pos + 1 < len(live) and pat[live[pos + 1]] == pat[c])
        ]
        if not cands:
            break
        take(max(cands, key=rank_outer))

    # 3. the column with the least ink
    while len(deleted) < need:
        cands = [c for c in range(8) if alive[c]]
        if not cands:
            break
        take(max(cands, key=lambda c: (-ink[c],) + rank_outer(c)))

    if len(deleted) != need:
        sys.exit(f"glyph {code} half {half}: deleted {len(deleted)} columns, wanted {need}")
    return deleted


def mask_of(deleted: list[int]) -> int:
    m = 0
    for c in deleted:
        m |= 0x80 >> c
    return m


def squeeze_row(row: int, mask: int) -> int:
    """Drop the masked columns and left-align the survivors in the byte."""
    out = 0
    kept = 0
    for c in range(8):
        if mask & (0x80 >> c):
            continue
        out = (out << 1) | ((row >> (7 - c)) & 1)
        kept += 1
    return out << (8 - kept)


def build_width(font: list[list[int]], width: int):
    """Returns (top_masks, bot_masks, packed_glyphs, stats)."""
    need = 8 - width
    top: list[int] = []
    bot: list[int] = []
    packed: list[list[int]] = []
    lost_ink = 0
    lost_shape = 0
    per_rule = [0, 0, 0]
    for i, g in enumerate(font):
        code = FIRST + i
        halves = {}
        for name, rows in (("t", g[:4]), ("b", g[4:])):
            halves[name] = choose_deletions(rows, need, code, name)
        tm, bm = mask_of(halves["t"]), mask_of(halves["b"])
        ov = OVERRIDES.get(code, {}).get(width)
        if ov is not None:
            tm, bm = ov
            for name, m in (("t", tm), ("b", bm)):
                if bin(m).count("1") != need:
                    sys.exit(f"override for {code} W={width} half {name}: {need} bits must be set")
            halves = {
                "t": [c for c in range(8) if tm & (0x80 >> c)],
                "b": [c for c in range(8) if bm & (0x80 >> c)],
            }
        top.append(tm)
        bot.append(bm)
        packed.append([squeeze_row(r, tm) for r in g[:4]] + [squeeze_row(r, bm) for r in g[4:]])

        if sum(bin(x).count("1") for x in packed[-1]) < sum(bin(x).count("1") for x in g):
            lost_ink += 1
        # shape loss = an inked column was dropped without a surviving twin
        shape = False
        for name, rows in (("t", g[:4]), ("b", g[4:])):
            pat = column_patterns(rows)
            alive = [c for c in range(8) if c not in halves[name]]
            for c in halves[name]:
                if pat[c] == 0:
                    continue
                if any(d in alive and pat[d] == pat[c] for d in (c - 1, c + 1)):
                    continue
                shape = True
        lost_shape += shape
    return top, bot, packed, (lost_ink, lost_shape)


# ── emitters ─────────────────────────────────────────────────────────────────

def emit_module(tables) -> str:
    lines = [
        "// ============================================================================",
        "// stdlib/zx/narrow_font_tables.nanz - narrow-font column deletion tables",
        "// ============================================================================",
        "//",
        "// GENERATED by scripts/zx_narrow_font.py - do not edit by hand.",
        "// Re-running the generator leaves this file byte-identical.",
        "//",
        "// Source: the 48K ROM font at $3D00 of minzc/cmd/mzx/roms/48.rom,",
        "//         96 glyphs (ASCII 32..127), 8 bytes each.",
        "//",
        "// For each glyph two 8-bit masks select which pixel columns are deleted to",
        "// reach the target width: one mask for the top half (rows 0-3) and one for",
        "// the bottom half (rows 4-7). Bit (7-c) set  =>  delete pixel column c.",
        "// Each mask carries exactly 8-W set bits, so the packed glyph is W pixels",
        "// wide, left-aligned in the byte.",
        "//",
        "// Heuristic (in order): empty columns outermost first, then a column that",
        "// duplicates its living neighbour, then the column with the least ink.",
        "// Hand overrides: none (see OVERRIDES in scripts/zx_narrow_font.py).",
        "//",
        "// Ink loss at 6 px: 3 of 96 glyphs; at 5 px: 40 of 96.",
        "//",
        "// Consumed by stdlib/zx/narrow_font.nanz (narrow_font_build) and by the",
        "// screen OCR in scripts/zx_narrow_font.py --ocr.",
        "",
    ]
    for width in WIDTHS:
        top, bot, _, (lost_ink, lost_shape) = tables[width]
        lines.append(f"// ── {width} px: {51 if width == 5 else 42 if width == 6 else '?'} columns "
                     f"({lost_ink}/96 glyphs lost ink, {lost_shape}/96 lost a non-duplicate column) ──")
        lines.append(f"global NF{width}_TOP: [u8; 96] = [")
        for row in range(0, 96, 12):
            lines.append("    " + ", ".join(f"0x{v:02X}" for v in top[row:row + 12]) + ",")
        lines.append("]")
        lines.append(f"global NF{width}_BOT: [u8; 96] = [")
        for row in range(0, 96, 12):
            lines.append("    " + ", ".join(f"0x{v:02X}" for v in bot[row:row + 12]) + ",")
        lines.append("]")
        lines.append("")
    return "\n".join(lines)


def render(glyph_rows: list[int], width: int) -> list[str]:
    out = []
    for r in glyph_rows:
        px = "".join("#" if r & (0x80 >> c) else "." for c in range(width))
        out.append(px)
    return out


def emit_preview(tables, width: int) -> str:
    font = load_font()
    _, _, packed, _ = tables[width]
    lines = [
        f"Narrow font preview - {width} px glyphs "
        f"({51 if width == 5 else 42} columns on a 256 px line)",
        "=" * 74,
        "",
        "Left: the ROM glyph as it is (8 px, bit 7 = leftmost column).",
        "Right: the same glyph after the deletion masks are applied, left-aligned.",
        "'v' marks a deleted column that carried ink (the glyph loses a pixel);",
        "'~' marks a deleted column that duplicated its neighbour (shape kept).",
        "",
    ]
    for i, g in enumerate(font):
        code = FIRST + i
        tm, bm = tables[width][0][i], tables[width][1][i]
        orig = render(g, 8)
        new = render(packed[i], width)
        head = f"{code:3d} '{chr(code) if 32 < code < 127 else ' '}'"
        note = []
        for name, rows, m in (("t", g[:4], tm), ("b", g[4:], bm)):
            pat = column_patterns(rows)
            alive = [c for c in range(8) if not (m & (0x80 >> c))]
            for c in range(8):
                if not (m & (0x80 >> c)):
                    continue
                if pat[c] == 0:
                    continue
                if any(d in alive and pat[d] == pat[c] for d in (c - 1, c + 1)):
                    note.append(f"~{name}{c}")
                else:
                    note.append(f"v{name}{c}")
        lines.append(f"{head}  mask top=0x{tm:02X} bot=0x{bm:02X}  "
                     f"{' '.join(note) if note else 'no ink lost'}")
        for r in range(8):
            lines.append(f"        {orig[r]}      {new[r]}")
        lines.append("")
    sample = "The quick brown fox jumps over the lazy dog 0123456789"
    lines.append("Sample line rendered at this width:")
    lines.append("")
    for r in range(8):
        acc = ""
        for ch in sample:
            acc += render(packed[ord(ch) - FIRST], width)[r]
        lines.append("    " + acc)
    lines.append("")
    return "\n".join(lines)


# ── main ─────────────────────────────────────────────────────────────────────

def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--check", action="store_true", help="exit 1 if any output file would change")
    ap.add_argument("--loss", action="store_true", help="print ink-loss counts only")
    args = ap.parse_args()

    font = load_font()
    tables = {w: build_width(font, w) for w in WIDTHS}

    if args.loss:
        for w in WIDTHS:
            lost_ink, lost_shape = tables[w][3]
            print(f"W={w}: {lost_ink}/{NGLYPH} glyphs lost ink, "
                  f"{lost_shape}/{NGLYPH} lost a non-duplicate ink column")
        return 0

    outputs = {OUT_MODULE: emit_module(tables)}
    for w in WIDTHS:
        outputs[OUT_PREVIEW[w]] = emit_preview(tables, w)

    changed = []
    for path, text in outputs.items():
        old = path.read_text() if path.exists() else None
        if old != text:
            changed.append(path)
            if not args.check:
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(text)

    if args.check:
        if changed:
            for p in changed:
                print(f"STALE: {p}", file=sys.stderr)
            return 1
        print(f"OK: {len(outputs)} generated files up to date")
        return 0

    for p in outputs:
        print(f"wrote {p}")
    for w in WIDTHS:
        lost_ink, lost_shape = tables[w][3]
        print(f"W={w}: {lost_ink}/{NGLYPH} glyphs lost ink, "
              f"{lost_shape}/{NGLYPH} lost a non-duplicate ink column")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
