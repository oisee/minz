#!/usr/bin/env python3
"""Generate a minimal 131070-byte Z3 story with code and text near 128 KiB.

SPDX-License-Identifier: MIT
"""

from pathlib import Path
import argparse

p = argparse.ArgumentParser()
p.add_argument('output', type=Path)
p.add_argument('--bytes', type=int, default=131070)
a = p.parse_args()
if a.bytes not in (98302, 131070):
    p.error('supported fixture sizes: 98302, 131070')
routine = a.bytes - 510
story = bytearray(a.bytes)
story[0] = 3
story[4:8] = bytes.fromhex('00400040')  # high memory and initial PC
story[14:16] = bytes.fromhex('0040')     # static-memory base
story[26:28] = (a.bytes // 2).to_bytes(2, 'big')     # file length / 2

# call packed $ff00 (byte $1fe00), store on stack, quit
story[0x40:0x46] = bytes([0xe0, 0x3f]) + (routine // 2).to_bytes(2, 'big') + bytes([0, 0xba])

# 0 locals; print "H" as a packed Z-string; newline; return true
story[routine:routine+5] = bytes.fromhex('00b291a5bb')
story[routine+5] = 0xB0

checksum = sum(story[64:]) & 0xFFFF
story[28:30] = checksum.to_bytes(2, 'big')
a.output.write_bytes(story)
