#!/usr/bin/env python3
"""End-to-end proof for the zvdb ZX Spectrum demo (task Z4).

Builds examples/nanz/zvdb_demo_zx.nanz with the real CLI (mz + mza), runs it
under mzx-headless at 51, 42 and 32 columns through native key-matrix scripts,
and checks what the emulator rendered against scripts/zvdb_zx_model.py: the
decoded text of both panes, the highlighted row and the attribute rectangles.
Also measures the keypress-to-settled time in frames from the per-frame screen
dumps, writes the screenshots and a .sna Alice can open in mzx, and prints the
memory map.

The byte-exact whole-screen diff (all 6912 bytes, after every step of the
scripted session) lives in the Go test
minzc/pkg/pipeline/zvdb_demo_zx_test.go; this script is the independent second
opinion that goes through the shipped binaries.

Usage:
    python3 scripts/zvdb_zx_verify.py [--keep]
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
import zvdb_zx_model as model  # noqa: E402
from zx_narrow_text_ocr import Screen  # noqa: E402

REPO = Path(__file__).resolve().parent.parent
MZ = REPO / "minzc" / "mz"
MZA = REPO / "minzc" / "mza"
MZX = REPO / "minzc" / "mzx-headless"
DEMO = REPO / "examples" / "nanz" / "zvdb_demo_zx.nanz"
OUT = REPO / "reports" / "zvdb-zx"

ROWS = 24
# key matrix: J = row 6 bit 3 (down), 1/2/3 = row 3 bits 0/1/2
KEY_BITS = {"J": (6, 3), "1": (3, 0), "2": (3, 1), "3": (3, 2)}
WIDTH_KEY = {5: None, 6: "2", 8: "3"}
TAG = {5: "w5", 6: "w6", 8: "w8"}


def run(cmd: list[str]) -> None:
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"command failed: {' '.join(cmd)}\n{r.stdout}\n{r.stderr}")


def build() -> Path:
    OUT.mkdir(parents=True, exist_ok=True)
    a80 = OUT / "zvdb_demo_zx.a80"
    bin_ = OUT / "zvdb_demo_zx.bin"
    run([str(MZ), str(DEMO), "-o", str(a80)])
    run([str(MZA), "-s", str(OUT / "zvdb_demo_zx.sym"), str(a80), "-o", str(bin_)])
    return bin_


def key_script(path: Path, presses: list[tuple[str, int]], hold: int) -> None:
    events = []
    for key, frame in presses:
        row, bit = KEY_BITS[key]
        events += [{"frame": frame, "row": row, "bit": bit, "down": True},
                   {"frame": frame + hold, "row": row, "bit": bit, "down": False}]
    path.write_text(json.dumps(events))


def capture(bin_: Path, tag: str, presses: list[tuple[str, int]], hold: int, frames: int) -> Path:
    """Run the demo and return the last screen dump plus the dump directory."""
    scr_dir = OUT / f"scr-{tag}"
    if scr_dir.exists():
        shutil.rmtree(scr_dir)
    cmd = [str(MZX), "--model", "48k", "--run", f"{bin_}@8000", "--frames", str(frames),
           "--dump-scr", str(scr_dir), "--screenshot", str(OUT / f"screenshot-{tag}.png")]
    if presses:
        script = OUT / f"keys-{tag}.json"
        key_script(script, presses, hold)
        cmd += ["--key-script", str(script)]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"mzx failed: {r.stderr}")
    dumps = sorted(scr_dir.glob("frame_*.scr"))
    if not dumps:
        sys.exit("mzx produced no screen dumps")
    return dumps[-1]


def settle_frames(scr_dir: Path, press: int, until: int) -> int:
    """Frames from a keypress to the last screen change it caused. mzx names a
    dump frame_NNNN after NNNN frames have run, so dumps[f] is the screen state
    after zero-based frame f: a key applied before frame `press` first shows in
    dumps[press]. The search stops just before `until`, the frame the next key
    is applied, so a later key cannot be credited to this one."""
    dumps = sorted(scr_dir.glob("frame_*.scr"))
    last = press
    for f in range(press, min(until, len(dumps) - 1) + 1):
        if dumps[f].read_bytes() != dumps[f - 1].read_bytes():
            last = f
    return last - press


def check_panes(tag: str, scr_path: Path, width: int, cursor: int, top: int) -> list[str]:
    g = model.geometry(width)
    grid = model.text_grid(width, cursor, top)
    scr = Screen(str(scr_path))
    data = scr_path.read_bytes()
    problems = []
    left = scr.window(x_px=0, y=model.ROW_ITEMS, cols=g["left_w"], rows=model.N_ROWS, width=width)
    right = scr.window(x_px=g["right_px"], y=model.ROW_ITEMS, cols=g["right_w"],
                       rows=model.N_ROWS, width=width)
    for i in range(model.N_ROWS):
        want_l = "".join(grid[model.ROW_ITEMS + i][:g["left_w"]])
        want_r = "".join(grid[model.ROW_ITEMS + i][g["right_x"]:])
        if left[i] != want_l:
            problems.append(f"{tag} master row {i}: {left[i]!r} want {want_l!r}")
        if right[i] != want_r:
            problems.append(f"{tag} detail row {i}: {right[i]!r} want {want_r!r}")
    # chrome rows. The header row's gutter columns carry the divider rule, so
    # they never decode as characters: compare the printed ranges only.
    for row, name in ((0, "title"), (1, "label"), (2, "header"), (model.ROW_SEP, "separator"),
                      (model.ROW_STATUS, "status")):
        got = scr.window(x_px=0, y=row, cols=g["cols"], rows=1, width=width)[0]
        want = "".join(grid[row])
        if row == 2:
            ranges = [(0, g["left_w"]), (g["right_x"], g["cols"])]
        else:
            ranges = [(0, g["cols"])]
        for lo, hi in ranges:
            if got[lo:hi] != want[lo:hi]:
                problems.append(f"{tag} {name} row [{lo}:{hi}]: {got[lo:hi]!r} want {want[lo:hi]!r}")
    # the highlighted row and the attribute rectangles
    want_attr = model.attributes(width, cursor, top)
    bad = 0
    first = ""
    for row in range(ROWS):
        for b in range(32):
            got = data[6144 + row * 32 + b]
            if got != want_attr[row][b]:
                bad += 1
                if not first:
                    first = f"attr col {b} row {row}: got {got:#04x} want {want_attr[row][b]:#04x}"
    if bad:
        problems.append(f"{tag}: {bad} attribute cells differ ({first})")
    return problems


def memory_map() -> str:
    sym = {}
    for line in (OUT / "zvdb_demo_zx.sym").read_text().splitlines():
        m = re.match(r"^(\S+)\s+= \$([0-9A-F]{4})", line)
        if m:
            sym[m.group(1)] = int(m.group(2), 16)
    size = (OUT / "zvdb_demo_zx.bin").stat().st_size
    end = 0x8000 + size
    lines = [f"image {size} bytes, $8000..${end - 1:04X}, free RAM to $FFFF: {0x10000 - end} bytes"]
    for name, note in [
        ("main", "code: demo + engine + kernel"),
        ("zx__text__zx__narrow_font__zx__narrow_font_tables__NF6_TOP", "deletion masks 384"),
        ("zx__text__zx__narrow_font__nf_packed6", "packed 6 px font 768"),
        ("zx__text__zx__narrow_font__nf_packed5", "packed 5 px font 768"),
        ("zx__text__zx__narrow_font__nf_shift", "pre-shifted glyphs 12288"),
        ("zx__text__tx_masks_w", "engine state"),
        ("zvdb_kernel__zvdb_demo_data__zv_label_off", "zvdb data: offsets 72"),
        ("zvdb_kernel__zvdb_demo_data__zv_labels", "labels 1617"),
        ("zvdb_kernel__zvdb_demo_data__zv_vecs", "vectors 1120"),
        ("zvdb_kernel__zv_pop", "popcount LUT 256"),
        ("zvdb_kernel__zv_top_val", "ranking buffers 510+255"),
        ("KEYTAB", "key matrix table 40"),
        ("_mir2_str_0", "string literals"),
    ]:
        if name in sym:
            lines.append(f"  ${sym[name]:04X}  {note}")
    return "\n".join(lines)


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--keep", action="store_true", help="keep the .scr dump directories")
    a = ap.parse_args()

    bin_ = build()
    failures = 0

    # 51 columns is the startup state; 42 and 32 are one keypress away
    layouts = [(5, [], 400, 0, 0), (6, [("2", 150)], 900, 0, 0), (8, [("3", 150)], 900, 0, 0)]
    for width, presses, frames, cursor, top in layouts:
        tag = TAG[width]
        scr = capture(bin_, tag, presses, 80, frames)
        problems = check_panes(tag, scr, width, cursor, top)
        print(f"[{tag}] {width} px ({model.COLS[width]} columns): "
              f"{'PASS' if not problems else 'FAIL'}")
        for p in problems:
            print("    " + p)
        failures += len(problems)

    # a scripted session at 51 columns: 3 down, 1 up, 30 down (the pane scrolls)
    session = []
    frame = 150
    for _ in range(3):
        session.append(("J", frame))
        frame += 45
    kup = frame                       # the one move up, spelled directly below
    frame += 45
    for _ in range(30):
        session.append(("J", frame))
        frame += 45
    scr_dir = OUT / "scr-session"
    if scr_dir.exists():
        shutil.rmtree(scr_dir)
    events = []
    for key, f in session:
        row, bit = KEY_BITS[key]
        events += [{"frame": f, "row": row, "bit": bit, "down": True},
                   {"frame": f + 8, "row": row, "bit": bit, "down": False}]
    events += [{"frame": kup, "row": 6, "bit": 2, "down": True},
               {"frame": kup + 8, "row": 6, "bit": 2, "down": False}]
    events.sort(key=lambda e: e["frame"])
    script = OUT / "keys-session.json"
    script.write_text(json.dumps(events))
    last = frame + 40
    cmd = [str(MZX), "--model", "48k", "--run", f"{bin_}@8000", "--frames", str(last),
           "--key-script", str(script),
           "--dump-scr", str(scr_dir), "--screenshot", str(OUT / "screenshot-session.png")]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"mzx failed: {r.stderr}")
    dumps = sorted(scr_dir.glob("frame_*.scr"))
    problems = check_panes("session", dumps[-1], 5, 32, 14)
    print(f"[session] 5 px after start, 3 down, 1 up, 30 down: "
          f"{'PASS' if not problems else 'FAIL'}")
    for p in problems:
        print("    " + p)
    failures += len(problems)

    # settle timings, from the same dump directory
    print(f"[settle] move that does not scroll: {settle_frames(scr_dir, 150, 194)} frames")
    print(f"[settle] move that scrolls the pane:  {settle_frames(scr_dir, 1050, 1094)} frames")
    print(f"[settle] startup (font build + search + full draw): "
          f"{settle_frames(scr_dir, 0, 149)} frames")
    for tag in ("w6", "w8"):
        d = OUT / f"scr-{tag}"
        if d.exists():
            print(f"[settle] switch to {tag}: {settle_frames(d, 150, 899)} frames")

    # a snapshot of the running 51-column screen
    run([str(MZX), "--model", "48k", "--run", f"{bin_}@8000", "--frames", "400",
         "--save-snapshot", str(OUT / "zvdb_demo_zx.sna")])
    print(f"[sna] {OUT / 'zvdb_demo_zx.sna'}")

    print(memory_map())
    if not a.keep:
        for d in OUT.glob("scr-*"):
            shutil.rmtree(d, ignore_errors=True)
    print("ALL PASS" if failures == 0 else f"{failures} FAILURES")
    return 0 if failures == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main())
