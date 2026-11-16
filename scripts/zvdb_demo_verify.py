#!/usr/bin/env python3
"""zvdb demo end-to-end verification and screen capture.

Builds the CP/M demo with the production pipeline, runs it headless under mze
with scripted keys, renders the VT100 byte stream into plain-text screens, and
checks the Z80 top-K against the Python reference.

Stages:
  1. compile examples/nanz/zvdb_dump_cpm.nanz -> .com, run it under mze, and
     compare every (sel, rank, idx, score, label) row with
     examples/nanz/zvdb_demo_expected.txt  (Z80 == Python for every record)
  2. compile examples/nanz/zvdb_demo_cpm.nanz -> .com, run the scripted session
     start / 3x down / 1x up / 30x down / q, and write each screen to
     reports/zvdb-demo-cpm/screen-*.txt
  3. check the session only clears the screen at startup and shutdown, and that
     a cursor move emits a bounded number of bytes (incremental redraw)
  4. measure per-keypress T-states with mze -c

Exit status 0 iff every check passes.
"""

from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
EXAMPLES = REPO / "examples" / "nanz"
ARTIFACTS = REPO / "reports" / "zvdb-demo-cpm"

EXPECTED = EXAMPLES / "zvdb_demo_expected.txt"

# Mirrors SCR_W / SCR_H in examples/nanz/zvdb_demo_cpm.nanz.
SCR_W, SCR_H = 80, 24

CYCLES_RE = re.compile(rb"Total execution: (\d+) T-states\r?\n?$")

# The scripted session from the task, plus which input path each step exercises.
#   start        - no keys
#   3x down      - VT100 arrow ESC [ B (tui_read_key decodes it to 129)
#   1x up        - 'k'
#   30x down     - 'j', scrolls the master list
#   q            - quit
#
# Every run appends 'q': a real CP/M console input blocks, but mze's BDOS 1
# returns 0 at EOF, and the demo ignores keys it does not know, so a script
# without a final 'q' would spin. The stream is cut at the shutdown clear so the
# screen captured is the one the user saw before quitting.
SESSION = [
    ("start", ""),
    ("down3", "\x1b[B\x1b[B\x1b[B"),
    ("up1", "k"),
    ("down30", "j" * 30),
]


class Screen:
    """Just enough VT100 to render the demo's output: CUP, ED, SGR, print."""

    def __init__(self, width: int, height: int) -> None:
        self.width, self.height = width, height
        self.cells = [[" "] * width for _ in range(height)]
        self.x = self.y = 0
        self.clears = 0
        self.inverse = False
        self.cursor_row = None    # last row written in selection colours

    def _put(self, ch: str) -> None:
        if self.y < self.height and self.x < self.width:
            self.cells[self.y][self.x] = ch
        if self.inverse:
            self.cursor_row = self.y
        self.x += 1
        if self.x >= self.width:
            self.x = 0
            self.y += 1

    def feed(self, data: bytes) -> None:
        i, n = 0, len(data)
        while i < n:
            b = data[i]
            if b != 0x1B:
                if 0x20 <= b < 0x7F:
                    self._put(chr(b))
                i += 1
                continue
            if i + 1 >= n or data[i + 1] != ord("["):
                i += 1  # bare ESC: ignored (private modes arrive as ESC [ ? ..)
                continue
            j = i + 2
            params = bytearray()
            while j < n and (0x30 <= data[j] <= 0x3F):
                params.append(data[j])
                j += 1
            while j < n and 0x20 <= data[j] <= 0x2F:
                j += 1
            if j >= n:
                break
            final = chr(data[j])
            i = j + 1
            if final == "H" or final == "f":
                row, _, col = params.decode("ascii", "replace").partition(";")
                self.y = max(0, int(row or 1) - 1)
                self.x = max(0, int(col or 1) - 1)
            elif final == "J":
                if params in (b"2", b""):
                    self.cells = [[" "] * self.width for _ in range(self.height)]
                    self.clears += 1
            elif final == "K":
                for c in range(self.x, self.width):
                    self.cells[self.y][c] = " "
            elif final == "A":
                self.y = max(0, self.y - 1)
            elif final == "B":
                self.y = min(self.height - 1, self.y + 1)
            elif final == "C":
                self.x = min(self.width - 1, self.x + 1)
            elif final == "D":
                self.x = max(0, self.x - 1)
            # 'm' (SGR): only the selection colour matters here (90;106 = black
            # on bright cyan); 'h'/'l' (modes) have no text effect.
            elif final == "m":
                self.inverse = params == b"90;106"

    def text(self) -> str:
        lines = ["".join(row).rstrip() for row in self.cells]
        # Tag the row the cursor highlights: SGR carries it, plain text cannot.
        # The title bar shares the selection's colours, so only a row inside the
        # master pane counts.
        if self.cursor_row is not None and 3 <= self.cursor_row < 3 + 19:
            lines[self.cursor_row] += "   <== cursor"
        return "\n".join(lines)


