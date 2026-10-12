"""Source-level VDU oracle for fixed Agon demos; not a framebuffer oracle.

The independent historical source-level text/layout model supplies the same
unchanged demo operations. Translate those operations into documented native
VDU commands; never use compiled output to construct expected bytes.
"""
import re
from agon_tui_stream_model import expected as source_layout


def expected(name, keys=(27,)):
    ansi = source_layout(name, keys).encode("ascii")
    # ANSI clear+home maps to VDU12, which already homes the viewport.
    ansi = ansi.replace(b"\x1b[2J\x1b[H", b"\x0c")
    ansi = ansi.replace(b"\x1b[?25l", bytes([23, 1, 0]))
    ansi = ansi.replace(b"\x1b[?25h", bytes([23, 1, 1]))

    def convert(match):
        params, op = match.group(1).decode(), match.group(2)
        if op == b"H":
            y, x = map(int, params.split(";"))
            return bytes([31, x - 1, y - 1])
        if op == b"m":
            if params == "0":
                return bytes([17, 7, 17, 128])
            fg, bg = map(int, params.split(";"))
            fg = fg - 90 + 8 if fg >= 90 else fg - 30
            bg = bg - 100 + 8 if bg >= 100 else bg - 40
            return bytes([17, fg, 17, 128 + bg])
        raise ValueError(match.group())

    result = re.sub(rb"\x1b\[([0-9;]*)([Hm])", convert, ansi)
    if b"\x1b" in result:
        raise ValueError("unmodelled ANSI operation")
    return result
