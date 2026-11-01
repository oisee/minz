#!/usr/bin/env python3
"""Independent model of the zvdb ZX demo's screen (task Z4).

A zvdb-specific model written on top of scripts/zx_narrow_text_model.py: it
re-derives the geometry split_vertical produces, the cell text the demo prints
and the attribute rectangles it paints, from the engine's documented contract
and the kernel reference — never by calling into the engine.

scripts/zvdb_zx_verify.py uses it to check what mzx actually rendered; the Go
test (minzc/pkg/pipeline/zvdb_demo_zx_test.go) carries a twin of it and diffs
all 6912 bytes.

Usage:
    zvdb_zx_model.py WIDTH            # print the geometry
    zvdb_zx_model.py WIDTH SCREEN     # print the expected cell grid
"""

from __future__ import annotations

import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
EXPECTED = REPO / "examples" / "nanz" / "zvdb_demo_expected.txt"

ROWS = 24
N_ROWS = 19
ROW_ITEMS, ROW_SEP, ROW_STATUS = 3, 22, 23
IDX_W = SCORE_W = NUM_W = 3
COLS = {5: 51, 6: 42, 8: 32}

TITLE_HEAD, TITLE_TAIL = " zvdb Z80 sim  recs:", "  256-bit hyperplanes"
STATUS_HEAD, STATUS_TAIL = " JK/67 mv 123 w Q quit scan:", " recs scanned"
HEAD_L, HEAD_R = "#   record", "scr nearest records"

# ink/paper/bright per region, as chrome()/win_attr() apply them
ATTR = {
    "title": (0, 5, 1),
    "label": (7, 1, 1),
    "head_l": (6, 1, 1),
    "head_r": (7, 0, 1),
    "items_l": (6, 1, 0),
    "items_r": (7, 0, 0),
}


def attr_byte(name):
    ink, paper, bright = ATTR[name]
    return (paper << 3) | ink | (0x40 if bright else 0)


def left_w_for(width: int) -> int:
    return 24 if width == 5 else 16


def geometry(width: int) -> dict:
    """Mirror the demo's layout(): split_vertical(left_w_for(w), 1) plus the
    pane re-open over the item rows."""
    cols, left_w = COLS[width], left_w_for(width)
    left_px = left_w * width                     # a multiple of 8 by choice
    div_end = left_px + 8                        # gap = 1 widens to the next cell
    right_x = (div_end + width - 1) // width     # ceil, as split_vertical walks it
    return {
        "width": width, "cols": cols, "left_w": left_w, "left_px": left_px,
        "right_x": right_x, "right_w": cols - right_x, "right_px": right_x * width,
        "div_px": left_px + 4, "div_b0": left_px >> 3, "div_b1": (div_end - 1) >> 3,
        "label_l_w": left_w - IDX_W - 2, "label_r_w": (cols - right_x) - SCORE_W - 2,
        "label_w": cols - 6 - IDX_W,
    }


def reference() -> tuple[list[str], dict[int, list[tuple[int, int]]]]:
    """labels by record index, and {selection: [(idx, score), ...]} best first."""
    labels: dict[int, str] = {}
    ranked: dict[int, list[tuple[int, int]]] = {}
    for line in EXPECTED.read_text().splitlines():
        if not line or line.startswith("#"):
            continue
        sel, _rank, idx, score, label = line.split("\t")
        sel, idx, score = int(sel), int(idx), int(score)
        ranked.setdefault(sel, []).append((idx, score))
        labels[idx] = label
    return [labels[i] for i in range(len(labels))], ranked


def _field(s: str, w: int) -> str:
    return s[:w].ljust(w)


def _num(n: int, w: int) -> str:
    s = str(n)
    return s.rjust(w)[:w] if len(s) >= w else " " * (w - len(s)) + s


def text_grid(width: int, cursor: int, top: int) -> list[list[str]]:
    """The cell grid the demo prints: one row of `cols` cells per screen row."""
    g = geometry(width)
    labels, ranked = reference()
    grid = [[" "] * g["cols"] for _ in range(ROWS)]

    def put(row, at, s):
        grid[row][at:at + len(s)] = list(s[:g["cols"] - at])

    def head_tail(row, head, tail, n):
        hw = min(len(head), g["cols"] - NUM_W)
        put(row, 0, _field(head, hw) + _num(n, NUM_W) + _field(tail, g["cols"] - hw - NUM_W))

    head_tail(0, TITLE_HEAD, TITLE_TAIL, len(labels))
    put(1, 0, _field("Rec ", 4) + _num(cursor, IDX_W) + _field(": ", 2)
        + _field(labels[cursor], g["label_w"]))
    put(2, 0, _field(HEAD_L, g["left_w"]))
    put(2, g["right_x"], _field(HEAD_R, g["right_w"]))
    for i in range(N_ROWS):
        row = ROW_ITEMS + i
        idx = top + i
        if idx < len(labels):
            put(row, 0, _num(idx, IDX_W) + " " + _field(labels[idx], g["label_l_w"]))
        else:
            put(row, 0, " " * g["left_w"])
        if i < len(ranked[cursor]):
            j, score = ranked[cursor][i]
            put(row, g["right_x"], _num(score, SCORE_W) + " " + _field(labels[j], g["label_r_w"]))
        else:
            put(row, g["right_x"], " " * g["right_w"])
    put(ROW_SEP, 0, "-" * g["cols"])
    head_tail(ROW_STATUS, STATUS_HEAD, STATUS_TAIL, len(labels))
    return grid


def attributes(width: int, cursor: int, top: int) -> list[list[int]]:
    """The 24x32 attribute bytes the demo leaves behind."""
    g = geometry(width)
    left_b1 = (g["left_px"] - 1) >> 3
    right_b0 = g["right_px"] >> 3
    hi = ROW_ITEMS + (cursor - top)
    out = [[0] * 32 for _ in range(ROWS)]
    for row in range(ROWS):
        for b in range(32):
            if row in (0, ROW_STATUS):
                a = "title"
            elif row == 1:
                a = "label"
            elif row == 2:
                a = "head_r" if b >= right_b0 else "head_l"
            elif row == ROW_SEP:
                a = "head_l"                      # chrome() always asks for bright
            elif ROW_ITEMS <= row < ROW_ITEMS + N_ROWS:
                if b >= right_b0:
                    a = "items_r"
                else:
                    a = "items_l"
                    if row == hi and b <= left_b1:
                        a = None                  # inverted below
            else:
                continue
            out[row][b] = attr_byte(a) if a else _invert(attr_byte("items_l"))
    return out


def _invert(a: int) -> int:
    return ((a & 0x07) << 3) | ((a & 0x38) >> 3) | (a & 0xC0)


def main() -> int:
    width = int(sys.argv[1])
    if len(sys.argv) > 2 and sys.argv[2] == "SCREEN":
        for row in text_grid(width, 0, 0):
            print("".join(row))
    else:
        print(width, geometry(width))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
