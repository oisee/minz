#!/usr/bin/env python3
"""End-to-end proof for the ZX narrow-text engine (task Z3).

Builds examples/nanz/zx_narrow_text_demo.nanz, runs it headless in mzx at 51, 42
and 32 columns (switching with native key-matrix scripts), dumps the 6912-byte
screen, decodes it back to characters with scripts/zx_narrow_text_ocr.py and
asserts the decoded cells equal the expected text cell for cell — including the
wrap in the right pane and the window scroll it causes. Attributes and the
split divider are checked too.

Usage:
    python3 scripts/zx_narrow_text_verify.py [--keep]
"""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import zx_narrow_text_ocr as ocr  # noqa: E402

REPO = Path(__file__).resolve().parent.parent
MZ = REPO / "minzc" / "mz"
MZA = REPO / "minzc" / "mza"
MZX = REPO / "minzc" / "mzx-headless"
DEMO = REPO / "examples" / "nanz" / "zx_narrow_text_demo.nanz"
OUT = REPO / "reports" / "zx-narrow-text"

ROWS = 24
NSENT = 24

# width -> (columns, left pane cells, key that switches to it, frame the key is
# pressed at, total frames to run). The demo starts at 51 columns.
LAYOUTS = [
    (5, 51, 24, None, 0, 500),
    (6, 42, 16, "2", 400, 1000),
    (8, 32, 16, "3", 400, 1000),
]

KEY_BITS = {"1": (3, 0), "2": (3, 1), "3": (3, 2)}


# ── expected content, read straight out of the demo source ──────────────────

def demo_sentences() -> list[str]:
    src = DEMO.read_text()
    body = re.search(r"fun sentence\(i: u8\) -> \^u8 \{(.*?)\n\}", src, re.S).group(1)
    out = {}
    for num, text in re.findall(r'if i == (\d+) \{ return c"([^"]*)" \}', body):
        out[int(num)] = text
    tail = body.split("if i ==")[-1]
    fallback = re.findall(r'return c"([^"]*)"', tail)[-1]
    return [out.get(i, fallback) for i in range(NSENT)]


def demo_paragraph() -> str:
    src = DEMO.read_text()
    return re.search(r'win_print\(c"([^"]*)"\)', src).group(1)


def left_row(i: int, cols: int) -> str:
    field = cols - 4
    text = demo_sentences()[i]
    padded = text[:field].ljust(field)
    return f"{i:02d} {padded}".ljust(cols)


def right_rows(right_cols: int) -> list[str]:
    """Mirror win_print's cursor advance exactly, including the wrap after the
    last character and the window scroll it triggers."""
    text = demo_paragraph()
    screen: list[list[str]] = []
    col = row = 0
    for ch in text:
        if row == len(screen):
            screen.append([" "] * right_cols)
        screen[row][col] = ch
        col += 1
        if col >= right_cols:
            col = 0
            row += 1
            if row >= ROWS:
                screen.pop(0)
                screen.append([" "] * right_cols)
                row = ROWS - 1
    while len(screen) < ROWS:
        screen.append([" "] * right_cols)
    return ["".join(r) for r in screen[:ROWS]]


def geometry(width: int, cols: int, left_w: int) -> tuple[int, int, int, int, int]:
    """(left_x_px, left_cols, right_x_px, right_cols, divider_px) — the same
    arithmetic split_vertical performs on the Z80."""
    left_end = left_w * width
    div_end = left_end + 8 if left_end % 8 == 0 else (left_end + 7) & ~7
    right_x = 0
    px = 0
    while px < div_end:
        px += width
        right_x += 1
    div_px = left_end + ((div_end - left_end) >> 1)
    return 0, left_w, right_x * width, cols - right_x, div_px


# ── runner ──────────────────────────────────────────────────────────────────

def run(cmd: list[str]) -> None:
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"command failed: {' '.join(cmd)}\n{r.stdout}\n{r.stderr}")


def build_demo() -> Path:
    OUT.mkdir(parents=True, exist_ok=True)
    a80 = OUT / "zx_narrow_text_demo.a80"
    bin_ = OUT / "zx_narrow_text_demo.bin"
    run([str(MZ), str(DEMO), "-o", str(a80)])
    run([str(MZA), str(a80), "-o", str(bin_)])
    return bin_


