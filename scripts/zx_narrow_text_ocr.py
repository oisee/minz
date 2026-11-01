#!/usr/bin/env python3
"""OCR for ZX Spectrum screens rendered by stdlib/zx/text.nanz (task Z3).

Reads a 6912-byte .scr dump, picks the W-pixel character cells out of the
interleaved bitmap using the same geometry the blitter uses, and matches each
cell against the packed font that scripts/zx_narrow_font.py derives from the
ROM font plus the shipped deletion masks — so the decoder and the Z80 code
agree by construction (both are driven by stdlib/zx/narrow_font_tables.nanz).

Library use:
    from zx_narrow_text_ocr import Screen
    s = Screen("frame.scr")
    rows = s.window(x_px=130, y=0, cols=25, rows=24, width=5)

Command line:
    zx_narrow_text_ocr.py --scr F --width 5 --x-px 0 --cols 24 [--rows 24] [--y 0]
    zx_narrow_text_ocr.py --scr F --width 5 --dump   # whole screen, 51 columns

Exit status is 0 only if every cell decoded to exactly one character.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import zx_narrow_font as nfg  # noqa: E402  (the generator is the font model)

REPO = Path(__file__).resolve().parent.parent
FIRST = 32


def scr_offset(bcol: int, row: int, line: int) -> int:
    """Offset of a pixel byte in the .scr, using the ZX interleaved layout."""
    return ((row & 0x18) << 8) + (line << 8) + ((row & 7) << 5) + bcol


def packed_fonts() -> dict[int, list[list[int]]]:
    font = nfg.load_font()
    out = {8: [g[:] for g in font]}
    for w in nfg.WIDTHS:
        out[w] = nfg.build_width(font, w)[2]
    return out


class Screen:
    def __init__(self, path: str | Path):
        data = Path(path).read_bytes()
        if len(data) != 6912:
            sys.exit(f"{path}: expected a 6912-byte .scr, got {len(data)}")
        self.scr = data
        self.fonts = packed_fonts()

    def cell(self, x_px: int, row: int, width: int) -> list[int]:
        """The W-pixel cell at pixel column x_px, char row `row`, left-aligned."""
        s = x_px & 7
        b = x_px >> 3
        out = []
        for line in range(8):
            v0 = self.scr[scr_offset(b, row, line)]
            v1 = self.scr[scr_offset(b + 1, row, line)] if b < 31 else 0
            g = 0
            for j in range(width):
                pos = s + j
                bit = (7 - pos) if pos < 8 else (15 - pos)
                src = v0 if pos < 8 else v1
                g = (g << 1) | ((src >> bit) & 1)
            out.append(g << (8 - width))
        return out

    def candidates(self, x_px: int, row: int, width: int) -> list[str]:
        cell = self.cell(x_px, row, width)
        font = self.fonts[width]
        return [chr(FIRST + i) for i, g in enumerate(font) if g == cell]

    def window(self, x_px: int, y: int, cols: int, rows: int, width: int) -> list[str]:
        """Decode a rectangle of character cells; '?' marks an unreadable cell."""
        out = []
        for r in range(rows):
            line = ""
            ambiguous = []
            for c in range(cols):
                cand = self.candidates(x_px + c * width, y + r, width)
                if not cand:
                    line += "?"
                else:
                    line += cand[0]
                    if len(cand) > 1:
                        ambiguous.append((c, "".join(cand)))
            out.append(line)
            if ambiguous:
                print(f"  note: row {r} ambiguous cells {ambiguous}", file=sys.stderr)
        return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--scr", required=True, help="6912-byte screen dump")
    ap.add_argument("--width", type=int, required=True, choices=(5, 6, 8))
    ap.add_argument("--x-px", type=int, default=0, help="pixel column of cell 0")
    ap.add_argument("--y", type=int, default=0, help="character row of the top line")
    ap.add_argument("--cols", type=int, default=0, help="0 = whole screen")
    ap.add_argument("--rows", type=int, default=24)
    a = ap.parse_args()

    s = Screen(a.scr)
    cols = a.cols if a.cols else {5: 51, 6: 42, 8: 32}[a.width]
    for line in s.window(a.x_px, a.y, cols, a.rows, a.width):
        print(line)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
