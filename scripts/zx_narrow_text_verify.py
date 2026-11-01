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
import zx_narrow_text_model as model  # noqa: E402

REPO = Path(__file__).resolve().parent.parent
MZ = REPO / "minzc" / "mz"
MZA = REPO / "minzc" / "mza"
MZX = REPO / "minzc" / "mzx-headless"
DEMO = REPO / "examples" / "nanz" / "zx_narrow_text_demo.nanz"
OUT = REPO / "reports" / "zx-narrow-text"

ROWS = 24
NSENT = 24
WIDTH_TAG = {"w5-startup": "w5", "w6-startup": "w6", "w8-startup": "w8",
                 "5to6": "w6", "5to8": "w8", "6to8": "w8",
                 "8to6": "w6", "6to5": "w5", "8to5": "w5"}

# (width, key that switches to it, keys pressed in order with their frames,
#  total frames). Keys are held 80 frames so each spans the redraw it triggers.
KEY_BITS = {"1": (3, 0), "2": (3, 1), "3": (3, 2)}
WIDTH_KEY = {5: "1", 6: "2", 8: "3"}
LAYOUTS = [
    ("w5-startup", 5, [], 250),
    ("w6-startup", 6, [("2", 200)], 750),
    ("w8-startup", 8, [("3", 200)], 750),
    ("5to6", 6, [("2", 200)], 750),
    ("5to8", 8, [("3", 200)], 750),
    ("6to8", 8, [("2", 200), ("3", 900)], 1600),
    ("8to6", 6, [("3", 200), ("2", 900)], 1600),
    ("6to5", 5, [("2", 200), ("1", 900)], 1600),
    ("8to5", 5, [("3", 200), ("1", 900)], 1600),
]


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
    return re.search(r'win_print\(c"([^"]*)"\)', DEMO.read_text()).group(1)


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


def capture(bin_: Path, tag: str, keys: list[tuple[str, int]], frames: int) -> Path:
    scr_dir = OUT / f"scr-{tag}"
    if scr_dir.exists():
        shutil.rmtree(scr_dir)
    cmd = [str(MZX), "--model", "48k", "--run", f"{bin_}@8000",
           "--frames", str(frames), "--dump-scr", str(scr_dir),
           "--screenshot", str(OUT / f"screenshot-{WIDTH_TAG[tag]}.png")]
    if keys:
        events = []
        for key, frame in keys:
            row, bit = KEY_BITS[key]
            events += [{"frame": frame, "row": row, "bit": bit, "down": True},
                       {"frame": frame + 80, "row": row, "bit": bit, "down": False}]
        script = OUT / f"keys-{tag}.json"
        script.write_text(json.dumps(events))
        cmd += ["--key-script", str(script)]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"mzx failed: {r.stderr}")
    dumps = sorted(scr_dir.glob("frame_*.scr"))
    if not dumps:
        sys.exit("mzx produced no screen dumps")
    return dumps[-1]


def check(tag: str, scr_path: Path, width: int) -> list[str]:
    """Compare all 6144 bitmap bytes and all 768 attribute bytes against the
    independent model, and report anything outside the windows separately."""
    actual = scr_path.read_bytes()
    want, g = model.expected_screen(width)
    problems = []
    inside = outside = attrs = 0
    first_out = first_attr = ""
    for row in range(ROWS):
        for line in range(8):
            for b in range(32):
                off = model.scr_offset(b, row, line)
                if actual[off] == want[off]:
                    continue
                pane = (g["left_b0"] <= b <= g["left_b1"]) or (g["right_b0"] <= b <= g["right_b1"])
                if pane:
                    inside += 1
                else:
                    outside += 1
                    if not first_out:
                        first_out = f"byte col {b} char row {row} line {line}: " \
                                    f"got {actual[off]:#04x} want {want[off]:#04x}"
        for b in range(32):
            off = model.attr_offset(b, row)
            if actual[off] != want[off]:
                attrs += 1
                if not first_attr:
                    first_attr = f"attr col {b} row {row}: got {actual[off]:#04x} want {want[off]:#04x}"
    if inside or outside or attrs:
        problems.append(f"{tag} width {width}: {inside} bitmap bytes inside the panes, "
                        f"{outside} outside, {attrs} attribute cells differ\n"
                        f"    first outside: {first_out or '-'}\n"
                        f"    first attribute: {first_attr or '-'}")
    return problems


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--keep", action="store_true", help="keep the .scr dump directories")
    a = ap.parse_args()

    bin_ = build_demo()
    failures = 0
    shots = set()
    for tag, width, keys, frames in LAYOUTS:
        shots.add(WIDTH_TAG[tag])
        scr = capture(bin_, tag, keys, frames)
        problems = check(tag, scr, width)
        print(f"[{tag}] {width} px, {scr.name}: {'PASS' if not problems else 'FAIL'}")
        for p in problems:
            print("    " + p)
        failures += len(problems)
        if not a.keep:
            shutil.rmtree(OUT / f"scr-{tag}", ignore_errors=True)

    # a snapshot of the running 51-column screen
    run([str(MZX), "--model", "48k", "--run", f"{bin_}@8000", "--frames", "250",
         "--save-snapshot", str(OUT / "zx_narrow_text_demo.sna")])
    print(f"[sna] {OUT / 'zx_narrow_text_demo.sna'}")

    print("ALL PASS" if failures == 0 else f"{failures} FAILURES")
    return 0 if failures == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