def capture(bin_: Path, tag: str, key: str | None, key_frame: int, frames: int) -> Path:
    scr_dir = OUT / f"scr-{tag}"
    if scr_dir.exists():
        shutil.rmtree(scr_dir)
    cmd = [str(MZX), "--model", "48k", "--run", f"{bin_}@8000",
           "--frames", str(frames), "--dump-scr", str(scr_dir),
           "--screenshot", str(OUT / f"screenshot-{tag}.png")]
    if key is not None:
        row, bit = KEY_BITS[key]
        script = OUT / f"keys-{tag}.json"
        script.write_text(json.dumps(
            [{"frame": key_frame, "row": row, "bit": bit, "down": True},
             {"frame": key_frame + 20, "row": row, "bit": bit, "down": False}]))
        cmd += ["--key-script", str(script)]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"mzx failed: {r.stderr}")
    dumps = sorted((scr_dir).glob("frame_*.scr"))
    if not dumps:
        sys.exit("mzx produced no screen dumps")
    return dumps[-1]


def check(tag: str, scr_path: Path, width: int, cols: int, left_w: int) -> list[str]:
    s = ocr.Screen(scr_path)
    left_px, left_cols, right_px, right_cols, div_px = geometry(width, cols, left_w)
    problems: list[str] = []

    want_left = [left_row(i, left_cols) for i in range(NSENT)]
    got_left = s.window(left_px, 0, left_cols, ROWS, width)
    for r, (want, got) in enumerate(zip(want_left, got_left)):
        if want != got:
            problems.append(f"{tag} left pane row {r}: want {want!r} got {got!r}")

    want_right = right_rows(right_cols)
    got_right = s.window(right_px, 0, right_cols, ROWS, width)
    for r, (want, got) in enumerate(zip(want_right, got_right)):
        if want != got:
            problems.append(f"{tag} right pane row {r}: want {want!r} got {got!r}")

    # attributes: left pane black ink on cyan paper bright, the cursor row (23)
    # inverted; right pane white ink on blue paper.
    def attr(bcol: int, row: int) -> int:
        return s.scr[6144 + row * 32 + bcol]

    lb0, lb1 = left_px >> 3, (left_px + left_cols * width - 1) >> 3
    rb0, rb1 = right_px >> 3, (right_px + right_cols * width - 1) >> 3
    want_l, want_li = (5 << 3) | 0 | 0x40, (0 << 3) | 5 | 0x40
    want_r = (1 << 3) | 7
    for row in range(ROWS):
        for b in range(lb0, lb1 + 1):
            want = want_li if row == ROWS - 1 else want_l
            if attr(b, row) != want:
                problems.append(f"{tag} left attr row {row} col {b}: "
                                f"{attr(b, row):#04x} want {want:#04x}")
        for b in range(rb0, rb1 + 1):
            if attr(b, row) != want_r:
                problems.append(f"{tag} right attr row {row} col {b}: "
                                f"{attr(b, row):#04x} want {want_r:#04x}")

    # the divider is a one-pixel vertical rule spanning all 24 character rows
    byte, bit = div_px >> 3, 0x80 >> (div_px & 7)
    for row in range(ROWS):
        for line in range(8):
            v = s.scr[ocr.scr_offset(byte, row, line)]
            if not v & bit:
                problems.append(f"{tag} divider missing at row {row} line {line}")
    return problems


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--keep", action="store_true", help="keep the .scr dump directories")
    a = ap.parse_args()

    bin_ = build_demo()
    failures = 0
    for width, cols, left_w, key, key_frame, frames in LAYOUTS:
        tag = f"w{width}"
        scr = capture(bin_, tag, key, key_frame, frames)
        problems = check(tag, scr, width, cols, left_w)
        status = "PASS" if not problems else f"FAIL ({len(problems)} problems)"
        print(f"[{tag}] {cols} columns, {scr.name}: {status}")
        for p in problems[:8]:
            print("    " + p)
        if len(problems) > 8:
            print(f"    ... and {len(problems) - 8} more")
        failures += len(problems)
        if not a.keep:
            shutil.rmtree(OUT / f"scr-{tag}", ignore_errors=True)

    # a snapshot of the running 51-column screen
    run([str(MZX), "--model", "48k", "--run", f"{bin_}@8000", "--frames", "500",
         "--save-snapshot", str(OUT / "zx_narrow_text_demo.sna")])
    print(f"[sna] {OUT / 'zx_narrow_text_demo.sna'}")

    print(f"{'ALL PASS' if failures == 0 else f'{failures} FAILURES'}")
    return 0 if failures == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