def run(tool: str, args: list[str], stdin: bytes | None = None) -> tuple[bytes, int]:
    p = subprocess.run([tool] + args, input=stdin or b"", capture_output=True)
    return p.stdout, p.returncode


def strip_cycles(out: bytes) -> tuple[bytes, int | None]:
    m = CYCLES_RE.search(out)
    if not m:
        return out, None
    return out[:m.start()], int(m.group(1))


def build(mz: str, source: Path, out: Path) -> None:
    out.write_bytes(b"")
    _, rc = run(mz, [str(source), "-t", "cpm", "-o", str(out)])
    if rc != 0 or not out.exists() or out.stat().st_size == 0:
        raise SystemExit(f"{mz} {source} -> {out} failed (rc={rc})")


def expected_rows() -> list[str]:
    return [l.rstrip("\n") for l in EXPECTED.read_text().splitlines()
            if l and not l.startswith("#")]


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    root = REPO / "minzc"
    ap.add_argument("--mz", default=os.environ.get("ZVDB_MZ", str(root / "mz")))
    ap.add_argument("--mze", default=os.environ.get("ZVDB_MZE", str(root / "mze")))
    ap.add_argument("--artifacts", type=Path, default=ARTIFACTS)
    ap.add_argument("--quiet", action="store_true")
    args = ap.parse_args()

    for tool in (args.mz, args.mze):
        if not Path(tool).exists():
            raise SystemExit(f"tool not found: {tool} (build it: cd minzc && make mz mze)")
    args.artifacts.mkdir(parents=True, exist_ok=True)

    dump_com = args.artifacts / "zvdb_dump_cpm.com"
    demo_com = args.artifacts / "zvdb_demo_cpm.com"

    # ── 1. Z80 top-K vs the Python reference, for every record ───────────────
    build(args.mz, EXAMPLES / "zvdb_dump_cpm.nanz", dump_com)
    out, rc = run(args.mze, ["-t", "cpm", "--cpm-no-echo", str(dump_com), "-c"])
    if rc != 0:
        raise SystemExit(f"mze {dump_com} exited {rc}")
    body, cycles = strip_cycles(out)
    dump_lines = [l for l in body.decode("ascii", "replace").replace("\r\n", "\n").split("\n") if l]
    want = expected_rows()
    (args.artifacts / "zvdb_dump.txt").write_text("\n".join(dump_lines) + "\n")
    records = len({l.split("\t", 1)[0] for l in want})
    mismatches = [(a, b) for a, b in zip(dump_lines, want) if a != b]
    ok = not mismatches and len(dump_lines) == len(want)
    print(f"[1] Z80 top-K vs Python reference: {len(dump_lines)}/{len(want)} rows, "
          f"{len(mismatches)} mismatches -> {'PASS' if ok else 'FAIL'}"
          + (f" ({cycles} T-states for all {records} searches)" if cycles else ""))
    for a, b in mismatches[:5]:
        print(f"    z80: {a!r}\n    py : {b!r}")
    stage1 = ok

    # ── 2. Scripted session, screen by screen ────────────────────────────────
    build(args.mz, EXAMPLES / "zvdb_demo_cpm.nanz", demo_com)
    keys, stage2 = "", True
    for name, chunk in SESSION:
        keys += chunk
        out, rc = run(args.mze, ["-t", "cpm", "--cpm-no-echo", str(demo_com), "-c"],
                      (keys + "q").encode())
        body, cycles = strip_cycles(out)
        # Drop the shutdown sequence: ESC[0m ESC[?25h ESC[2J ESC[H
        cut = body.rfind(b"\x1b[2J")
        live = body[:cut] if cut >= 0 else body
        screen = Screen(SCR_W, SCR_H)
        screen.feed(live)
        (args.artifacts / f"screen-{name}.txt").write_text(screen.text() + "\n")
        (args.artifacts / f"stream-{name}.bin").write_bytes(live)
        if rc != 0:
            print(f"[2] {name}: mze exited {rc} -> FAIL")
            stage2 = False
        if screen.clears != 1:
            print(f"[2] {name}: expected exactly 1 full clear (startup), "
                  f"saw {screen.clears} -> FAIL")
            stage2 = False
        if not args.quiet:
            print(f"[2] {name:<7} keys={len(keys):>3}B out={len(live):>5}B "
                  f"clears={screen.clears} T={cycles}")

    # Incremental redraw: the bytes one extra cursor move adds must sit well
    # under a full repaint, must not clear the screen, and must not touch the
    # chrome rows (title, pane headers, separator) that never change.
    out, _ = run(args.mze, ["-t", "cpm", "--cpm-no-echo", str(demo_com), "-c"], b"jq")
    body, _ = strip_cycles(out)
    cut = body.rfind(b"\x1b[2J")
    one = (body[:cut] if cut >= 0 else body)
    # Both sessions end with the shutdown "reset + show cursor"; drop it so the
    # one-move stream is a byte-for-byte extension of the startup stream.
    tail = b"\x1b[0m\x1b[?25h"
    if not (one.endswith(tail)):
        print("[2] incremental redraw: unexpected shutdown tail -> FAIL")
        stage2 = False
    else:
        one = one[: len(one) - len(tail)]
    zero = (args.artifacts / "stream-start.bin").read_bytes()
    if zero.endswith(tail):
        zero = zero[: len(zero) - len(tail)]
    if not one.startswith(zero):
        print("[2] incremental redraw: one-move stream is not an extension of the "
              "startup stream -> FAIL")
        stage2 = False
    else:
        delta = one[len(zero):]
        chrome = [b"\x1b[2J",          # screen clear
                  b"\x1b[01;01H",      # title row
                  b"\x1b[03;01H",      # left pane header
                  b"\x1b[03;39H",      # right pane header
                  b"\x1b[23;01H"]      # separator row
        touched = [c for c in chrome if c in delta]
        if touched:
            print(f"[2] incremental redraw: cursor move rewrote {touched} -> FAIL")
            stage2 = False
        else:
            print(f"[2] incremental redraw: one cursor move emits {len(delta)}B "
                  f"(full repaint {len(zero)}B), chrome untouched -> PASS")

    # ── 3. Per-keypress T-states ─────────────────────────────────────────────
    base = None
    costs = []
    for moves in (0, 1, 10, 34):
        out, _ = run(args.mze, ["-t", "cpm", "--cpm-no-echo", str(demo_com), "-c"],
                     ("j" * moves + "q").encode())
        _, cycles = strip_cycles(out)
        if cycles is None:
            print("[3] mze did not report T-states -> FAIL")
            return 1
        if moves == 0:
            base = cycles
        else:
            costs.append((moves, (cycles - base) / moves))
    print(f"[3] startup + first search + full draw: {base} T-states")
    for moves, each in costs:
        print(f"[3] marginal cost per cursor move over {moves:>2} moves: {each:,.0f} T-states")

    print(f"artifacts in {args.artifacts}")
    return 0 if stage1 and stage2 else 1


if __name__ == "__main__":
    raise SystemExit(main())
