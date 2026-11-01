#!/usr/bin/env python3
"""Independent full-screen model of the Z3 demo (task Z3, round 2).

Renders, from first principles, the 6912 bytes the screen should hold after a
clean width switch: a blank screen, the two panes' text blitted with the packed
fonts (derived here from the ROM font plus the shipped deletion masks), the
divider rule, and the attributes the engine paints. The Go test and
scripts/zx_narrow_text_verify.py diff an emulator dump against this model byte
for byte — all 6144 bitmap bytes and all 768 attribute bytes, including
everything *outside* the windows, which is where the round-1 leftovers lived.

Nothing here calls into the engine: the blit, the wrap/scroll, the geometry and
the attribute rule are re-derived from the module's documented contract.

Usage:
    zx_narrow_text_model.py WIDTH            # print the geometry
    zx_narrow_text_model.py WIDTH out.scr    # write the expected screen
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import zx_narrow_font as nfg  # noqa: E402

REPO = Path(__file__).resolve().parent.parent
DEMO = REPO / "examples" / "nanz" / "zx_narrow_text_demo.nanz"
ROWS = 24
NSENT = 24
COLS = {5: 51, 6: 42, 8: 32}
ATTR_L = (5 << 3) | 0x40          # win_attr(0, 5, 1)   black on cyan, bright
ATTR_L_INV = (0 << 3) | 5 | 0x40  # row 23 inverted
ATTR_R = (1 << 3) | 7             # win_attr(7, 1, 0)   white on blue


def scr_offset(bcol: int, row: int, line: int) -> int:
    return ((row & 0x18) << 8) | ((row & 7) << 5) | (line << 8) | bcol


def attr_offset(bcol: int, row: int) -> int:
    return 6144 + row * 32 + bcol


def keep_masks(width: int) -> list[tuple[int, int]]:
    """The complement of a cell's footprint, by pixel shift — the same table
    stdlib/zx/text.nanz installs (tx_masks_w) and the reason a blit cannot
    disturb the cell sharing its byte."""
    out = []
    for s in range(8):
        if s + width <= 8:
            f0, f1 = ((1 << width) - 1) << (8 - width - s), 0
        else:
            f0, f1 = 0xFF >> s, (0xFF << (16 - s - width)) & 0xFF
        out.append((f0 ^ 0xFF, f1 ^ 0xFF))
    return out


def blit(screen: bytearray, ch: int, xpx: int, row: int, width: int, font) -> None:
    if not 32 <= ch < 128:
        glyph = [0] * 8
    else:
        glyph = font[ch - 32]
    b, s = xpx >> 3, xpx & 7
    m0, m1 = keep_masks(width)[s]
    for line in range(8):
        v = glyph[line]
        t = (v >> s) & 0xFF
        u = (v << (8 - s)) & 0xFF if s else 0
        off = scr_offset(b, row, line)
        screen[off] = (screen[off] & m0) | t
        if m1:
            off1 = scr_offset(b + 1, row, line)
            screen[off1] = (screen[off1] & m1) | u


def sentences() -> list[str]:
    src = DEMO.read_text()
    body = re.search(r"fun sentence\(i: u8\) -> \^u8 \{(.*?)\n\}", src, re.S).group(1)
    by_index = {int(n): s for n, s in
                re.findall(r'if i == (\d+) \{ return c"([^"]*)" \}', body)}
    tail = body.split("if i ==")[-1]
    fallback = re.findall(r'return c"([^"]*)"', tail)[-1]
    return [by_index.get(i, fallback) for i in range(NSENT)]


def paragraph() -> str:
    return re.search(r'win_print\(c"([^"]*)"\)', DEMO.read_text()).group(1)


def left_w_for(w: int) -> int:
    return 16 if w in (6, 8) else 24


def split_geometry(w: int) -> dict:
    """Mirror split_vertical(left_w_for(w), 1) exactly."""
    cols, left_w = COLS[w], left_w_for(w)
    left_px = left_w * w
    div_end = left_px + 8 if left_px % 8 == 0 else (left_px + 7) & ~7
    right_x, px = 0, 0
    while px < div_end:
        px += w
        right_x += 1
    return {
        "left_x": 0, "left_cols": left_w, "left_px": left_px,
        "right_x": right_x, "right_cols": cols - right_x, "right_px": right_x * w,
        "div_px": left_px + ((div_end - left_px) >> 1),
        "left_b0": 0, "left_b1": (left_px - 1) >> 3,
        "right_b0": (right_x * w) >> 3,
        "right_b1": (right_x * w + (cols - right_x) * w - 1) >> 3,
        "div_b0": left_px >> 3, "div_b1": (div_end - 1) >> 3,
    }


def wrapped(text: str, cols: int) -> list[str]:
    """win_print's cursor advance: one cell per character, wrap at the window
    edge, scroll when the last row wraps."""
    rows: list[list[str]] = [[]]
    for ch in text:
        rows[-1].append(ch)
        if len(rows[-1]) == cols:
            rows.append([])
            if len(rows) > ROWS:
                rows.pop(0)
    while len(rows) < ROWS:
        rows.append([])
    return ["".join(r) for r in rows[:ROWS]]


def expected_screen(w: int, cursor_row: int = ROWS - 1) -> tuple[bytes, dict]:
    g = split_geometry(w)
    scr = bytearray(6912)
    rom = nfg.load_font()
    font = [[b for b in g] for g in rom] if w == 8 else nfg.build_width(rom, w)[2]

    for i in range(NSENT):
        field = g["left_cols"] - 4
        text = f"{i:02d} {sentences()[i][:field].ljust(field)}"
        for c, ch in enumerate(text):
            blit(scr, ord(ch), g["left_x"] * w + c * w, i, w, font)
    for r, line in enumerate(wrapped(paragraph(), g["right_cols"])):
        for c, ch in enumerate(line):
            blit(scr, ord(ch), g["right_px"] + c * w, r, w, font)

    byte, bit = g["div_px"] >> 3, 0x80 >> (g["div_px"] & 7)
    for row in range(ROWS):
        for line in range(8):
            scr[scr_offset(byte, row, line)] |= bit

    for row in range(ROWS):
        for b in range(32):
            a = None
            if g["left_b0"] <= b <= g["left_b1"]:
                a = ATTR_L_INV if row == cursor_row else ATTR_L
            elif g["right_b0"] <= b <= g["right_b1"]:
                a = ATTR_R
            elif g["div_b0"] <= b <= g["div_b1"]:
                a = ATTR_L          # the divider takes the left pane's attribute
            if a is not None:
                scr[attr_offset(b, row)] = a
    return bytes(scr), g


def main() -> int:
    w = int(sys.argv[1])
    scr, g = expected_screen(w)
    if len(sys.argv) > 2:
        Path(sys.argv[2]).write_bytes(scr)
    print(w, g)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
